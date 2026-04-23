package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/equisolve/teamwork-cli/internal/format"
	"github.com/spf13/cobra"
)

var linksCmd = &cobra.Command{
	Use:     "links",
	Aliases: []string{"link"},
	Short:   "List and view project links (v1)",
}

var linksListCmd = &cobra.Command{
	Use:   "list",
	Short: "List links in a project",
	Run:   runLinksList,
}

var linksShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show a single link",
	Args:  cobra.ExactArgs(1),
	Run:   runLinksShow,
}

func init() {
	linksListCmd.Flags().StringP("project", "p", "", "Project ID or name (required)")
	linksCmd.AddCommand(linksListCmd, linksShowCmd)
	rootCmd.AddCommand(linksCmd)
}

func runLinksList(cmd *cobra.Command, args []string) {
	client := getClient()
	mode := getOutputMode()

	projectQ, _ := cmd.Flags().GetString("project")
	if projectQ == "" {
		fmt.Fprintln(os.Stderr, "Error: --project is required")
		exitFn(1)
	}
	pid, err := getResolver().Project(projectQ)
	if err != nil {
		exitOnError(err)
	}

	data, err := client.Get(fmt.Sprintf("/projects/%d/links.json", pid), nil)
	if err != nil {
		exitOnError(err)
	}
	if mode == format.JSON {
		format.PrintJSON(data)
		return
	}
	var resp struct {
		Links []struct {
			ID      json.Number `json:"id"`
			Name    string      `json:"name"`
			URL     string      `json:"url"`
			Private string      `json:"private"`
			Category string     `json:"category-name"`
		} `json:"links"`
	}
	_ = json.Unmarshal(data, &resp)

	headers := []string{"ID", "NAME", "URL", "CATEGORY"}
	rows := make([][]string, len(resp.Links))
	for i, l := range resp.Links {
		rows[i] = []string{
			l.ID.String(),
			format.Truncate(l.Name, 35),
			format.Truncate(l.URL, 50),
			format.Truncate(l.Category, 20),
		}
	}
	if mode == format.CSV {
		format.PrintCSV(headers, rows)
	} else {
		format.PrintTable(os.Stdout, headers, rows)
		fmt.Printf("\n%d link(s)\n", len(resp.Links))
	}
}

func runLinksShow(cmd *cobra.Command, args []string) {
	client := getClient()
	mode := getOutputMode()
	data, err := client.Get("/links/"+args[0]+".json", nil)
	if err != nil {
		exitOnError(err)
	}
	if mode == format.JSON {
		format.PrintJSON(data)
		return
	}
	wrap, _ := decodeMap(data)
	l, _ := wrap["link"].(map[string]interface{})
	if l == nil {
		format.PrintJSON(data)
		return
	}
	for _, f := range []struct{ label, key string }{
		{"ID", "id"}, {"Name", "name"}, {"URL", "url"},
		{"Description", "description"}, {"Category", "category-name"},
	} {
		if v, ok := l[f.key]; ok && v != nil {
			val := fmt.Sprintf("%v", v)
			if val == "" {
				continue
			}
			fmt.Printf("%-13s %s\n", f.label+":", val)
		}
	}
}
