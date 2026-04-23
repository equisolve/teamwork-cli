package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/equisolve/teamwork-cli/internal/format"
	"github.com/spf13/cobra"
)

var tagsCmd = &cobra.Command{
	Use:   "tags",
	Short: "List Teamwork tags (all or per-project)",
}

var tagsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tags",
	Run:   runTagsList,
}

func init() {
	tagsListCmd.Flags().StringP("project", "p", "", "Filter tags to a specific project (ID or name)")
	tagsListCmd.Flags().String("search", "", "Search tag names")
	tagsCmd.AddCommand(tagsListCmd)
	rootCmd.AddCommand(tagsCmd)
}

func runTagsList(cmd *cobra.Command, args []string) {
	client := getClient()
	mode := getOutputMode()
	params := url.Values{}
	if v, _ := cmd.Flags().GetString("search"); v != "" {
		params.Set("searchTerm", v)
	}
	if v, _ := cmd.Flags().GetString("project"); v != "" {
		pid, err := getResolver().Project(v)
		if err != nil {
			exitOnError(err)
		}
		params.Set("projectId", fmt.Sprintf("%d", pid))
	}

	data, err := client.Get("/tags.json", params)
	if err != nil {
		exitOnError(err)
	}
	if mode == format.JSON {
		format.PrintJSON(data)
		return
	}
	var resp struct {
		Tags []struct {
			ID        json.Number `json:"id"`
			Name      string      `json:"name"`
			Color     string      `json:"color"`
			ProjectID json.Number `json:"projectId"`
			Created   string      `json:"dateCreated"`
		} `json:"tags"`
	}
	_ = json.Unmarshal(data, &resp)

	headers := []string{"ID", "NAME", "COLOR", "PROJECT", "CREATED"}
	rows := make([][]string, len(resp.Tags))
	for i, t := range resp.Tags {
		scope := "all"
		if t.ProjectID.String() != "" && t.ProjectID.String() != "0" {
			scope = t.ProjectID.String()
		}
		rows[i] = []string{
			t.ID.String(),
			format.Truncate(t.Name, 30),
			t.Color,
			scope,
			formatDate(t.Created),
		}
	}
	if mode == format.CSV {
		format.PrintCSV(headers, rows)
	} else {
		format.PrintTable(os.Stdout, headers, rows)
		fmt.Printf("\n%d tag(s)\n", len(resp.Tags))
	}
}
