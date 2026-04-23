package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/equisolve/teamwork-cli/internal/api"
	"github.com/equisolve/teamwork-cli/internal/format"
	"github.com/spf13/cobra"
)

var timerCmd = &cobra.Command{
	Use:   "timer",
	Short: "Start, stop, and manage running timers",
}

var timerListCmd = &cobra.Command{
	Use:   "list",
	Short: "List your running/paused timers",
	Run:   runTimerList,
}

var timerStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a timer (stops any other running timer)",
	Run:   runTimerStart,
}

var timerStopCmd = &cobra.Command{
	Use:   "stop <timer-id>",
	Short: "Stop a running timer and convert it to a time entry",
	Args:  cobra.ExactArgs(1),
	Run:   runTimerStop,
}

var timerPauseCmd = &cobra.Command{
	Use:   "pause <timer-id>",
	Short: "Pause a timer",
	Args:  cobra.ExactArgs(1),
	Run:   runTimerPause,
}

var timerResumeCmd = &cobra.Command{
	Use:   "resume <timer-id>",
	Short: "Resume a paused timer",
	Args:  cobra.ExactArgs(1),
	Run:   runTimerResume,
}

var timerDeleteCmd = &cobra.Command{
	Use:   "delete <timer-id>",
	Short: "Delete a timer without logging time",
	Args:  cobra.ExactArgs(1),
	Run:   runTimerDelete,
}

func init() {
	timerStartCmd.Flags().String("task", "", "Task ID or name to time against")
	timerStartCmd.Flags().String("project", "", "Project ID or name (used when --task isn't set)")
	timerStartCmd.Flags().String("description", "", "Description of the work")
	timerStartCmd.Flags().Bool("billable", true, "Mark the resulting time entry as billable")

	timerCmd.AddCommand(timerListCmd, timerStartCmd, timerStopCmd, timerPauseCmd, timerResumeCmd, timerDeleteCmd)
	rootCmd.AddCommand(timerCmd)
}

func runTimerList(cmd *cobra.Command, args []string) {
	client := getClient()
	mode := getOutputMode()

	data, err := client.Get("/projects/api/v3/me/timers.json?include=tasks,projects", nil)
	if err != nil {
		exitOnError(err)
	}

	if mode == format.JSON {
		format.PrintJSON(data)
		return
	}

	var resp struct {
		Timers []struct {
			ID          int    `json:"id"`
			TaskID      *int   `json:"taskId"`
			ProjectID   int    `json:"projectId"`
			Description string `json:"description"`
			Running     bool   `json:"running"`
			Billable    bool   `json:"billable"`
			Duration    int    `json:"duration"`
			LastStarted string `json:"lastStartedAt"`
		} `json:"timers"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		fmt.Fprintln(os.Stderr, "Error parsing response:", err)
		exitFn(1)
	}
	included := api.ParseIncluded(data)

	headers := []string{"ID", "TASK", "PROJECT", "DURATION", "STATE", "BILLABLE", "DESC"}
	rows := make([][]string, len(resp.Timers))
	for i, t := range resp.Timers {
		taskName := ""
		if t.TaskID != nil {
			taskName = included.LookupString("tasks", fmt.Sprintf("%d", *t.TaskID), "name")
		}
		project := included.LookupString("projects", fmt.Sprintf("%d", t.ProjectID), "name")
		state := "paused"
		if t.Running {
			state = "running"
		}
		billable := ""
		if t.Billable {
			billable = "Y"
		}
		rows[i] = []string{
			fmt.Sprintf("%d", t.ID),
			format.Truncate(taskName, 30),
			format.Truncate(project, 20),
			format.DurationMinutes(t.Duration / 60),
			state,
			billable,
			format.Truncate(t.Description, 30),
		}
	}

	if mode == format.CSV {
		format.PrintCSV(headers, rows)
	} else {
		format.PrintTable(os.Stdout, headers, rows)
		fmt.Printf("\n%d timer(s)\n", len(resp.Timers))
	}
}

func runTimerStart(cmd *cobra.Command, args []string) {
	client := getClient()

	taskQuery, _ := cmd.Flags().GetString("task")
	projectQuery, _ := cmd.Flags().GetString("project")
	desc, _ := cmd.Flags().GetString("description")
	billable, _ := cmd.Flags().GetBool("billable")

	timer := map[string]interface{}{
		"description": desc,
		"billable":    billable,
	}

	if taskQuery != "" {
		taskID, err := resolveTaskID(taskQuery)
		if err != nil {
			exitOnError(err)
		}
		timer["taskId"] = taskID
	} else if projectQuery != "" {
		pid, err := getResolver().Project(projectQuery)
		if err != nil {
			exitOnError(err)
		}
		timer["projectId"] = pid
	} else {
		fmt.Fprintln(os.Stderr, "Error: must provide --task or --project")
		exitFn(1)
	}

	payload := map[string]interface{}{"timer": timer}
	data, err := client.Post("/projects/api/v3/me/timers.json", nil, payload)
	if err != nil {
		exitOnError(err)
	}

	var resp struct {
		Timer struct {
			ID      int  `json:"id"`
			Running bool `json:"running"`
		} `json:"timer"`
	}
	_ = json.Unmarshal(data, &resp)
	fmt.Printf("Timer %d started", resp.Timer.ID)
	if desc != "" {
		fmt.Printf(" — %q", desc)
	}
	fmt.Printf(" at %s\n", time.Now().Format("15:04"))
}

func runTimerStop(cmd *cobra.Command, args []string) {
	client := getClient()
	if _, err := client.Put("/projects/api/v3/me/timers/"+args[0]+"/stop.json", nil, nil); err != nil {
		exitOnError(err)
	}
	fmt.Printf("Timer %s stopped and logged.\n", args[0])
}

func runTimerPause(cmd *cobra.Command, args []string) {
	client := getClient()
	if _, err := client.Put("/projects/api/v3/me/timers/"+args[0]+"/pause.json", nil, nil); err != nil {
		exitOnError(err)
	}
	fmt.Printf("Timer %s paused.\n", args[0])
}

func runTimerResume(cmd *cobra.Command, args []string) {
	client := getClient()
	if _, err := client.Put("/projects/api/v3/me/timers/"+args[0]+"/resume.json", nil, nil); err != nil {
		exitOnError(err)
	}
	fmt.Printf("Timer %s resumed.\n", args[0])
}

func runTimerDelete(cmd *cobra.Command, args []string) {
	client := getClient()
	if _, err := client.Delete("/projects/api/v3/me/timers/"+args[0]+".json", nil); err != nil {
		exitOnError(err)
	}
	fmt.Printf("Timer %s deleted (no time logged).\n", args[0])
}

// resolveTaskID accepts a numeric ID or falls back to a v3 task search. Task
// search is less predictable than project search, so we only support numeric
// for now and surface a helpful error for names.
func resolveTaskID(query string) (int, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return 0, fmt.Errorf("empty task query")
	}
	var id int
	if _, err := fmt.Sscanf(q, "%d", &id); err == nil && id > 0 {
		return id, nil
	}
	// Very small v3 search by name.
	params := url.Values{}
	params.Set("searchTerm", q)
	params.Set("pageSize", "5")
	data, err := getClient().Get("/projects/api/v3/tasks.json", params)
	if err != nil {
		return 0, err
	}
	var resp struct {
		Tasks []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, err
	}
	switch len(resp.Tasks) {
	case 0:
		return 0, fmt.Errorf("no task matches %q", query)
	case 1:
		return resp.Tasks[0].ID, nil
	default:
		names := make([]string, 0, len(resp.Tasks))
		for _, t := range resp.Tasks {
			names = append(names, fmt.Sprintf("%d: %s", t.ID, t.Name))
		}
		return 0, fmt.Errorf("ambiguous task query %q — %d matches:\n  %s",
			query, len(resp.Tasks), strings.Join(names, "\n  "))
	}
}
