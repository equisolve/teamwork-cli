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

var projectsCmd = &cobra.Command{
	Use:     "projects",
	Aliases: []string{"project"},
	Short:   "List and view Teamwork projects",
}

var projectsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List projects",
	Run:   runProjectsList,
}

var projectsShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show project details",
	Args:  cobra.ExactArgs(1),
	Run:   runProjectsShow,
}

func init() {
	projectsListCmd.Flags().String("status", "active", "Filter by status: active, archived, current, late, upcoming, completed, all")
	projectsListCmd.Flags().StringP("company", "c", "", "Filter by company ID")
	projectsListCmd.Flags().String("search", "", "Search project names")
	projectsListCmd.Flags().Int("page", 1, "Page number")
	projectsListCmd.Flags().Int("page-size", 50, "Results per page")

	projectsCmd.AddCommand(projectsListCmd, projectsShowCmd)
	rootCmd.AddCommand(projectsCmd)
}

func runProjectsList(cmd *cobra.Command, args []string) {
	client := getClient()
	mode := getOutputMode()
	params := url.Values{}
	params.Set("include", "companies")

	if v, _ := cmd.Flags().GetString("status"); v != "" {
		params.Set("projectStatuses", v) // v3 uses projectStatuses (comma-sep)
	}
	if v, _ := cmd.Flags().GetString("company"); v != "" {
		params.Set("companyIds", v)
	}
	if v, _ := cmd.Flags().GetString("search"); v != "" {
		params.Set("searchTerm", v)
	}
	page, _ := cmd.Flags().GetInt("page")
	pageSize, _ := cmd.Flags().GetInt("page-size")
	params.Set("page", fmt.Sprintf("%d", page))
	params.Set("pageSize", fmt.Sprintf("%d", pageSize))

	data, err := client.Get("/projects/api/v3/projects.json", params)
	if err != nil {
		exitOnError(err)
	}

	if mode == format.JSON {
		format.PrintJSON(data)
		return
	}

	var resp struct {
		Projects []struct {
			ID      int    `json:"id"`
			Name    string `json:"name"`
			Status  string `json:"status"`
			Company struct {
				ID int `json:"id"`
			} `json:"company"`
			StartAt   string `json:"startAt"`
			EndAt     string `json:"endAt"`
			UpdatedAt string `json:"updatedAt"`
		} `json:"projects"`
		Meta struct {
			Page struct {
				Count int `json:"count"`
			} `json:"page"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		fmt.Fprintln(os.Stderr, "Error parsing response:", err)
		exitFn(1)
	}
	included := api.ParseIncluded(data)

	headers := []string{"ID", "NAME", "COMPANY", "STATUS", "END DATE", "UPDATED"}
	rows := make([][]string, len(resp.Projects))
	for i, p := range resp.Projects {
		company := included.LookupString("companies", fmt.Sprintf("%d", p.Company.ID), "name")
		rows[i] = []string{
			fmt.Sprintf("%d", p.ID),
			format.Truncate(p.Name, 45),
			format.Truncate(company, 25),
			p.Status,
			formatDate(p.EndAt),
			formatDate(p.UpdatedAt),
		}
	}

	if mode == format.CSV {
		format.PrintCSV(headers, rows)
	} else {
		format.PrintTable(os.Stdout, headers, rows)
		fmt.Printf("\nPage %d · %d of %d project(s)\n", page, len(resp.Projects), resp.Meta.Page.Count)
	}
}

func runProjectsShow(cmd *cobra.Command, args []string) {
	client := getClient()
	mode := getOutputMode()

	data, err := client.Get("/projects/api/v3/projects/"+args[0]+".json?include=companies", nil)
	if err != nil {
		exitOnError(err)
	}

	if mode == format.JSON {
		format.PrintJSON(data)
		return
	}

	wrap, err := decodeMap(data)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error parsing response:", err)
		exitFn(1)
	}
	p, _ := wrap["project"].(map[string]interface{})
	if p == nil {
		format.PrintJSON(data)
		return
	}
	included := api.ParseIncluded(data)

	fields := []struct{ label, key string }{
		{"ID", "id"},
		{"Name", "name"},
		{"Status", "status"},
		{"Sub-status", "subStatus"},
		{"Description", "description"},
		{"Start", "startAt"},
		{"End", "endAt"},
		{"Created", "createdAt"},
		{"Updated", "updatedAt"},
	}
	for _, f := range fields {
		if v, ok := p[f.key]; ok && v != nil {
			val := fmt.Sprintf("%v", v)
			if strings.HasSuffix(f.key, "At") {
				val = formatDate(val)
			}
			if val == "" {
				continue
			}
			fmt.Printf("%-14s %s\n", f.label+":", val)
		}
	}
	if comp, ok := p["company"].(map[string]interface{}); ok {
		if id, ok := comp["id"]; ok {
			name := included.LookupString("companies", fmt.Sprintf("%v", id), "name")
			if name != "" {
				fmt.Printf("%-14s %s\n", "Company:", name)
			}
		}
	}
}
