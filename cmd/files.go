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
	Short: "Upload a file and attach it to a project",
	Long: `Two-step v1 upload: POST the bytes to /pendingfiles.json, then attach
the returned ref to /projects/<id>/files.json. Files must attach to a project
(Teamwork's task-file endpoint is unreliable).`,
	Run: runFilesUpload,
}

func init() {
	filesListCmd.Flags().StringP("project", "p", "", "Filter by project ID or name")
	filesListCmd.Flags().Int("page", 1, "Page number")
	filesListCmd.Flags().Int("page-size", 25, "Results per page")

	filesUploadCmd.Flags().StringP("project", "p", "", "Project ID or name (required)")
	filesUploadCmd.Flags().String("file", "", "Path to the file to upload (required)")
	filesUploadCmd.Flags().String("description", "", "Optional description for the file")
	filesUploadCmd.Flags().StringSlice("category", nil, "Category ID(s) to attach the file to (repeatable)")

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
			ID          int    `json:"id"`
			OriginalName string `json:"originalName"`
			DisplayName string `json:"displayName"`
			Description string `json:"description"`
			Version     int    `json:"latestFileVersionNo"`
			ProjectID   int    `json:"projectId"`
		} `json:"files"`
		Meta struct {
			Page struct{ Count int `json:"count"` } `json:"page"`
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
	path, _ := cmd.Flags().GetString("file")
	desc, _ := cmd.Flags().GetString("description")
	cats, _ := cmd.Flags().GetStringSlice("category")

	if projectQ == "" || path == "" {
		fmt.Fprintln(os.Stderr, "Error: --project and --file are required")
		exitFn(1)
	}
	pid, err := getResolver().Project(projectQ)
	if err != nil {
		exitOnError(err)
	}

	// Step 1: pending upload (multipart). The response shape varies — older
	// servers return {"pendingFile":{"ref":"…"}}, newer ones drop the wrapper
	// and put ref/pendingFileRef at the top level.
	pending, err := client.Upload("/pendingfiles.json", "file", path)
	if err != nil {
		exitOnError(err)
	}
	ref := pendingFileRef(pending)
	if ref == "" {
		fmt.Fprintln(os.Stderr, "Error: upload succeeded but no pendingFileRef returned:", string(pending))
		exitFn(1)
	}

	// Step 2: attach to the project.
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
		FileID  json.Number `json:"fileId"`
		ID      json.Number `json:"id"`
		Status  string      `json:"STATUS"`
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
