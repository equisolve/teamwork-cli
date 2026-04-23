package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/equisolve/teamwork-cli/internal/format"
	"github.com/spf13/cobra"
)

var workloadCmd = &cobra.Command{
	Use:   "workload",
	Short: "Show capacity/workload per user over a date range",
	Run:   runWorkload,
}

func init() {
	workloadCmd.Flags().String("from", "", "Start date YYYY-MM-DD (default: today)")
	workloadCmd.Flags().String("to", "", "End date YYYY-MM-DD (default: today + 7d)")
	rootCmd.AddCommand(workloadCmd)
}

func runWorkload(cmd *cobra.Command, args []string) {
	client := getClient()
	mode := getOutputMode()

	from, _ := cmd.Flags().GetString("from")
	to, _ := cmd.Flags().GetString("to")
	if from == "" {
		from = time.Now().Format("2006-01-02")
	}
	if to == "" {
		to = time.Now().AddDate(0, 0, 7).Format("2006-01-02")
	}

	params := url.Values{}
	params.Set("startDate", strings.ReplaceAll(from, "-", ""))
	params.Set("endDate", strings.ReplaceAll(to, "-", ""))

	data, err := client.Get("/workload.json", params)
	if err != nil {
		exitOnError(err)
	}
	if mode == format.JSON {
		format.PrintJSON(data)
		return
	}

	// v1 /workload.json returns {"STATUS":"OK","workload":[ ...project+user rows ]}.
	// Each row is one user's workload on one project, so aggregate per user.
	var resp struct {
		Workload []struct {
			UserID             json.Number `json:"userId"`
			UserFirstName      string      `json:"userFirstName"`
			UserLastName       string      `json:"userLastName"`
			ProjectID          string      `json:"projectId"`
			TotalEstimatedTime json.Number `json:"totalEstimatedTime"`
			TotalLoggedTime    json.Number `json:"totalLoggedTime"`
			ActiveTasks        json.Number `json:"numberOfActiveTasks"`
			CompletedTasks     json.Number `json:"numberOfCompletedTasks"`
		} `json:"workload"`
	}
	_ = json.Unmarshal(data, &resp)

	type agg struct {
		name                 string
		estMin, loggedMin    int
		active, completed    int
		projects             map[string]struct{}
	}
	byUser := map[string]*agg{}
	var order []string
	atoi := func(n json.Number) int { v, _ := n.Int64(); return int(v) }
	for _, r := range resp.Workload {
		uid := r.UserID.String()
		a, ok := byUser[uid]
		if !ok {
			a = &agg{
				name:     strings.TrimSpace(r.UserFirstName + " " + r.UserLastName),
				projects: map[string]struct{}{},
			}
			byUser[uid] = a
			order = append(order, uid)
		}
		a.estMin += atoi(r.TotalEstimatedTime)
		a.loggedMin += atoi(r.TotalLoggedTime)
		a.active += atoi(r.ActiveTasks)
		a.completed += atoi(r.CompletedTasks)
		if r.ProjectID != "" {
			a.projects[r.ProjectID] = struct{}{}
		}
	}

	headers := []string{"USER", "PROJECTS", "ACTIVE", "COMPLETED", "EST (min)", "LOGGED (min)"}
	rows := make([][]string, 0, len(order))
	for _, uid := range order {
		a := byUser[uid]
		rows = append(rows, []string{
			format.Truncate(a.name, 25),
			fmt.Sprintf("%d", len(a.projects)),
			fmt.Sprintf("%d", a.active),
			fmt.Sprintf("%d", a.completed),
			fmt.Sprintf("%d", a.estMin),
			fmt.Sprintf("%d", a.loggedMin),
		})
	}
	if mode == format.CSV {
		format.PrintCSV(headers, rows)
	} else {
		format.PrintTable(os.Stdout, headers, rows)
		fmt.Printf("\n%s → %s · %d user(s)\n", from, to, len(rows))
	}
}
