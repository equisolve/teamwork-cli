package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/equisolve/teamwork-cli/internal/format"
	"github.com/spf13/cobra"
)

var templatesCmd = &cobra.Command{
	Use:     "templates",
	Aliases: []string{"template"},
	Short:   "List and view project templates (v3)",
}

var templatesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List project templates",
	Run:   runTemplatesList,
}

var templatesShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show a project template",
	Args:  cobra.ExactArgs(1),
	Run:   runTemplatesShow,
}

func init() {
	templatesCmd.AddCommand(templatesListCmd, templatesShowCmd)
	rootCmd.AddCommand(templatesCmd)
}

func runTemplatesList(cmd *cobra.Command, args []string) {
	client := getClient()
	mode := getOutputMode()
	data, err := client.Get("/projects/api/v3/projects/templates.json", nil)
	if err != nil {
		exitOnError(err)
	}
	if mode == format.JSON {
		format.PrintJSON(data)
		return
	}
	var resp struct {
		Templates []struct {
			ID          int    `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"templates"`
	}
	_ = json.Unmarshal(data, &resp)
	headers := []string{"ID", "NAME", "DESCRIPTION"}
	rows := make([][]string, len(resp.Templates))
	for i, t := range resp.Templates {
		rows[i] = []string{
			fmt.Sprintf("%d", t.ID),
			format.Truncate(t.Name, 40),
			format.Truncate(t.Description, 60),
		}
	}
	if mode == format.CSV {
		format.PrintCSV(headers, rows)
	} else {
		format.PrintTable(os.Stdout, headers, rows)
		fmt.Printf("\n%d template(s)\n", len(rows))
	}
}

func runTemplatesShow(cmd *cobra.Command, args []string) {
	client := getClient()
	mode := getOutputMode()
	data, err := client.Get("/projects/api/v3/projects/templates/"+args[0]+".json", nil)
	if err != nil {
		exitOnError(err)
	}
	if mode == format.JSON {
		format.PrintJSON(data)
		return
	}
	wrap, _ := decodeMap(data)
	t, _ := wrap["template"].(map[string]interface{})
	if t == nil {
		format.PrintJSON(data)
		return
	}
	for _, f := range []struct{ label, key string }{
		{"ID", "id"}, {"Name", "name"}, {"Description", "description"},
	} {
		if v, ok := t[f.key]; ok && v != nil {
			val := fmt.Sprintf("%v", v)
			if val == "" {
				continue
			}
			fmt.Printf("%-13s %s\n", f.label+":", val)
		}
	}
}
