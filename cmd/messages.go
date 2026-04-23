package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/equisolve/teamwork-cli/internal/api"
	"github.com/equisolve/teamwork-cli/internal/format"
	"github.com/spf13/cobra"
)

var messagesCmd = &cobra.Command{
	Use:     "messages",
	Aliases: []string{"message"},
	Short:   "List and view project messages",
}

var messagesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List messages",
	Run:   runMessagesList,
}

var messagesShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show message details",
	Args:  cobra.ExactArgs(1),
	Run:   runMessagesShow,
}

func init() {
	messagesListCmd.Flags().StringP("project", "p", "", "Filter by project ID or name")
	messagesListCmd.Flags().Int("page", 1, "Page number")
	messagesListCmd.Flags().Int("page-size", 25, "Results per page")

	messagesCmd.AddCommand(messagesListCmd, messagesShowCmd)
	rootCmd.AddCommand(messagesCmd)
}

func runMessagesList(cmd *cobra.Command, args []string) {
	client := getClient()
	mode := getOutputMode()
	params := url.Values{}
	params.Set("include", "projects,users")

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

	data, err := client.Get("/projects/api/v3/messages.json", params)
	if err != nil {
		exitOnError(err)
	}
	if mode == format.JSON {
		format.PrintJSON(data)
		return
	}

	var resp struct {
		Messages []struct {
			ID        int    `json:"id"`
			Title     string `json:"title"`
			Status    string `json:"status"`
			ProjectID int    `json:"projectId"`
			Author    struct {
				ID int `json:"id"`
			} `json:"author"`
			CreatedAt string `json:"createdAt"`
		} `json:"messages"`
		Meta struct {
			Page struct{ Count int `json:"count"` } `json:"page"`
		} `json:"meta"`
	}
	_ = json.Unmarshal(data, &resp)
	included := api.ParseIncluded(data)

	headers := []string{"ID", "TITLE", "PROJECT", "AUTHOR", "STATUS", "CREATED"}
	rows := make([][]string, len(resp.Messages))
	for i, m := range resp.Messages {
		project := included.LookupString("projects", fmt.Sprintf("%d", m.ProjectID), "name")
		first := included.LookupString("users", fmt.Sprintf("%d", m.Author.ID), "firstName")
		last := included.LookupString("users", fmt.Sprintf("%d", m.Author.ID), "lastName")
		rows[i] = []string{
			fmt.Sprintf("%d", m.ID),
			format.Truncate(m.Title, 40),
			format.Truncate(project, 20),
			format.Truncate(strings.TrimSpace(first+" "+last), 20),
			m.Status,
			formatDate(m.CreatedAt),
		}
	}

	if mode == format.CSV {
		format.PrintCSV(headers, rows)
	} else {
		format.PrintTable(os.Stdout, headers, rows)
		fmt.Printf("\nPage %d · %d of %d message(s)\n", page, len(resp.Messages), resp.Meta.Page.Count)
	}
}

func runMessagesShow(cmd *cobra.Command, args []string) {
	client := getClient()
	mode := getOutputMode()

	data, err := client.Get("/projects/api/v3/messages/"+args[0]+".json?include=projects,users", nil)
	if err != nil {
		exitOnError(err)
	}
	if mode == format.JSON {
		format.PrintJSON(data)
		return
	}
	wrap, _ := decodeMap(data)
	m, _ := wrap["message"].(map[string]interface{})
	if m == nil {
		format.PrintJSON(data)
		return
	}
	for _, f := range []struct{ label, key string }{
		{"ID", "id"}, {"Title", "title"}, {"Status", "status"},
		{"Created", "createdAt"}, {"Updated", "updatedAt"},
	} {
		if v, ok := m[f.key]; ok && v != nil {
			val := fmt.Sprintf("%v", v)
			if val == "" {
				continue
			}
			if strings.HasSuffix(f.key, "At") {
				val = formatDate(val)
			}
			fmt.Printf("%-10s %s\n", f.label+":", val)
		}
	}
	if body, ok := m["body"].(string); ok && body != "" {
		fmt.Println("\n" + body)
	}
}
