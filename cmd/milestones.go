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

var milestonesCmd = &cobra.Command{
	Use:     "milestones",
	Aliases: []string{"milestone"},
	Short:   "List and view project milestones",
}

var milestonesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List milestones",
	Run:   runMilestonesList,
}

var milestonesShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show milestone details",
	Args:  cobra.ExactArgs(1),
	Run:   runMilestonesShow,
}

func init() {
	milestonesListCmd.Flags().StringP("project", "p", "", "Filter by project ID or name")
	milestonesListCmd.Flags().Bool("completed", false, "Include completed milestones")
	milestonesListCmd.Flags().Int("page", 1, "Page number")
	milestonesListCmd.Flags().Int("page-size", 50, "Results per page")

	milestonesCmd.AddCommand(milestonesListCmd, milestonesShowCmd)
	rootCmd.AddCommand(milestonesCmd)
}

func runMilestonesList(cmd *cobra.Command, args []string) {
	client := getClient()
	mode := getOutputMode()
	params := url.Values{}
	params.Set("include", "projects,users")

	if v, _ := cmd.Flags().GetString("project"); v != "" {
		pid, err := getResolver().Project(v)
		if err != nil {
			exitOnError(err)
		}
		params.Set("projectIds", fmt.Sprintf("%d", pid))
	}
	if v, _ := cmd.Flags().GetBool("completed"); v {
		params.Set("includeCompleted", "true")
	}
	page, _ := cmd.Flags().GetInt("page")
	pageSize, _ := cmd.Flags().GetInt("page-size")
	params.Set("page", fmt.Sprintf("%d", page))
	params.Set("pageSize", fmt.Sprintf("%d", pageSize))

	data, err := client.Get("/projects/api/v3/milestones.json", params)
	if err != nil {
		exitOnError(err)
	}

	if mode == format.JSON {
		format.PrintJSON(data)
		return
	}

	var resp struct {
		Milestones []struct {
			ID        int    `json:"id"`
			Name      string `json:"name"`
			Deadline  string `json:"deadline"`
			Completed bool   `json:"completed"`
			ProjectID int    `json:"projectId"`
			Status    string `json:"status"`
		} `json:"milestones"`
		Meta struct {
			Page struct{ Count int `json:"count"` } `json:"page"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		fmt.Fprintln(os.Stderr, "Error parsing response:", err)
		exitFn(1)
	}
	included := api.ParseIncluded(data)

	headers := []string{"ID", "NAME", "PROJECT", "DEADLINE", "STATUS"}
	rows := make([][]string, len(resp.Milestones))
	for i, m := range resp.Milestones {
		project := included.LookupString("projects", fmt.Sprintf("%d", m.ProjectID), "name")
		rows[i] = []string{
			fmt.Sprintf("%d", m.ID),
			format.Truncate(m.Name, 40),
			format.Truncate(project, 25),
			formatDate(m.Deadline),
			m.Status,
		}
	}

	if mode == format.CSV {
		format.PrintCSV(headers, rows)
	} else {
		format.PrintTable(os.Stdout, headers, rows)
		fmt.Printf("\nPage %d · %d of %d milestone(s)\n", page, len(resp.Milestones), resp.Meta.Page.Count)
	}
}

func runMilestonesShow(cmd *cobra.Command, args []string) {
	client := getClient()
	mode := getOutputMode()

	data, err := client.Get("/projects/api/v3/milestones/"+args[0]+".json?include=projects", nil)
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
	m, _ := wrap["milestone"].(map[string]interface{})
	if m == nil {
		format.PrintJSON(data)
		return
	}

	for _, f := range []struct{ label, key string }{
		{"ID", "id"}, {"Name", "name"}, {"Description", "description"},
		{"Deadline", "deadline"}, {"Completed", "completed"},
		{"Status", "status"}, {"Created", "createdOn"},
	} {
		if v, ok := m[f.key]; ok && v != nil {
			val := fmt.Sprintf("%v", v)
			if val == "" {
				continue
			}
			if strings.HasSuffix(f.key, "line") || strings.HasSuffix(f.key, "On") || strings.HasSuffix(f.key, "At") {
				val = formatDate(val)
			}
			fmt.Printf("%-13s %s\n", f.label+":", val)
		}
	}
}
