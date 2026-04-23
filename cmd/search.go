package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/equisolve/teamwork-cli/internal/format"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search across tasks, messages, milestones, notebooks, files, people, etc.",
	Args:  cobra.ExactArgs(1),
	Run:   runSearch,
}

func init() {
	searchCmd.Flags().String("type", "tasks", "Type: tasks, messages, milestones, notebooks, files, links, people, companies, projects, events")
	rootCmd.AddCommand(searchCmd)
}

func runSearch(cmd *cobra.Command, args []string) {
	client := getClient()
	mode := getOutputMode()
	kind, _ := cmd.Flags().GetString("type")

	params := url.Values{}
	params.Set("searchFor", kind)
	params.Set("searchTerm", args[0])

	data, err := client.Get("/search.json", params)
	if err != nil {
		exitOnError(err)
	}

	if mode == format.JSON {
		format.PrintJSON(data)
		return
	}

	// searchResult: { tasks: [...], messages: [...], ... }
	var resp struct {
		SearchResult map[string]json.RawMessage `json:"searchResult"`
	}
	_ = json.Unmarshal(data, &resp)
	bucket, ok := resp.SearchResult[kind]
	if !ok {
		fmt.Printf("No results.\n")
		return
	}

	var items []map[string]interface{}
	if err := json.Unmarshal(bucket, &items); err != nil {
		format.PrintJSON(data)
		return
	}

	headers := []string{"ID", "NAME", "PROJECT"}
	rows := make([][]string, len(items))
	for i, it := range items {
		id := fmt.Sprintf("%v", firstKey(it, "id", "searchResultId"))
		name := fmt.Sprintf("%v", firstKey(it, "name", "title", "content"))
		project := fmt.Sprintf("%v", firstKey(it, "projectName", "companyName"))
		rows[i] = []string{
			id,
			format.Truncate(name, 45),
			format.Truncate(project, 30),
		}
	}
	if mode == format.CSV {
		format.PrintCSV(headers, rows)
	} else {
		format.PrintTable(os.Stdout, headers, rows)
		fmt.Printf("\n%d result(s) for %q (%s)\n", len(items), args[0], kind)
	}
}

func firstKey(m map[string]interface{}, keys ...string) interface{} {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			return v
		}
	}
	return ""
}
