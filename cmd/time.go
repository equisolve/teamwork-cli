package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/equisolve/teamwork-cli/internal/format"
	"github.com/spf13/cobra"
)

var timeCmd = &cobra.Command{
	Use:   "time",
	Short: "List and log time entries",
}

var timeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List time entries",
	Run:   runTimeList,
}

var timeLogCmd = &cobra.Command{
	Use:   "log",
	Short: "Log time against a task or project",
	Run:   runTimeLog,
}

func init() {
	timeListCmd.Flags().StringP("project", "p", "", "Filter by project ID")
	timeListCmd.Flags().String("user", "", "Filter by user ID (or 'me')")
	timeListCmd.Flags().String("from", "", "From date YYYY-MM-DD (or YYYYMMDD)")
	timeListCmd.Flags().String("to", "", "To date YYYY-MM-DD (or YYYYMMDD)")
	timeListCmd.Flags().Bool("billable", false, "Only billable entries")
	timeListCmd.Flags().Bool("invoiced", false, "Only invoiced entries")
	timeListCmd.Flags().Int("page", 1, "Page number")
	timeListCmd.Flags().Int("page-size", 50, "Results per page")

	timeLogCmd.Flags().String("task", "", "Task ID to log against (preferred)")
	timeLogCmd.Flags().String("project", "", "Project ID to log against (if no task)")
	timeLogCmd.Flags().String("date", "", "Date YYYY-MM-DD (default: today)")
	timeLogCmd.Flags().String("start", "", "Start time HH:MM (default: 09:00)")
	timeLogCmd.Flags().Float64("hours", 0, "Hours worked (decimal allowed, e.g. 1.5)")
	timeLogCmd.Flags().Int("minutes", 0, "Additional minutes (combined with --hours)")
	timeLogCmd.Flags().String("description", "", "Description of the work")
	timeLogCmd.Flags().Bool("billable", true, "Is this time billable")
	timeLogCmd.Flags().String("user", "", "Log on behalf of user ID (default: yourself)")

	timeCmd.AddCommand(timeListCmd, timeLogCmd)
	rootCmd.AddCommand(timeCmd)
}

func runTimeList(cmd *cobra.Command, args []string) {
	client := getClient()
	mode := getOutputMode()
	params := url.Values{}

	projectID, _ := cmd.Flags().GetString("project")
	if v, _ := cmd.Flags().GetString("user"); v != "" {
		params.Set("userId", v)
	}
	if v, _ := cmd.Flags().GetString("from"); v != "" {
		params.Set("fromdate", compactDate(v))
	}
	if v, _ := cmd.Flags().GetString("to"); v != "" {
		params.Set("todate", compactDate(v))
	}
	if v, _ := cmd.Flags().GetBool("billable"); v {
		params.Set("billableType", "billable")
	}
	if v, _ := cmd.Flags().GetBool("invoiced"); v {
		params.Set("invoicedType", "invoiced")
	}
	page, _ := cmd.Flags().GetInt("page")
	pageSize, _ := cmd.Flags().GetInt("page-size")
	params.Set("page", fmt.Sprintf("%d", page))
	params.Set("pageSize", fmt.Sprintf("%d", pageSize))

	path := "/time_entries.json"
	if projectID != "" {
		path = "/projects/" + projectID + "/time_entries.json"
	}

	data, err := client.Get(path, params)
	if err != nil {
		exitOnError(err)
	}

	if mode == format.JSON {
		format.PrintJSON(data)
		return
	}

	var resp struct {
		TimeEntries []struct {
			ID          json.Number `json:"id"`
			Date        string      `json:"date"`
			Description string      `json:"description"`
			PersonFirst string      `json:"person-first-name"`
			PersonLast  string      `json:"person-last-name"`
			ProjectName string      `json:"project-name"`
			TodoName    string      `json:"todo-item-name"`
			Hours       json.Number `json:"hours"`
			Minutes     json.Number `json:"minutes"`
			Billable    string      `json:"isbillable"`
			Invoiced    string      `json:"invoiceNo"`
		} `json:"time-entries"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		fmt.Fprintln(os.Stderr, "Error parsing response:", err)
		exitFn(1)
	}

	headers := []string{"ID", "DATE", "PERSON", "PROJECT", "TASK", "DURATION", "BILLABLE", "DESC"}
	rows := make([][]string, len(resp.TimeEntries))
	for i, e := range resp.TimeEntries {
		h, _ := e.Hours.Int64()
		m, _ := e.Minutes.Int64()
		dur := fmt.Sprintf("%dh %02dm", h, m)
		billable := ""
		if e.Billable == "1" {
			billable = "Y"
		}
		rows[i] = []string{
			e.ID.String(),
			formatDate(e.Date),
			format.Truncate(strings.TrimSpace(e.PersonFirst+" "+e.PersonLast), 20),
			format.Truncate(e.ProjectName, 20),
			format.Truncate(e.TodoName, 20),
			dur,
			billable,
			format.Truncate(e.Description, 40),
		}
	}

	if mode == format.CSV {
		format.PrintCSV(headers, rows)
	} else {
		format.PrintTable(os.Stdout, headers, rows)
		fmt.Printf("\nPage %d · %d entry/entries\n", page, len(resp.TimeEntries))
	}
}

func runTimeLog(cmd *cobra.Command, args []string) {
	client := getClient()

	taskID, _ := cmd.Flags().GetString("task")
	projectID, _ := cmd.Flags().GetString("project")
	if taskID == "" && projectID == "" {
		fmt.Fprintln(os.Stderr, "Error: must provide --task or --project")
		exitFn(1)
	}

	hours, _ := cmd.Flags().GetFloat64("hours")
	minutes, _ := cmd.Flags().GetInt("minutes")
	if hours == 0 && minutes == 0 {
		fmt.Fprintln(os.Stderr, "Error: must provide --hours and/or --minutes")
		exitFn(1)
	}
	totalMin := int(hours*60) + minutes
	h := totalMin / 60
	m := totalMin % 60

	date, _ := cmd.Flags().GetString("date")
	if date == "" {
		date = time.Now().Format("20060102")
	} else {
		date = compactDate(date)
	}
	start, _ := cmd.Flags().GetString("start")
	if start == "" {
		start = "09:00"
	}
	desc, _ := cmd.Flags().GetString("description")
	billable, _ := cmd.Flags().GetBool("billable")
	userID, _ := cmd.Flags().GetString("user")

	entry := map[string]interface{}{
		"date":        date,
		"time":        start,
		"hours":       strconv.Itoa(h),
		"minutes":     strconv.Itoa(m),
		"description": desc,
		"isbillable":  boolToStr(billable),
	}
	if userID != "" {
		entry["person-id"] = userID
	}
	payload := map[string]interface{}{"time-entry": entry}

	var path string
	if taskID != "" {
		path = "/tasks/" + taskID + "/time_entries.json"
	} else {
		path = "/projects/" + projectID + "/time_entries.json"
	}

	data, err := client.Post(path, nil, payload)
	if err != nil {
		exitOnError(err)
	}

	var resp struct {
		TimeEntryID string `json:"timeLogId"`
		ID          string `json:"id"`
	}
	_ = json.Unmarshal(data, &resp)
	id := resp.TimeEntryID
	if id == "" {
		id = resp.ID
	}
	// `date` is YYYYMMDD for the API; render ISO for humans.
	displayDate := date
	if len(date) == 8 {
		displayDate = date[:4] + "-" + date[4:6] + "-" + date[6:]
	}
	fmt.Printf("Logged %dh %02dm on %s", h, m, displayDate)
	if id != "" {
		fmt.Printf(" (entry %s)", id)
	}
	fmt.Println()
}

// compactDate converts YYYY-MM-DD to YYYYMMDD (Teamwork v1 wants compact dates).
func compactDate(s string) string {
	return strings.ReplaceAll(s, "-", "")
}

func boolToStr(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
