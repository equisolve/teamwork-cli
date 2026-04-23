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

	var resp struct {
		Workload struct {
			UserCapacities []struct {
				UserID     json.Number `json:"user-id"`
				Name       string      `json:"user-full-name"`
				Capacity   json.Number `json:"capacity"`
				Available  json.Number `json:"available-minutes"`
				LoggedTime json.Number `json:"logged-time"`
				Estimated  json.Number `json:"estimated-time"`
			} `json:"userCapacities"`
		} `json:"workload"`
	}
	_ = json.Unmarshal(data, &resp)

	headers := []string{"USER", "CAPACITY %", "AVAILABLE (min)", "ESTIMATED (min)", "LOGGED (min)"}
	rows := make([][]string, len(resp.Workload.UserCapacities))
	for i, u := range resp.Workload.UserCapacities {
		rows[i] = []string{
			format.Truncate(u.Name, 25),
			u.Capacity.String(),
			u.Available.String(),
			u.Estimated.String(),
			u.LoggedTime.String(),
		}
	}
	if mode == format.CSV {
		format.PrintCSV(headers, rows)
	} else {
		format.PrintTable(os.Stdout, headers, rows)
		fmt.Printf("\n%s → %s · %d user(s)\n", from, to, len(rows))
	}
}
