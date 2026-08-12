package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/equisolve/teamwork-cli/internal/api"
	"github.com/equisolve/teamwork-cli/internal/format"
	"github.com/spf13/cobra"
)

var filesCmd = &cobra.Command{
	Use:     "files",
	Aliases: []string{"file"},
	Short:   "List, view, and upload project files",
}

var filesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List files",
	Run:   runFilesList,
}

var filesShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show file details",
	Args:  cobra.ExactArgs(1),
	Run:   runFilesShow,
}

var filesUploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "Upload file(s) and attach them to a project or a task",
	Long: `Two-step v1 upload: POST the bytes to /pendingfiles.json, then attach the
returned ref to either a project or a task.

  --project  attaches to /projects/<id>/files.json. The file lands in the
             project's Files list and is linked to no task.
  --task     attaches to PUT /tasks/<id>.json with pendingFileAttachments, so
             the file shows up in the task's own Files section (relatedItems.tasks).

Pass --file more than once to send several files; with --task they go up in a
single attach call.

Teamwork rejects attachments on a completed task with "Your user account does
not have permission for this action". Pass --reopen to reopen the task, attach,
and re-complete it.`,
	Run: runFilesUpload,
}

func init() {
	filesListCmd.Flags().StringP("project", "p", "", "Filter by project ID or name")
	filesListCmd.Flags().Int("page", 1, "Page number")
	filesListCmd.Flags().Int("page-size", 25, "Results per page")

	filesUploadCmd.Flags().StringP("project", "p", "", "Project ID or name (exactly one of --project/--task)")
	filesUploadCmd.Flags().StringP("task", "t", "", "Task ID or name (exactly one of --project/--task)")
	filesUploadCmd.Flags().StringArray("file", nil, "Path to a file to upload (repeatable, required)")
	filesUploadCmd.Flags().String("description", "", "Optional description for the file (--project only)")
	filesUploadCmd.Flags().StringSlice("category", nil, "Category ID(s) to attach the file to (repeatable, --project only)")
	filesUploadCmd.Flags().Bool("reopen", false, "If the task is completed, reopen it to attach, then re-complete it")

	filesCmd.AddCommand(filesListCmd, filesShowCmd, filesUploadCmd)
	rootCmd.AddCommand(filesCmd)
}

func runFilesList(cmd *cobra.Command, args []string) {
	client := getClient()
	mode := getOutputMode()
	params := url.Values{}
	page, _ := cmd.Flags().GetInt("page")
	pageSize, _ := cmd.Flags().GetInt("page-size")
	params.Set("page", fmt.Sprintf("%d", page))
	params.Set("pageSize", fmt.Sprintf("%d", pageSize))

	// v3 /files.json silently ignores `projectIds`, so when the caller scopes
	// by project we use the v1 /projects/<id>/files.json endpoint instead.
	projectQ, _ := cmd.Flags().GetString("project")
	if projectQ != "" {
		pid, err := getResolver().Project(projectQ)
		if err != nil {
			exitOnError(err)
		}
		data, err := client.Get(fmt.Sprintf("/projects/%d/files.json", pid), params)
		if err != nil {
			exitOnError(err)
		}
		if mode == format.JSON {
			format.PrintJSON(data)
			return
		}
		var v1 struct {
			Project struct {
				Name  string `json:"name"`
				Files []struct {
					ID           json.Number `json:"id"`
					Name         string      `json:"name"`
					OriginalName string      `json:"originalName"`
					Description  string      `json:"description"`
					Version      json.Number `json:"version"`
				} `json:"files"`
			} `json:"project"`
		}
		_ = json.Unmarshal(data, &v1)
		headers := []string{"ID", "NAME", "VERSION", "PROJECT", "DESCRIPTION"}
		rows := make([][]string, len(v1.Project.Files))
		for i, f := range v1.Project.Files {
			name := f.Name
			if name == "" {
				name = f.OriginalName
			}
			rows[i] = []string{
				f.ID.String(),
				format.Truncate(name, 35),
				f.Version.String(),
				format.Truncate(v1.Project.Name, 25),
				format.Truncate(f.Description, 35),
			}
		}
		if mode == format.CSV {
			format.PrintCSV(headers, rows)
		} else {
			format.PrintTable(os.Stdout, headers, rows)
			fmt.Printf("\nPage %d · %d file(s)\n", page, len(rows))
		}
		return
	}

	params.Set("include", "projects")
	data, err := client.Get("/projects/api/v3/files.json", params)
	if err != nil {
		exitOnError(err)
	}
	if mode == format.JSON {
		format.PrintJSON(data)
		return
	}

	var resp struct {
		Files []struct {
			ID           int    `json:"id"`
			OriginalName string `json:"originalName"`
			DisplayName  string `json:"displayName"`
			Description  string `json:"description"`
			Version      int    `json:"latestFileVersionNo"`
			ProjectID    int    `json:"projectId"`
		} `json:"files"`
		Meta struct {
			Page struct {
				Count int `json:"count"`
			} `json:"page"`
		} `json:"meta"`
	}
	_ = json.Unmarshal(data, &resp)
	included := api.ParseIncluded(data)

	headers := []string{"ID", "NAME", "VERSION", "PROJECT", "DESCRIPTION"}
	rows := make([][]string, len(resp.Files))
	for i, f := range resp.Files {
		name := f.DisplayName
		if name == "" {
			name = f.OriginalName
		}
		project := included.LookupString("projects", fmt.Sprintf("%d", f.ProjectID), "name")
		rows[i] = []string{
			fmt.Sprintf("%d", f.ID),
			format.Truncate(name, 35),
			fmt.Sprintf("%d", f.Version),
			format.Truncate(project, 25),
			format.Truncate(f.Description, 35),
		}
	}
	if mode == format.CSV {
		format.PrintCSV(headers, rows)
	} else {
		format.PrintTable(os.Stdout, headers, rows)
		fmt.Printf("\nPage %d · %d of %d file(s)\n", page, len(resp.Files), resp.Meta.Page.Count)
	}
}

func runFilesUpload(cmd *cobra.Command, args []string) {
	client := getClient()
	projectQ, _ := cmd.Flags().GetString("project")
	taskQ, _ := cmd.Flags().GetString("task")
	paths, _ := cmd.Flags().GetStringArray("file")
	desc, _ := cmd.Flags().GetString("description")
	cats, _ := cmd.Flags().GetStringSlice("category")
	reopen, _ := cmd.Flags().GetBool("reopen")

	if len(paths) == 0 {
		fmt.Fprintln(os.Stderr, "Error: --file is required")
		exitFn(1)
	}
	if (projectQ == "") == (taskQ == "") {
		fmt.Fprintln(os.Stderr, "Error: provide exactly one of --project or --task")
		exitFn(1)
	}

	if taskQ != "" {
		uploadFilesToTask(client, taskQ, paths, desc, cats, reopen)
		return
	}

	pid, err := getResolver().Project(projectQ)
	if err != nil {
		exitOnError(err)
	}
	for _, path := range paths {
		uploadFileToProject(client, pid, path, desc, cats)
	}
}

func uploadFileToProject(client *api.Client, pid int, path, desc string, cats []string) {
	ref := uploadPending(client, path)

	attach := map[string]interface{}{"pendingFileRef": ref}
	if desc != "" {
		attach["description"] = desc
	}
	if len(cats) > 0 {
		attach["category-ids"] = strings.Join(cats, ",")
	}
	payload := map[string]interface{}{"file": attach}

	resp, err := client.Post(fmt.Sprintf("/projects/%d/files.json", pid), nil, payload)
	if err != nil {
		exitOnError(err)
	}
	var attached struct {
		FileID json.Number `json:"fileId"`
		ID     json.Number `json:"id"`
		Status string      `json:"STATUS"`
	}
	_ = json.Unmarshal(resp, &attached)
	id := attached.FileID.String()
	if id == "" {
		id = attached.ID.String()
	}
	if id == "" {
		fmt.Printf("Uploaded %s to project %d.\n", path, pid)
	} else {
		fmt.Printf("Uploaded %s as file %s on project %d.\n", path, id, pid)
	}
}

// uploadFilesToTask attaches files to the task itself, not just its project.
// Only `pendingFileAttachments` on the v1 task update does this — `attachments`
// wants numeric file ids and `attachmentIds` is accepted and silently ignored.
func uploadFilesToTask(client *api.Client, taskQ string, paths []string, desc string, cats []string, reopen bool) {
	if desc != "" || len(cats) > 0 {
		fmt.Fprintln(os.Stderr, "Error: --description and --category apply to --project uploads only")
		exitFn(1)
	}
	taskID, err := resolveTaskID(taskQ)
	if err != nil {
		exitOnError(err)
	}

	// Check before spending the upload: a completed task rejects the attach
	// with a permission error rather than anything that names the real cause.
	wasCompleted, err := taskIsCompleted(client, taskID)
	if err != nil {
		exitOnError(err)
	}
	if wasCompleted && !reopen {
		fmt.Fprintf(os.Stderr, "Error: task %d is completed — Teamwork rejects attachments on completed tasks.\n"+
			"Re-run with --reopen to reopen it, attach, and re-complete it.\n", taskID)
		exitFn(1)
	}

	refs := make([]string, len(paths))
	for i, path := range paths {
		refs[i] = uploadPending(client, path)
	}

	if wasCompleted {
		if _, err := client.Put(fmt.Sprintf("/tasks/%d/uncomplete.json", taskID), nil, nil); err != nil {
			exitOnError(err)
		}
	}

	payload := map[string]interface{}{"todo-item": map[string]interface{}{
		"updateFiles":            true,
		"pendingFileAttachments": strings.Join(refs, ","),
	}}
	resp, attachErr := client.Put(fmt.Sprintf("/tasks/%d.json", taskID), nil, payload)

	// Put the task back the way we found it even if the attach failed.
	if wasCompleted {
		if _, err := client.Put(fmt.Sprintf("/tasks/%d/complete.json", taskID), nil, nil); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: attached but could not re-complete task %d: %v\n", taskID, err)
		}
	}
	if attachErr != nil {
		exitOnError(attachErr)
	}

	var attached struct {
		AssignedFileIDs []json.Number `json:"assignedFileIds"`
	}
	_ = json.Unmarshal(resp, &attached)
	ids := make([]string, len(attached.AssignedFileIDs))
	for i, n := range attached.AssignedFileIDs {
		ids[i] = n.String()
	}
	if len(ids) == len(paths) {
		for i, path := range paths {
			fmt.Printf("Uploaded %s as file %s on task %d.\n", path, ids[i], taskID)
		}
		return
	}
	fmt.Printf("Uploaded %d file(s) to task %d.\n", len(paths), taskID)
}

// uploadPending POSTs the bytes and returns the pending ref. The response shape
// varies — older servers return {"pendingFile":{"ref":"…"}}, newer ones drop the
// wrapper and put ref/pendingFileRef at the top level.
func uploadPending(client *api.Client, path string) string {
	pending, err := client.Upload("/pendingfiles.json", "file", path)
	if err != nil {
		exitOnError(err)
	}
	ref := pendingFileRef(pending)
	if ref == "" {
		fmt.Fprintln(os.Stderr, "Error: upload succeeded but no pendingFileRef returned:", string(pending))
		exitFn(1)
	}
	return ref
}

// taskIsCompleted reports whether the task is already closed, so the caller can
// reopen it before attaching.
func taskIsCompleted(client *api.Client, taskID int) (bool, error) {
	data, err := client.Get(fmt.Sprintf("/projects/api/v3/tasks/%d.json", taskID), nil)
	if err != nil {
		return false, err
	}
	var resp struct {
		Task struct {
			Status string `json:"status"`
		} `json:"task"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return false, err
	}
	return resp.Task.Status == "completed", nil
}

// pendingFileRef pulls the pending upload reference from the various response
// shapes Teamwork returns from /pendingfiles.json.
func pendingFileRef(body json.RawMessage) string {
	var top struct {
		Ref            string `json:"ref"`
		PendingFileRef string `json:"pendingFileRef"`
		PendingFile    struct {
			Ref string `json:"ref"`
		} `json:"pendingFile"`
	}
	if err := json.Unmarshal(body, &top); err == nil {
		if top.Ref != "" {
			return top.Ref
		}
		if top.PendingFileRef != "" {
			return top.PendingFileRef
		}
		if top.PendingFile.Ref != "" {
			return top.PendingFile.Ref
		}
	}
	return ""
}

func runFilesShow(cmd *cobra.Command, args []string) {
	client := getClient()
	mode := getOutputMode()
	data, err := client.Get("/projects/api/v3/files/"+args[0]+".json?include=projects", nil)
	if err != nil {
		exitOnError(err)
	}
	if mode == format.JSON {
		format.PrintJSON(data)
		return
	}
	wrap, _ := decodeMap(data)
	f, _ := wrap["file"].(map[string]interface{})
	if f == nil {
		format.PrintJSON(data)
		return
	}
	for _, field := range []struct{ label, key string }{
		{"ID", "id"}, {"Name", "displayName"}, {"Original", "originalName"},
		{"Description", "description"}, {"Version", "latestFileVersionNo"},
	} {
		if v, ok := f[field.key]; ok && v != nil {
			val := fmt.Sprintf("%v", v)
			if val == "" {
				continue
			}
			fmt.Printf("%-13s %s\n", field.label+":", val)
		}
	}
}
