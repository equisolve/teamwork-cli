package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/equisolve/teamwork-cli/internal/format"
	"github.com/spf13/cobra"
)

var tasklistsCmd = &cobra.Command{
	Use:     "tasklists",
	Aliases: []string{"tasklist", "lists"},
	Short:   "List and view task lists in a project",
}

var tasklistsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List task lists for a project",
	Run:   runTasklistsList,
}

var tasklistsShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show task list details",
	Args:  cobra.ExactArgs(1),
	Run:   runTasklistsShow,
}

func init() {
	tasklistsListCmd.Flags().StringP("project", "p", "", "Project ID or name (required)")
	tasklistsListCmd.Flags().Bool("completed", false, "Include completed lists")
	tasklistsListCmd.Flags().Int("page", 1, "Page number")
	tasklistsListCmd.Flags().Int("page-size", 50, "Results per page")

	tasklistsCmd.AddCommand(tasklistsListCmd, tasklistsShowCmd)
	rootCmd.AddCommand(tasklistsCmd)
}

func runTasklistsList(cmd *cobra.Command, args []string) {
	client := getClient()
	mode := getOutputMode()

	projectQ, _ := cmd.Flags().GetString("project")
	if projectQ == "" {
		fmt.Fprintln(os.Stderr, "Error: --project is required")
		exitFn(1)
	}
	pid, err := getResolver().Project(projectQ)
	if err != nil {
		exitOnError(err)
	}

	params := url.Values{}
	if v, _ := cmd.Flags().GetBool("completed"); v {
		params.Set("showCompleted", "true")
	}
	page, _ := cmd.Flags().GetInt("page")
	pageSize, _ := cmd.Flags().GetInt("page-size")
	params.Set("page", fmt.Sprintf("%d", page))
	params.Set("pageSize", fmt.Sprintf("%d", pageSize))

	data, err := client.Get(fmt.Sprintf("/projects/%d/tasklists.json", pid), params)
	if err != nil {
		exitOnError(err)
	}

	if mode == format.JSON {
		format.PrintJSON(data)
		return
	}

	var resp struct {
		Tasklists []struct {
			ID        json.Number `json:"id"`
			Name      string      `json:"name"`
			Completed bool        `json:"complete"`
			Milestone string      `json:"milestone-id"`
			UncompletedCount json.Number `json:"uncompleted-count"`
		} `json:"tasklists"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		fmt.Fprintln(os.Stderr, "Error parsing response:", err)
		exitFn(1)
	}

	headers := []string{"ID", "NAME", "OPEN TASKS", "DONE"}
	rows := make([][]string, len(resp.Tasklists))
	for i, tl := range resp.Tasklists {
		done := ""
		if tl.Completed {
			done = "Y"
		}
		rows[i] = []string{
			tl.ID.String(),
			format.Truncate(tl.Name, 45),
			tl.UncompletedCount.String(),
			done,
		}
	}

	if mode == format.CSV {
		format.PrintCSV(headers, rows)
	} else {
		format.PrintTable(os.Stdout, headers, rows)
		fmt.Printf("\n%d list(s)\n", len(resp.Tasklists))
	}
}

func runTasklistsShow(cmd *cobra.Command, args []string) {
	client := getClient()
	mode := getOutputMode()

	data, err := client.Get("/tasklists/"+args[0]+".json", nil)
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
	tl, _ := wrap["todo-list"].(map[string]interface{})
	if tl == nil {
		format.PrintJSON(data)
		return
	}

	fields := []struct{ label, key string }{
		{"ID", "id"},
		{"Name", "name"},
		{"Description", "description"},
		{"Project", "project-name"},
		{"Milestone", "milestone-id"},
		{"Complete", "complete"},
		{"Uncompleted", "uncompleted-count"},
		{"Created", "created-on"},
	}
	for _, f := range fields {
		if v, ok := tl[f.key]; ok && v != nil {
			val := fmt.Sprintf("%v", v)
			if val == "" {
				continue
			}
			if strings.HasSuffix(f.key, "-on") {
				val = formatDate(val)
			}
			fmt.Printf("%-14s %s\n", f.label+":", val)
		}
	}
}
