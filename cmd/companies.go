package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/equisolve/teamwork-cli/internal/format"
	"github.com/spf13/cobra"
)

var companiesCmd = &cobra.Command{
	Use:     "companies",
	Aliases: []string{"company"},
	Short:   "List and view Teamwork companies",
}

var companiesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List companies",
	Run:   runCompaniesList,
}

var companiesShowCmd = &cobra.Command{
	Use:   "show <id|name>",
	Short: "Show company details",
	Args:  cobra.ExactArgs(1),
	Run:   runCompaniesShow,
}

func init() {
	companiesListCmd.Flags().String("search", "", "Search company names")
	companiesListCmd.Flags().Int("page", 1, "Page number")
	companiesListCmd.Flags().Int("page-size", 100, "Results per page")

	companiesCmd.AddCommand(companiesListCmd, companiesShowCmd)
	rootCmd.AddCommand(companiesCmd)
}

func runCompaniesList(cmd *cobra.Command, args []string) {
	client := getClient()
	mode := getOutputMode()
	params := url.Values{}

	if v, _ := cmd.Flags().GetString("search"); v != "" {
		params.Set("searchTerm", v)
	}
	page, _ := cmd.Flags().GetInt("page")
	pageSize, _ := cmd.Flags().GetInt("page-size")
	params.Set("page", fmt.Sprintf("%d", page))
	params.Set("pageSize", fmt.Sprintf("%d", pageSize))

	data, err := client.Get("/projects/api/v3/companies.json", params)
	if err != nil {
		exitOnError(err)
	}

	if mode == format.JSON {
		format.PrintJSON(data)
		return
	}

	var resp struct {
		Companies []struct {
			ID          int    `json:"id"`
			Name        string `json:"name"`
			EmailOne    string `json:"emailOne"`
			Phone       string `json:"phone"`
			Website     string `json:"website"`
			CountryCode string `json:"countryCode"`
			UpdatedAt   string `json:"updatedAt"`
		} `json:"companies"`
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

	headers := []string{"ID", "NAME", "EMAIL", "PHONE", "WEBSITE", "COUNTRY", "UPDATED"}
	rows := make([][]string, len(resp.Companies))
	for i, c := range resp.Companies {
		rows[i] = []string{
			fmt.Sprintf("%d", c.ID),
			format.Truncate(c.Name, 35),
			format.Truncate(c.EmailOne, 30),
			format.Truncate(c.Phone, 18),
			format.Truncate(c.Website, 30),
			c.CountryCode,
			formatDate(c.UpdatedAt),
		}
	}

	if mode == format.CSV {
		format.PrintCSV(headers, rows)
	} else {
		format.PrintTable(os.Stdout, headers, rows)
		fmt.Printf("\nPage %d · %d of %d companies\n", page, len(resp.Companies), resp.Meta.Page.Count)
	}
}

func runCompaniesShow(cmd *cobra.Command, args []string) {
	client := getClient()
	mode := getOutputMode()

	id, err := getResolver().Company(args[0])
	if err != nil {
		exitOnError(err)
	}

	data, err := client.Get(fmt.Sprintf("/projects/api/v3/companies/%d.json", id), nil)
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
	c, _ := wrap["company"].(map[string]interface{})
	if c == nil {
		format.PrintJSON(data)
		return
	}

	fields := []struct{ label, key string }{
		{"ID", "id"},
		{"Name", "name"},
		{"Email", "emailOne"},
		{"Phone", "phone"},
		{"Website", "website"},
		{"Address 1", "addressOne"},
		{"Address 2", "addressTwo"},
		{"City", "city"},
		{"State", "state"},
		{"Zip", "zip"},
		{"Country", "countryCode"},
		{"Created", "createdAt"},
		{"Updated", "updatedAt"},
	}
	for _, f := range fields {
		if v, ok := c[f.key]; ok && v != nil {
			val := fmt.Sprintf("%v", v)
			if val == "" {
				continue
			}
			if strings.HasSuffix(f.key, "At") {
				val = formatDate(val)
			}
			fmt.Printf("%-13s %s\n", f.label+":", val)
		}
	}
}
