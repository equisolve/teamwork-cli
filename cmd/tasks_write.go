package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/equisolve/teamwork-cli/internal/api"
	"github.com/spf13/cobra"
)

var tasksCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new task in a task list",
	Run:   runTasksCreate,
}

var tasksUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update fields on an existing task",
	Args:  cobra.ExactArgs(1),
	Run:   runTasksUpdate,
}

var tasksDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a task",
	Args:  cobra.ExactArgs(1),
	Run:   runTasksDelete,
}

var tasksUncompleteCmd = &cobra.Command{
	Use:   "uncomplete <id>",
	Short: "Reopen a completed task",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := getClient()
		if _, err := client.Put("/tasks/"+args[0]+"/uncomplete.json", nil, nil); err != nil {
			exitOnError(err)
		}
		fmt.Printf("Task %s reopened.\n", args[0])
	},
}

var tasksSubtasksCmd = &cobra.Command{
	Use:   "subtasks <parent-id>",
	Short: "List subtasks or add several at once",
	Args:  cobra.ExactArgs(1),
	Run:   runTasksSubtasks,
}

var tasksSweepCmd = &cobra.Command{
	Use:   "sweep",
	Short: "Bucket open tasks in a list (Done/N/A/QA/Blocked) and optionally batch-close",
	Long: `Walks the open tasks of a task list and bucket-classifies each one by name
prefix. Buckets default to:
    done    ^(Done|DONE):
    na      ^(N/A|NA):
    qa      ^(QA|Q/A):
    blocked ^(Blocked|BLOCKED|Hold):

By default this is a dry run — pass --close <bucket> (repeatable) to actually
mark those tasks complete. Use --comment "text" to attach a comment before
closing (handy for N/A explanations).`,
	Run: runTasksSweep,
}

func init() {
	tasksCreateCmd.Flags().String("tasklist", "", "Task list ID (required)")
	tasksCreateCmd.Flags().String("name", "", "Task name/content (required)")
	tasksCreateCmd.Flags().String("description", "", "Long description")
	tasksCreateCmd.Flags().String("assignee", "", "Assignee ID, email, name, or 'me'")
	tasksCreateCmd.Flags().String("due", "", "Due date YYYY-MM-DD")
	tasksCreateCmd.Flags().String("start", "", "Start date YYYY-MM-DD")
	tasksCreateCmd.Flags().String("priority", "", "Priority: low, medium, high")
	tasksCreateCmd.Flags().Int("estimate", 0, "Estimated minutes")

	tasksUpdateCmd.Flags().String("name", "", "New task name/content")
	tasksUpdateCmd.Flags().String("description", "", "New description")
	tasksUpdateCmd.Flags().String("assignee", "", "New assignee ID, email, name, or 'me'")
	tasksUpdateCmd.Flags().String("due", "", "New due date YYYY-MM-DD")
	tasksUpdateCmd.Flags().String("start", "", "New start date YYYY-MM-DD")
	tasksUpdateCmd.Flags().String("priority", "", "New priority: low, medium, high")
	tasksUpdateCmd.Flags().Int("estimate", -1, "New estimated minutes (-1 = leave alone)")

	tasksDeleteCmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")

	tasksSubtasksCmd.Flags().String("add", "", "Add subtasks; newline-separated or ~|~")

	tasksSweepCmd.Flags().String("tasklist", "", "Task list ID to sweep (required unless --project + --list-name)")
	tasksSweepCmd.Flags().StringP("project", "p", "", "Project ID/name (with --list-name to resolve a tasklist)")
	tasksSweepCmd.Flags().String("list-name", "", "Tasklist name match (case-insensitive substring), used with --project")
	tasksSweepCmd.Flags().StringSlice("close", nil, "Bucket(s) to close: done, na, qa, blocked (repeatable)")
	tasksSweepCmd.Flags().String("comment", "", "Comment to add to each task before closing (e.g. \"N/A for this site type\")")
	tasksSweepCmd.Flags().BoolP("yes", "y", false, "Skip confirmation when closing")
	tasksSweepCmd.Flags().Bool("with-predecessors", false, "Auto-close any incomplete predecessors first")

	tasksCmd.AddCommand(tasksCreateCmd, tasksUpdateCmd, tasksDeleteCmd, tasksUncompleteCmd, tasksSubtasksCmd, tasksSweepCmd)
}

func runTasksCreate(cmd *cobra.Command, args []string) {
	client := getClient()
	tasklist, _ := cmd.Flags().GetString("tasklist")
	name, _ := cmd.Flags().GetString("name")
	if tasklist == "" || name == "" {
		fmt.Fprintln(os.Stderr, "Error: --tasklist and --name are required")
		exitFn(1)
	}

	todo := map[string]interface{}{"content": name}
	if v, _ := cmd.Flags().GetString("description"); v != "" {
		todo["description"] = v
	}
	if v, _ := cmd.Flags().GetString("assignee"); v != "" {
		uid, err := getResolver().Person(v)
		if err != nil {
			exitOnError(err)
		}
		todo["responsible-party-id"] = strconv.Itoa(uid)
	}
	if v, _ := cmd.Flags().GetString("due"); v != "" {
		todo["due-date"] = strings.ReplaceAll(v, "-", "")
	}
	if v, _ := cmd.Flags().GetString("start"); v != "" {
		todo["start-date"] = strings.ReplaceAll(v, "-", "")
	}
	if v, _ := cmd.Flags().GetString("priority"); v != "" {
		todo["priority"] = v
	}
	if v, _ := cmd.Flags().GetInt("estimate"); v > 0 {
		todo["estimated-minutes"] = v
	}

	payload := map[string]interface{}{"todo-item": todo}
	data, err := client.Post("/tasklists/"+tasklist+"/tasks.json", nil, payload)
	if err != nil {
		exitOnError(err)
	}
	var resp struct {
		ID       string `json:"id"`
		TaskID   string `json:"taskId"`
	}
	_ = json.Unmarshal(data, &resp)
	id := resp.TaskID
	if id == "" {
		id = resp.ID
	}
	fmt.Printf("Task created: %s\n", id)
}

func runTasksUpdate(cmd *cobra.Command, args []string) {
	client := getClient()
	todo := map[string]interface{}{}
	if v, _ := cmd.Flags().GetString("name"); v != "" {
		todo["content"] = v
	}
	if v, _ := cmd.Flags().GetString("description"); v != "" {
		todo["description"] = v
	}
	if v, _ := cmd.Flags().GetString("assignee"); v != "" {
		uid, err := getResolver().Person(v)
		if err != nil {
			exitOnError(err)
		}
		todo["responsible-party-id"] = strconv.Itoa(uid)
	}
	if v, _ := cmd.Flags().GetString("due"); v != "" {
		todo["due-date"] = strings.ReplaceAll(v, "-", "")
	}
	if v, _ := cmd.Flags().GetString("start"); v != "" {
		todo["start-date"] = strings.ReplaceAll(v, "-", "")
	}
	if v, _ := cmd.Flags().GetString("priority"); v != "" {
		todo["priority"] = v
	}
	if v, _ := cmd.Flags().GetInt("estimate"); v >= 0 {
		todo["estimated-minutes"] = v
	}
	if len(todo) == 0 {
		fmt.Fprintln(os.Stderr, "Error: no updates specified; pass at least one flag")
		exitFn(1)
	}

	payload := map[string]interface{}{"todo-item": todo}
	if _, err := client.Put("/tasks/"+args[0]+".json", nil, payload); err != nil {
		exitOnError(err)
	}
	fmt.Printf("Task %s updated.\n", args[0])
}

func runTasksDelete(cmd *cobra.Command, args []string) {
	client := getClient()
	skip, _ := cmd.Flags().GetBool("yes")
	if !skip && !confirm(fmt.Sprintf("Delete task %s?", args[0])) {
		fmt.Println("Aborted.")
		return
	}
	if _, err := client.Delete("/tasks/"+args[0]+".json", nil); err != nil {
		exitOnError(err)
	}
	fmt.Printf("Task %s deleted.\n", args[0])
}

func runTasksSubtasks(cmd *cobra.Command, args []string) {
	client := getClient()
	add, _ := cmd.Flags().GetString("add")
	if add != "" {
		// Split on newline or literal ~|~; one POST per subtask.
		raw := strings.ReplaceAll(add, "~|~", "\n")
		var names []string
		for _, line := range strings.Split(raw, "\n") {
			if s := strings.TrimSpace(line); s != "" {
				names = append(names, s)
			}
		}
		if len(names) == 0 {
			fmt.Fprintln(os.Stderr, "Error: --add produced no non-empty subtask names")
			exitFn(1)
		}
		path := "/projects/api/v3/tasks/" + args[0] + "/subtasks.json"
		for _, name := range names {
			body := map[string]interface{}{
				"task": map[string]interface{}{"name": name},
			}
			if _, err := client.Post(path, nil, body); err != nil {
				exitOnError(err)
			}
		}
		fmt.Printf("Added %d subtask(s).\n", len(names))
		return
	}

	data, err := client.Get("/tasks/"+args[0]+"/subtasks.json", nil)
	if err != nil {
		exitOnError(err)
	}
	// Reuse v1 show-style raw print — these endpoints are v1 only.
	if string(getOutputMode()) == "json" {
		fmt.Println(string(data))
		return
	}
	var resp struct {
		TodoItems []struct {
			ID      json.Number `json:"id"`
			Content string      `json:"content"`
			Done    bool        `json:"completed"`
			Due     string      `json:"due-date"`
		} `json:"todo-items"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		fmt.Println(string(data))
		return
	}
	if len(resp.TodoItems) == 0 {
		fmt.Println("No subtasks.")
		return
	}
	for _, t := range resp.TodoItems {
		check := "[ ]"
		if t.Done {
			check = "[x]"
		}
		due := ""
		if t.Due != "" {
			due = " (due " + formatDate(t.Due) + ")"
		}
		fmt.Printf("%s %s · %s%s\n", check, t.ID.String(), t.Content, due)
	}
}

// sweepBucket maps a bucket key to its display label and prefix matcher. Order
// here is also the column order in the dry-run table.
type sweepBucket struct {
	key    string
	label  string
	prefix []string // case-insensitive prefix match on task name (after trimming)
}

type sweepTask struct {
	id   string
	name string
}

var defaultSweepBuckets = []sweepBucket{
	{"done", "Done", []string{"done:", "done -", "done —"}},
	{"na", "N/A", []string{"n/a:", "na:", "n/a -", "n/a —"}},
	{"qa", "QA", []string{"qa:", "q/a:", "qa -"}},
	{"blocked", "Blocked", []string{"blocked:", "blocked -", "hold:"}},
}

// classifySweep buckets a task name. Returns "" if no bucket matched.
func classifySweep(name string, buckets []sweepBucket) string {
	n := strings.ToLower(strings.TrimSpace(name))
	for _, b := range buckets {
		for _, p := range b.prefix {
			if strings.HasPrefix(n, p) {
				return b.key
			}
		}
	}
	return ""
}

func runTasksSweep(cmd *cobra.Command, args []string) {
	client := getClient()
	tasklist, _ := cmd.Flags().GetString("tasklist")
	projectQ, _ := cmd.Flags().GetString("project")
	listName, _ := cmd.Flags().GetString("list-name")
	closeBuckets, _ := cmd.Flags().GetStringSlice("close")
	comment, _ := cmd.Flags().GetString("comment")
	skipConfirm, _ := cmd.Flags().GetBool("yes")
	withPreds, _ := cmd.Flags().GetBool("with-predecessors")

	if tasklist == "" {
		if projectQ == "" || listName == "" {
			fmt.Fprintln(os.Stderr, "Error: pass --tasklist <id>, or --project + --list-name")
			exitFn(1)
		}
		id, err := resolveTasklistByName(client, projectQ, listName)
		if err != nil {
			exitOnError(err)
		}
		tasklist = strconv.Itoa(id)
	}

	tasks, err := fetchOpenTasks(client, tasklist)
	if err != nil {
		exitOnError(err)
	}

	groups := map[string][]sweepTask{}
	var unbucketed []sweepTask
	for _, t := range tasks {
		key := classifySweep(t.name, defaultSweepBuckets)
		if key == "" {
			unbucketed = append(unbucketed, t)
			continue
		}
		groups[key] = append(groups[key], t)
	}

	// Render dry-run summary regardless of close intent.
	printSweepSummary(groups, unbucketed)

	if len(closeBuckets) == 0 {
		fmt.Println("\nDry run only — pass --close <bucket> to actually mark these complete.")
		return
	}

	var toClose []sweepTask
	for _, b := range closeBuckets {
		key := strings.ToLower(strings.TrimSpace(b))
		toClose = append(toClose, groups[key]...)
	}
	if len(toClose) == 0 {
		fmt.Println("\nNothing matched the requested bucket(s). Aborting.")
		return
	}
	if !skipConfirm && !confirm(fmt.Sprintf("Close %d task(s)?", len(toClose))) {
		fmt.Println("Aborted.")
		return
	}

	visited := map[string]bool{}
	var done, failed int
	for _, t := range toClose {
		if comment != "" {
			payload := map[string]interface{}{
				"comment": map[string]interface{}{"body": comment, "content-type": "TEXT"},
			}
			if _, err := client.Post("/tasks/"+t.id+"/comments.json", nil, payload); err != nil {
				fmt.Fprintf(os.Stderr, "Task %s comment: %s\n", t.id, api.FormatError(err, ""))
			}
		}
		if err := completeTask(client, t.id, withPreds, visited); err != nil {
			failed++
			fmt.Fprintf(os.Stderr, "Task %s: %s\n", t.id, api.FormatError(err, ""))
			continue
		}
		done++
		fmt.Printf("✓ %s · %s\n", t.id, t.name)
	}
	fmt.Printf("\nClosed %d of %d (%d failed).\n", done, len(toClose), failed)
	if failed > 0 {
		exitFn(1)
	}
}

func printSweepSummary(groups map[string][]sweepTask, unbucketed []sweepTask) {
	for _, b := range defaultSweepBuckets {
		entries := groups[b.key]
		fmt.Printf("\n%s (%d)\n", b.label, len(entries))
		for _, e := range entries {
			fmt.Printf("  %s · %s\n", e.id, e.name)
		}
	}
	if len(unbucketed) > 0 {
		fmt.Printf("\nUnbucketed (%d) — left open\n", len(unbucketed))
		for _, e := range unbucketed {
			fmt.Printf("  %s · %s\n", e.id, e.name)
		}
	}
}

// fetchOpenTasks returns id+name for every non-completed task in the list.
func fetchOpenTasks(client *api.Client, tasklistID string) ([]sweepTask, error) {
	params := url.Values{}
	params.Set("tasklistIds", tasklistID)
	params.Set("pageSize", "200")
	body, err := client.Get("/projects/api/v3/tasks.json", params)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Tasks []struct {
			ID     int    `json:"id"`
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	out := make([]sweepTask, 0, len(resp.Tasks))
	for _, t := range resp.Tasks {
		if t.Status == "completed" {
			continue
		}
		out = append(out, sweepTask{id: strconv.Itoa(t.ID), name: t.Name})
	}
	return out, nil
}

func resolveTasklistByName(client *api.Client, projectQuery, listName string) (int, error) {
	pid, err := getResolver().Project(projectQuery)
	if err != nil {
		return 0, err
	}
	body, err := client.Get(fmt.Sprintf("/projects/%d/tasklists.json", pid), nil)
	if err != nil {
		return 0, err
	}
	var resp struct {
		Tasklists []struct {
			ID   json.Number `json:"id"`
			Name string      `json:"name"`
		} `json:"tasklists"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, err
	}
	needle := strings.ToLower(listName)
	var matches []struct {
		ID   json.Number
		Name string
	}
	for _, tl := range resp.Tasklists {
		if strings.Contains(strings.ToLower(tl.Name), needle) {
			matches = append(matches, struct {
				ID   json.Number
				Name string
			}{tl.ID, tl.Name})
		}
	}
	switch len(matches) {
	case 0:
		return 0, fmt.Errorf("no tasklist on project matches %q", listName)
	case 1:
		i, _ := matches[0].ID.Int64()
		return int(i), nil
	default:
		var preview []string
		for _, m := range matches {
			preview = append(preview, fmt.Sprintf("%s: %s", m.ID, m.Name))
		}
		return 0, fmt.Errorf("ambiguous tasklist match for %q:\n  %s", listName, strings.Join(preview, "\n  "))
	}
}

// confirm reads a yes/no from stdin. Returns true on yes/y.
func confirm(prompt string) bool {
	fmt.Printf("%s [y/N]: ", prompt)
	reader := bufio.NewReader(os.Stdin)
	resp, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	resp = strings.TrimSpace(strings.ToLower(resp))
	return resp == "y" || resp == "yes"
}
