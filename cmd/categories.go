package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/equisolve/teamwork-cli/internal/format"
	"github.com/spf13/cobra"
)

var categoriesCmd = &cobra.Command{
	Use:     "categories",
	Aliases: []string{"category"},
	Short:   "List categories for projects, messages, files, notebooks, or links",
}

var categoriesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List categories for a given kind",
	Run:   runCategoriesList,
}

func init() {
	categoriesListCmd.Flags().String("kind", "project", "Kind: project, message, file, notebook, link")
	categoriesListCmd.Flags().StringP("project", "p", "", "Scope to a project (not applicable to project categories)")
	categoriesCmd.AddCommand(categoriesListCmd)
	rootCmd.AddCommand(categoriesCmd)
}

func runCategoriesList(cmd *cobra.Command, args []string) {
	client := getClient()
	mode := getOutputMode()
	kind, _ := cmd.Flags().GetString("kind")

	var path string
	switch strings.ToLower(kind) {
	case "project":
		path = "/projectCategories.json"
	case "message":
		path = categoryProjectPath(cmd, "messageCategories.json")
	case "file":
		path = categoryProjectPath(cmd, "fileCategories.json")
	case "notebook":
		path = categoryProjectPath(cmd, "notebookCategories.json")
	case "link":
		path = categoryProjectPath(cmd, "linkCategories.json")
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown kind %q\n", kind)
		exitFn(1)
		return
	}

	data, err := client.Get(path, nil)
	if err != nil {
		exitOnError(err)
	}
	if mode == format.JSON {
		format.PrintJSON(data)
		return
	}
	var resp struct {
		Categories []struct {
			ID    json.Number `json:"id"`
			Name  string      `json:"name"`
			Color string      `json:"color"`
			Count json.Number `json:"count"`
		} `json:"categories"`
	}
	_ = json.Unmarshal(data, &resp)

	headers := []string{"ID", "NAME", "COLOR", "COUNT"}
	rows := make([][]string, len(resp.Categories))
	for i, c := range resp.Categories {
		rows[i] = []string{
			c.ID.String(),
			format.Truncate(c.Name, 40),
			c.Color,
			c.Count.String(),
		}
	}
	if mode == format.CSV {
		format.PrintCSV(headers, rows)
	} else {
		format.PrintTable(os.Stdout, headers, rows)
		fmt.Printf("\n%d %s categor(ies)\n", len(rows), kind)
	}
}

// categoryProjectPath requires --project for non-project categories.
func categoryProjectPath(cmd *cobra.Command, suffix string) string {
	v, _ := cmd.Flags().GetString("project")
	if v == "" {
		fmt.Fprintln(os.Stderr, "Error: --project is required for this category kind")
		exitFn(1)
	}
	pid, err := getResolver().Project(v)
	if err != nil {
		exitOnError(err)
	}
	return fmt.Sprintf("/projects/%d/%s", pid, suffix)
}
