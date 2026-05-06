package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/equisolve/teamwork-cli/internal/api"
	"github.com/equisolve/teamwork-cli/internal/format"
	"github.com/spf13/cobra"
)

var tasksCmd = &cobra.Command{
	Use:     "tasks",
	Aliases: []string{"task"},
	Short:   "List, view, and update Teamwork tasks",
}

var tasksListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tasks (all projects, or filtered)",
	Run:   runTasksList,
}

var tasksShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show task details",
	Args:  cobra.ExactArgs(1),
	Run:   runTasksShow,
}

var tasksCompleteCmd = &cobra.Command{
	Use:   "complete <id>...",
	Short: "Mark one or more tasks complete (reads stdin with --from-stdin)",
	Args:  cobra.ArbitraryArgs,
	Run:   runTasksComplete,
}

func init() {
	tasksListCmd.Flags().StringP("project", "p", "", "Filter by project ID or name")
	tasksListCmd.Flags().String("tasklist", "", "Filter by tasklist ID")
	tasksListCmd.Flags().String("assignee", "", "Filter by assignee ID, name, email, or 'me'")
	tasksListCmd.Flags().String("status", "", "Preset filter: upcoming, late (v3 ignores other values)")
	tasksListCmd.Flags().Bool("completed", false, "Show only completed tasks (due-from/due-to filter by completion date)")
	tasksListCmd.Flags().String("due-from", "", "Due date range start YYYY-MM-DD (completion date when --completed)")
	tasksListCmd.Flags().String("due-to", "", "Due date range end YYYY-MM-DD (completion date when --completed)")
	tasksListCmd.Flags().Int("page", 1, "Page number")
	tasksListCmd.Flags().Int("page-size", 50, "Results per page")

	tasksCompleteCmd.Flags().Bool("from-stdin", false, "Read whitespace-separated task IDs from stdin")
	tasksCompleteCmd.Flags().Bool("continue-on-error", false, "Keep going past failures when batch-closing")
	tasksCompleteCmd.Flags().Bool("with-predecessors", false, "Auto-close any incomplete predecessors first")

	tasksCmd.AddCommand(tasksListCmd, tasksShowCmd, tasksCompleteCmd)
	rootCmd.AddCommand(tasksCmd)
}

func runTasksList(cmd *cobra.Command, args []string) {
	client := getClient()
	mode := getOutputMode()
	params := url.Values{}
	params.Set("include", "projects,users")

	projectQuery, _ := cmd.Flags().GetString("project")
	if projectQuery != "" {
		pid, err := getResolver().Project(projectQuery)
		if err != nil {
			exitOnError(err)
		}
		params.Set("projectIds", fmt.Sprintf("%d", pid))
	}
	if v, _ := cmd.Flags().GetString("tasklist"); v != "" {
		params.Set("tasklistIds", v)
	}
	if v, _ := cmd.Flags().GetString("assignee"); v != "" {
		uid, err := getResolver().Person(v)
		if err != nil {
			exitOnError(err)
		}
		params.Set("responsiblePartyIds", fmt.Sprintf("%d", uid))
	}
	if v, _ := cmd.Flags().GetString("status"); v != "" {
		params.Set("status", v)
	}
	onlyCompleted, _ := cmd.Flags().GetBool("completed")
	dueFrom, _ := cmd.Flags().GetString("due-from")
	dueTo, _ := cmd.Flags().GetString("due-to")
	if onlyCompleted {
		params.Set("includeCompletedTasks", "true")
		if dueFrom != "" {
			params.Set("completedAfter", dueFrom)
		}
		if dueTo != "" {
			params.Set("completedBefore", dueTo)
		}
	} else {
		if dueFrom != "" {
			params.Set("startDate", dueFrom)
		}
		if dueTo != "" {
			params.Set("endDate", dueTo)
		}
	}
	page, _ := cmd.Flags().GetInt("page")
	pageSize, _ := cmd.Flags().GetInt("page-size")
	params.Set("page", fmt.Sprintf("%d", page))
	params.Set("pageSize", fmt.Sprintf("%d", pageSize))

	data, err := client.Get("/projects/api/v3/tasks.json", params)
	if err != nil {
		exitOnError(err)
	}

	if mode == format.JSON {
		format.PrintJSON(data)
		return
	}

	var resp struct {
		Tasks []struct {
			ID          int    `json:"id"`
			Name        string `json:"name"`
			Status      string `json:"status"`
			Priority    string `json:"priority"`
			Progress    int    `json:"progress"`
			DueDate     string `json:"dueDate"`
			StartDate   string `json:"startDate"`
			Assignees   []struct {
				ID   int    `json:"id"`
				Type string `json:"type"`
			} `json:"assignees"`
			TasklistID int `json:"tasklistId"`
		} `json:"tasks"`
		Meta struct {
			Page struct {
				Count int `json:"count"`
			} `json:"page"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		fmt.Fprintln(os.Stderr, "Error parsing response:", err)
		exitFn(1)
	}
	included := api.ParseIncluded(data)

	if onlyCompleted {
		filtered := resp.Tasks[:0]
		for _, t := range resp.Tasks {
			if t.Status == "completed" {
				filtered = append(filtered, t)
			}
		}
		resp.Tasks = filtered
	}

	headers := []string{"ID", "TASK", "ASSIGNEE", "DUE", "PRIORITY", "STATUS"}
	rows := make([][]string, len(resp.Tasks))
	for i, t := range resp.Tasks {
		assignee := ""
		if len(t.Assignees) > 0 {
			first := included.LookupString("users", fmt.Sprintf("%d", t.Assignees[0].ID), "firstName")
			last := included.LookupString("users", fmt.Sprintf("%d", t.Assignees[0].ID), "lastName")
			assignee = strings.TrimSpace(first + " " + last)
			if len(t.Assignees) > 1 {
				assignee += fmt.Sprintf(" (+%d)", len(t.Assignees)-1)
			}
		}
		rows[i] = []string{
			fmt.Sprintf("%d", t.ID),
			format.Truncate(t.Name, 45),
			format.Truncate(assignee, 20),
			formatDate(t.DueDate),
			t.Priority,
			t.Status,
		}
	}

	if mode == format.CSV {
		format.PrintCSV(headers, rows)
	} else {
		format.PrintTable(os.Stdout, headers, rows)
		fmt.Printf("\nPage %d · %d of %d task(s)\n", page, len(resp.Tasks), resp.Meta.Page.Count)
	}
}

func runTasksShow(cmd *cobra.Command, args []string) {
	client := getClient()
	mode := getOutputMode()

	data, err := client.Get("/projects/api/v3/tasks/"+args[0]+".json?include=projects,users,tasklists", nil)
	if err != nil {
		exitOnError(err)
	}

	if mode == format.JSON {
		format.PrintJSON(data)
		return
	}

	wrap, err := decodeMap(data)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error parsing response:", err)
		exitFn(1)
	}
	t, _ := wrap["task"].(map[string]interface{})
	if t == nil {
		format.PrintJSON(data)
		return
	}
	included := api.ParseIncluded(data)

	fields := []struct{ label, key string }{
		{"ID", "id"},
		{"Name", "name"},
		{"Description", "description"},
		{"Status", "status"},
		{"Priority", "priority"},
		{"Progress", "progress"},
		{"Start", "startDate"},
		{"Due", "dueDate"},
		{"Estimated min", "estimateMinutes"},
		{"Created", "createdAt"},
		{"Updated", "updatedAt"},
	}
	for _, f := range fields {
		if v, ok := t[f.key]; ok && v != nil {
			val := fmt.Sprintf("%v", v)
			if val == "" || val == "<nil>" {
				continue
			}
			if strings.HasSuffix(f.key, "Date") || strings.HasSuffix(f.key, "At") {
				val = formatDate(val)
			}
			fmt.Printf("%-14s %s\n", f.label+":", val)
		}
	}

	if assignees, ok := t["assignees"].([]interface{}); ok && len(assignees) > 0 {
		var names []string
		for _, a := range assignees {
			m, _ := a.(map[string]interface{})
			if m == nil {
				continue
			}
			id := fmt.Sprintf("%v", m["id"])
			first := included.LookupString("users", id, "firstName")
			last := included.LookupString("users", id, "lastName")
			name := strings.TrimSpace(first + " " + last)
			if name != "" {
				names = append(names, name)
			}
		}
		if len(names) > 0 {
			fmt.Printf("%-14s %s\n", "Assignees:", strings.Join(names, ", "))
		}
	}

	if preds := taskPredecessorIDs(t); len(preds) > 0 {
		var parts []string
		for _, p := range preds {
			mark := ""
			if !p.completed {
				mark = " ⚠"
			}
			parts = append(parts, fmt.Sprintf("%d%s", p.id, mark))
		}
		fmt.Printf("%-14s %s\n", "Predecessors:", strings.Join(parts, ", "))
	}
}

// predecessor is the minimum we extract from a v3 task response. Teamwork
// returns predecessor objects under a few different shapes depending on the
// endpoint version, so we read defensively.
type predecessor struct {
	id        int
	completed bool
}

// taskPredecessorIDs walks `predecessors` on a v3 task body and returns the
// referenced task IDs plus a best-effort "completed" hint when the API ships
// it sideloaded.
func taskPredecessorIDs(task map[string]interface{}) []predecessor {
	raw, ok := task["predecessors"].([]interface{})
	if !ok {
		return nil
	}
	var out []predecessor
	for _, item := range raw {
		m, _ := item.(map[string]interface{})
		if m == nil {
			continue
		}
		// Try id, taskId, predecessorId — different responses use different
		// keys; prefer the one that points at the predecessor task.
		var id int
		for _, k := range []string{"taskId", "predecessorId", "id"} {
			if v, ok := m[k]; ok && v != nil {
				if n, ok := v.(json.Number); ok {
					if i, err := n.Int64(); err == nil && i > 0 {
						id = int(i)
						break
					}
				}
				if f, ok := v.(float64); ok && f > 0 {
					id = int(f)
					break
				}
			}
		}
		if id == 0 {
			continue
		}
		done := false
		if v, ok := m["completed"].(bool); ok {
			done = v
		} else if s, ok := m["status"].(string); ok && s == "completed" {
			done = true
		}
		out = append(out, predecessor{id: id, completed: done})
	}
	return out
}

// completeTask wraps PUT /tasks/<id>/complete.json with optional recursive
// predecessor closing. visited prevents loops in cyclic predecessor graphs.
func completeTask(client *api.Client, id string, withPredecessors bool, visited map[string]bool) error {
	if visited[id] {
		return nil
	}
	visited[id] = true

	_, err := client.Put("/tasks/"+id+"/complete.json", nil, nil)
	if err == nil {
		return nil
	}
	if !withPredecessors {
		return err
	}
	// Only retry the predecessor branch when the API specifically blocks on them.
	apiErr, _ := err.(*api.APIError)
	if apiErr == nil || !looksLikePredecessorError(apiErr.Message) {
		return err
	}

	body, fetchErr := client.Get("/projects/api/v3/tasks/"+id+".json", nil)
	if fetchErr != nil {
		return err
	}
	wrap, _ := decodeMap(body)
	t, _ := wrap["task"].(map[string]interface{})
	preds := taskPredecessorIDs(t)
	if len(preds) == 0 {
		return err
	}
	for _, p := range preds {
		if p.completed {
			continue
		}
		if perr := completeTask(client, fmt.Sprintf("%d", p.id), true, visited); perr != nil {
			return fmt.Errorf("predecessor %d: %w", p.id, perr)
		}
	}
	_, err = client.Put("/tasks/"+id+"/complete.json", nil, nil)
	return err
}

func looksLikePredecessorError(msg string) bool {
	m := strings.ToLower(msg)
	return strings.Contains(m, "predecessor") || strings.Contains(m, "dependency") || strings.Contains(m, "depend")
}

func runTasksComplete(cmd *cobra.Command, args []string) {
	client := getClient()
	fromStdin, _ := cmd.Flags().GetBool("from-stdin")
	continueOnError, _ := cmd.Flags().GetBool("continue-on-error")
	withPreds, _ := cmd.Flags().GetBool("with-predecessors")

	ids := append([]string{}, args...)
	if fromStdin {
		stdinIDs, err := readIDs(os.Stdin)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error reading stdin:", err)
			exitFn(1)
		}
		ids = append(ids, stdinIDs...)
	}
	if len(ids) == 0 {
		fmt.Fprintln(os.Stderr, "Error: pass at least one task ID, or use --from-stdin")
		exitFn(1)
	}

	visited := map[string]bool{}
	var done, failed int
	for _, id := range ids {
		// Fresh visited set per top-level id is fine; we still avoid loops
		// inside any one predecessor traversal.
		if err := completeTask(client, id, withPreds, visited); err != nil {
			failed++
			fmt.Fprintf(os.Stderr, "Task %s: %s\n", id, api.FormatError(err, ""))
			if apiErr, _ := err.(*api.APIError); apiErr != nil && looksLikePredecessorError(apiErr.Message) && !withPreds {
				fmt.Fprintln(os.Stderr, "  hint: rerun with --with-predecessors to auto-close them first")
			}
			if !continueOnError {
				exitFn(1)
			}
			continue
		}
		done++
		fmt.Printf("Task %s marked complete.\n", id)
	}
	if len(ids) > 1 {
		fmt.Printf("\nCompleted %d of %d (%d failed).\n", done, len(ids), failed)
	}
	if failed > 0 && continueOnError {
		exitFn(1)
	}
}

// readIDs pulls whitespace-separated task IDs from a reader. Blank lines and
// `#`-prefixed comments are ignored so output from `tasks list --json | jq`
// pipes cleanly.
func readIDs(r io.Reader) ([]string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, tok := range strings.Fields(string(data)) {
		if strings.HasPrefix(tok, "#") {
			continue
		}
		if _, err := strconv.Atoi(tok); err != nil {
			continue
		}
		out = append(out, tok)
	}
	return out, nil
}
