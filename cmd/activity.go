package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/equisolve/teamwork-cli/internal/format"
	"github.com/spf13/cobra"
)

var activityCmd = &cobra.Command{
	Use:   "activity",
	Short: "Show latest activity across projects",
	Run:   runActivity,
}

func init() {
	activityCmd.Flags().StringP("project", "p", "", "Scope to a single project (ID or name)")
	activityCmd.Flags().Int("max", 50, "Maximum records to return")
	rootCmd.AddCommand(activityCmd)
}

func runActivity(cmd *cobra.Command, args []string) {
	client := getClient()
	mode := getOutputMode()

	path := "/latestActivity.json"
	if projectQ, _ := cmd.Flags().GetString("project"); projectQ != "" {
		pid, err := getResolver().Project(projectQ)
		if err != nil {
			exitOnError(err)
		}
		path = fmt.Sprintf("/projects/%d/latestActivity.json", pid)
	}

	params := url.Values{}
	m, _ := cmd.Flags().GetInt("max")
	params.Set("maxRecords", fmt.Sprintf("%d", m))

	data, err := client.Get(path, params)
	if err != nil {
		exitOnError(err)
	}
	if mode == format.JSON {
		format.PrintJSON(data)
		return
	}

	var resp struct {
		Activity []struct {
			ID          json.Number `json:"id"`
			Type        string      `json:"activity-type"`
			Action      string      `json:"action"`
			DateTime    string      `json:"datetime"`
			ProjectName string      `json:"project-name"`
			CompanyName string      `json:"company-name"`
			ForUser     string      `json:"for-user-name"`
			Description string      `json:"description"`
		} `json:"activity"`
	}
	_ = json.Unmarshal(data, &resp)

	headers := []string{"WHEN", "PROJECT", "USER", "ACTION", "TYPE", "DESC"}
	rows := make([][]string, len(resp.Activity))
	for i, a := range resp.Activity {
		rows[i] = []string{
			formatDate(a.DateTime),
			format.Truncate(a.ProjectName, 20),
			format.Truncate(a.ForUser, 18),
			a.Action,
			format.Truncate(a.Type, 15),
			format.Truncate(a.Description, 40),
		}
	}
	if mode == format.CSV {
		format.PrintCSV(headers, rows)
	} else {
		format.PrintTable(os.Stdout, headers, rows)
		fmt.Printf("\n%d activity record(s)\n", len(resp.Activity))
	}
}
