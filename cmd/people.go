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

var peopleCmd = &cobra.Command{
	Use:     "people",
	Aliases: []string{"person", "users"},
	Short:   "List and view Teamwork people",
}

var peopleListCmd = &cobra.Command{
	Use:   "list",
	Short: "List people",
	Run:   runPeopleList,
}

var peopleShowCmd = &cobra.Command{
	Use:   "show <id|me|name-or-email>",
	Short: "Show person details",
	Args:  cobra.ExactArgs(1),
	Run:   runPeopleShow,
}

func init() {
	peopleListCmd.Flags().StringP("project", "p", "", "Filter by project ID or name")
	peopleListCmd.Flags().StringP("company", "c", "", "Filter by company ID or name")
	peopleListCmd.Flags().String("search", "", "Search name/email")
	peopleListCmd.Flags().Int("page", 1, "Page number")
	peopleListCmd.Flags().Int("page-size", 100, "Results per page")

	peopleCmd.AddCommand(peopleListCmd, peopleShowCmd)
	rootCmd.AddCommand(peopleCmd)
}

func runPeopleList(cmd *cobra.Command, args []string) {
	client := getClient()
	mode := getOutputMode()
	params := url.Values{}
	params.Set("include", "companies")

	if v, _ := cmd.Flags().GetString("project"); v != "" {
		pid, err := getResolver().Project(v)
		if err != nil {
			exitOnError(err)
		}
		params.Set("projectIds", fmt.Sprintf("%d", pid))
	}
	if v, _ := cmd.Flags().GetString("company"); v != "" {
		cid, err := getResolver().Company(v)
		if err != nil {
			exitOnError(err)
		}
		params.Set("companyIds", fmt.Sprintf("%d", cid))
	}
	if v, _ := cmd.Flags().GetString("search"); v != "" {
		params.Set("searchTerm", v)
	}
	page, _ := cmd.Flags().GetInt("page")
	pageSize, _ := cmd.Flags().GetInt("page-size")
	params.Set("page", fmt.Sprintf("%d", page))
	params.Set("pageSize", fmt.Sprintf("%d", pageSize))

	data, err := client.Get("/projects/api/v3/people.json", params)
	if err != nil {
		exitOnError(err)
	}

	if mode == format.JSON {
		format.PrintJSON(data)
		return
	}

	var resp struct {
		People []struct {
			ID        int    `json:"id"`
			FirstName string `json:"firstName"`
			LastName  string `json:"lastName"`
			Email     string `json:"email"`
			Title     string `json:"title"`
			CompanyID int    `json:"companyId"`
			UserType  string `json:"userType"`
			LastLogin string `json:"lastLogin"`
		} `json:"people"`
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

	headers := []string{"ID", "NAME", "EMAIL", "COMPANY", "TITLE", "LAST LOGIN"}
	rows := make([][]string, len(resp.People))
	for i, p := range resp.People {
		company := included.LookupString("companies", fmt.Sprintf("%d", p.CompanyID), "name")
		rows[i] = []string{
			fmt.Sprintf("%d", p.ID),
			format.Truncate(strings.TrimSpace(p.FirstName+" "+p.LastName), 30),
			format.Truncate(p.Email, 35),
			format.Truncate(company, 25),
			format.Truncate(p.Title, 25),
			formatDate(p.LastLogin),
		}
	}

	if mode == format.CSV {
		format.PrintCSV(headers, rows)
	} else {
		format.PrintTable(os.Stdout, headers, rows)
		fmt.Printf("\nPage %d · %d of %d person/people\n", page, len(resp.People), resp.Meta.Page.Count)
	}
}

func runPeopleShow(cmd *cobra.Command, args []string) {
	client := getClient()
	mode := getOutputMode()

	id, err := getResolver().Person(args[0])
	if err != nil {
		exitOnError(err)
	}

	data, err := client.Get(fmt.Sprintf("/projects/api/v3/people/%d.json?include=companies", id), nil)
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
	p, _ := wrap["person"].(map[string]interface{})
	if p == nil {
		format.PrintJSON(data)
		return
	}
	included := api.ParseIncluded(data)

	fields := []struct{ label, key string }{
		{"ID", "id"},
		{"First name", "firstName"},
		{"Last name", "lastName"},
		{"Email", "email"},
		{"Title", "title"},
		{"User type", "userType"},
		{"Timezone", "timezone"},
		{"Last login", "lastLogin"},
		{"Created", "createdAt"},
	}
	for _, f := range fields {
		if v, ok := p[f.key]; ok && v != nil {
			val := fmt.Sprintf("%v", v)
			if val == "" {
				continue
			}
			if strings.HasSuffix(f.key, "Login") || strings.HasSuffix(f.key, "At") {
				val = formatDate(val)
			}
			fmt.Printf("%-13s %s\n", f.label+":", val)
		}
	}
	if cid, ok := p["companyId"]; ok {
		name := included.LookupString("companies", fmt.Sprintf("%v", cid), "name")
		if name != "" {
			fmt.Printf("%-13s %s\n", "Company:", name)
		}
	}
}
