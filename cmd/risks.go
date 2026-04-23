package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/equisolve/teamwork-cli/internal/format"
	"github.com/spf13/cobra"
)

var risksCmd = &cobra.Command{
	Use:     "risks",
	Aliases: []string{"risk"},
	Short:   "List and view risks",
}

var risksListCmd = &cobra.Command{
	Use:   "list",
	Short: "List risks",
	Run:   runRisksList,
}

var risksShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show risk details",
	Args:  cobra.ExactArgs(1),
	Run:   runRisksShow,
}

func init() {
	risksListCmd.Flags().StringP("project", "p", "", "Filter by project ID or name")
	risksCmd.AddCommand(risksListCmd, risksShowCmd)
	rootCmd.AddCommand(risksCmd)
}

func runRisksList(cmd *cobra.Command, args []string) {
	client := getClient()
	mode := getOutputMode()

	path := "/risks.json"
	params := url.Values{}
	if v, _ := cmd.Flags().GetString("project"); v != "" {
		pid, err := getResolver().Project(v)
		if err != nil {
			exitOnError(err)
		}
		path = fmt.Sprintf("/projects/%d/risks.json", pid)
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
		Risks []struct {
			ID          json.Number `json:"id"`
			Title       string      `json:"title"`
			ProjectName string      `json:"project-name"`
			Impact      string      `json:"impact"`
			Probability string      `json:"probability"`
			Status      string      `json:"status"`
		} `json:"risks"`
	}
	_ = json.Unmarshal(data, &resp)

	headers := []string{"ID", "TITLE", "PROJECT", "IMPACT", "PROBABILITY", "STATUS"}
	rows := make([][]string, len(resp.Risks))
	for i, r := range resp.Risks {
		rows[i] = []string{
			r.ID.String(),
			format.Truncate(r.Title, 35),
			format.Truncate(r.ProjectName, 20),
			r.Impact, r.Probability, r.Status,
		}
	}
	if mode == format.CSV {
		format.PrintCSV(headers, rows)
	} else {
		format.PrintTable(os.Stdout, headers, rows)
		fmt.Printf("\n%d risk(s)\n", len(resp.Risks))
	}
}

func runRisksShow(cmd *cobra.Command, args []string) {
	client := getClient()
	mode := getOutputMode()
	data, err := client.Get("/risks/"+args[0]+".json", nil)
	if err != nil {
		exitOnError(err)
	}
	if mode == format.JSON {
		format.PrintJSON(data)
		return
	}
	wrap, _ := decodeMap(data)
	r, _ := wrap["risk"].(map[string]interface{})
	if r == nil {
		format.PrintJSON(data)
		return
	}
	for _, f := range []struct{ label, key string }{
		{"ID", "id"}, {"Title", "title"}, {"Description", "description"},
		{"Impact", "impact"}, {"Probability", "probability"}, {"Status", "status"},
	} {
		if v, ok := r[f.key]; ok && v != nil {
			val := fmt.Sprintf("%v", v)
			if val == "" {
				continue
			}
			fmt.Printf("%-13s %s\n", f.label+":", val)
		}
	}
}
