package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/equisolve/teamwork-cli/internal/api"
	"github.com/equisolve/teamwork-cli/internal/format"
	"github.com/spf13/cobra"
)

var notebooksCmd = &cobra.Command{
	Use:     "notebooks",
	Aliases: []string{"notebook"},
	Short:   "List and view project notebooks",
}

var notebooksListCmd = &cobra.Command{
	Use:   "list",
	Short: "List notebooks",
	Run:   runNotebooksList,
}

var notebooksShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show notebook details",
	Args:  cobra.ExactArgs(1),
	Run:   runNotebooksShow,
}

func init() {
	notebooksListCmd.Flags().StringP("project", "p", "", "Filter by project ID or name")
	notebooksListCmd.Flags().Int("page", 1, "Page number")
	notebooksListCmd.Flags().Int("page-size", 25, "Results per page")

	notebooksCmd.AddCommand(notebooksListCmd, notebooksShowCmd)
	rootCmd.AddCommand(notebooksCmd)
}

func runNotebooksList(cmd *cobra.Command, args []string) {
	client := getClient()
	mode := getOutputMode()
	params := url.Values{}
	params.Set("include", "projects")
	if v, _ := cmd.Flags().GetString("project"); v != "" {
		pid, err := getResolver().Project(v)
		if err != nil {
			exitOnError(err)
		}
		params.Set("projectIds", fmt.Sprintf("%d", pid))
	}
	page, _ := cmd.Flags().GetInt("page")
	pageSize, _ := cmd.Flags().GetInt("page-size")
	params.Set("page", fmt.Sprintf("%d", page))
	params.Set("pageSize", fmt.Sprintf("%d", pageSize))

	data, err := client.Get("/projects/api/v3/notebooks.json", params)
	if err != nil {
		exitOnError(err)
	}
	if mode == format.JSON {
		format.PrintJSON(data)
		return
	}
	var resp struct {
		Notebooks []struct {
			ID        int    `json:"id"`
			Name      string `json:"name"`
			ProjectID int    `json:"projectId"`
			Locked    bool   `json:"locked"`
			Type      string `json:"type"`
		} `json:"notebooks"`
		Meta struct{ Page struct{ Count int `json:"count"` } `json:"page"` } `json:"meta"`
	}
	_ = json.Unmarshal(data, &resp)
	included := api.ParseIncluded(data)

	headers := []string{"ID", "NAME", "PROJECT", "TYPE", "LOCKED"}
	rows := make([][]string, len(resp.Notebooks))
	for i, n := range resp.Notebooks {
		project := included.LookupString("projects", fmt.Sprintf("%d", n.ProjectID), "name")
		locked := ""
		if n.Locked {
			locked = "Y"
		}
		rows[i] = []string{
			fmt.Sprintf("%d", n.ID),
			format.Truncate(n.Name, 40),
			format.Truncate(project, 25),
			n.Type,
			locked,
		}
	}
	if mode == format.CSV {
		format.PrintCSV(headers, rows)
	} else {
		format.PrintTable(os.Stdout, headers, rows)
		fmt.Printf("\nPage %d · %d of %d notebook(s)\n", page, len(resp.Notebooks), resp.Meta.Page.Count)
	}
}

func runNotebooksShow(cmd *cobra.Command, args []string) {
	client := getClient()
	mode := getOutputMode()
	data, err := client.Get("/projects/api/v3/notebooks/"+args[0]+".json", nil)
	if err != nil {
		exitOnError(err)
	}
	if mode == format.JSON {
		format.PrintJSON(data)
		return
	}
	wrap, _ := decodeMap(data)
	n, _ := wrap["notebook"].(map[string]interface{})
	if n == nil {
		format.PrintJSON(data)
		return
	}
	for _, f := range []struct{ label, key string }{
		{"ID", "id"}, {"Name", "name"}, {"Type", "type"},
		{"Description", "description"}, {"Locked", "locked"},
	} {
		if v, ok := n[f.key]; ok && v != nil {
			val := fmt.Sprintf("%v", v)
			if val == "" {
				continue
			}
			fmt.Printf("%-13s %s\n", f.label+":", val)
		}
	}
	if contents, ok := n["contents"].(string); ok && contents != "" {
		fmt.Println("\n" + contents)
	}
}
