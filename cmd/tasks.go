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
	Use:   "complete <id>",
	Short: "Mark a task complete",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := getClient()
		if _, err := client.Put("/tasks/"+args[0]+"/complete.json", nil, nil); err != nil {
			exitOnError(err)
		}
		fmt.Printf("Task %s marked complete.\n", args[0])
	},
}

func init() {
	tasksListCmd.Flags().StringP("project", "p", "", "Filter by project ID or name")
	tasksListCmd.Flags().String("assignee", "", "Filter by assignee ID, name, email, or 'me'")
	tasksListCmd.Flags().String("status", "", "Filter by status: new, reopened, completed, deleted")
	tasksListCmd.Flags().Bool("completed", false, "Include completed tasks")
	tasksListCmd.Flags().String("due-from", "", "Due date range start YYYY-MM-DD")
	tasksListCmd.Flags().String("due-to", "", "Due date range end YYYY-MM-DD")
	tasksListCmd.Flags().Int("page", 1, "Page number")
	tasksListCmd.Flags().Int("page-size", 50, "Results per page")

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
	if v, _ := cmd.Flags().GetString("assignee"); v != "" {
		uid, err := getResolver().Person(v)
		if err != nil {
			exitOnError(err)
		}
		params.Set("assignedToUserIds", fmt.Sprintf("%d", uid))
	}
	if v, _ := cmd.Flags().GetString("status"); v != "" {
		params.Set("status", v)
	}
	if v, _ := cmd.Flags().GetBool("completed"); v {
		params.Set("includeCompletedTasks", "true")
	}
	if v, _ := cmd.Flags().GetString("due-from"); v != "" {
		params.Set("startDate", strings.ReplaceAll(v, "-", ""))
	}
	if v, _ := cmd.Flags().GetString("due-to"); v != "" {
		params.Set("endDate", strings.ReplaceAll(v, "-", ""))
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
		{"Estimated min", "estimatedMinutes"},
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
}
