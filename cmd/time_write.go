package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

var timeUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update fields on a time entry",
	Args:  cobra.ExactArgs(1),
	Run:   runTimeUpdate,
}

var timeDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a time entry",
	Args:  cobra.ExactArgs(1),
	Run:   runTimeDelete,
}

func init() {
	timeUpdateCmd.Flags().Float64("hours", -1, "New hours (decimal allowed)")
	timeUpdateCmd.Flags().Int("minutes", -1, "New minutes (combined with --hours)")
	timeUpdateCmd.Flags().String("description", "", "New description")
	timeUpdateCmd.Flags().String("date", "", "New date YYYY-MM-DD")
	timeUpdateCmd.Flags().String("start", "", "New start time HH:MM")
	timeUpdateCmd.Flags().String("billable", "", "Billable: yes or no")

	timeDeleteCmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")

	timeCmd.AddCommand(timeUpdateCmd, timeDeleteCmd)
}

func runTimeUpdate(cmd *cobra.Command, args []string) {
	client := getClient()

	entry := map[string]interface{}{}

	h, _ := cmd.Flags().GetFloat64("hours")
	m, _ := cmd.Flags().GetInt("minutes")
	if h >= 0 || m >= 0 {
		totalMin := 0
		if h > 0 {
			totalMin += int(h * 60)
		}
		if m > 0 {
			totalMin += m
		}
		entry["hours"] = strconv.Itoa(totalMin / 60)
		entry["minutes"] = strconv.Itoa(totalMin % 60)
	}
	if v, _ := cmd.Flags().GetString("description"); v != "" {
		entry["description"] = v
	}
	if v, _ := cmd.Flags().GetString("date"); v != "" {
		entry["date"] = compactDate(v)
	}
	if v, _ := cmd.Flags().GetString("start"); v != "" {
		entry["time"] = v
	}
	if v, _ := cmd.Flags().GetString("billable"); v != "" {
		if v == "yes" || v == "true" || v == "1" {
			entry["isbillable"] = "1"
		} else {
			entry["isbillable"] = "0"
		}
	}
	if len(entry) == 0 {
		fmt.Fprintln(os.Stderr, "Error: no updates specified")
		exitFn(1)
	}

	payload := map[string]interface{}{"time-entry": entry}
	if _, err := client.Put("/time_entries/"+args[0]+".json", nil, payload); err != nil {
		exitOnError(err)
	}
	fmt.Printf("Time entry %s updated.\n", args[0])
}

func runTimeDelete(cmd *cobra.Command, args []string) {
	client := getClient()
	skip, _ := cmd.Flags().GetBool("yes")
	if !skip && !confirm(fmt.Sprintf("Delete time entry %s?", args[0])) {
		fmt.Println("Aborted.")
		return
	}
	if _, err := client.Delete("/time_entries/"+args[0]+".json", nil); err != nil {
		exitOnError(err)
	}
	fmt.Printf("Time entry %s deleted.\n", args[0])
}
