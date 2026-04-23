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

var expensesCmd = &cobra.Command{
	Use:     "expenses",
	Aliases: []string{"expense"},
	Short:   "List and view project expenses",
}

var expensesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List expenses",
	Run:   runExpensesList,
}

var expensesShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show expense details",
	Args:  cobra.ExactArgs(1),
	Run:   runExpensesShow,
}

func init() {
	expensesListCmd.Flags().StringP("project", "p", "", "Filter by project ID or name")
	expensesListCmd.Flags().Int("page", 1, "Page number")
	expensesListCmd.Flags().Int("page-size", 25, "Results per page")

	expensesCmd.AddCommand(expensesListCmd, expensesShowCmd)
	rootCmd.AddCommand(expensesCmd)
}

func runExpensesList(cmd *cobra.Command, args []string) {
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

	data, err := client.Get("/projects/api/v3/expenses.json", params)
	if err != nil {
		exitOnError(err)
	}
	if mode == format.JSON {
		format.PrintJSON(data)
		return
	}
	var resp struct {
		Expenses []struct {
			ID          int     `json:"id"`
			Name        string  `json:"name"`
			Description string  `json:"description"`
			Cost        float64 `json:"cost"`
			Currency    string  `json:"currencyCode"`
			ProjectID   int     `json:"projectId"`
			Date        string  `json:"date"`
		} `json:"expenses"`
		Meta struct{ Page struct{ Count int `json:"count"` } `json:"page"` } `json:"meta"`
	}
	_ = json.Unmarshal(data, &resp)
	included := api.ParseIncluded(data)

	headers := []string{"ID", "NAME", "PROJECT", "COST", "DATE"}
	rows := make([][]string, len(resp.Expenses))
	for i, e := range resp.Expenses {
		project := included.LookupString("projects", fmt.Sprintf("%d", e.ProjectID), "name")
		rows[i] = []string{
			fmt.Sprintf("%d", e.ID),
			format.Truncate(e.Name, 35),
			format.Truncate(project, 25),
			fmt.Sprintf("%s %.2f", e.Currency, e.Cost),
			formatDate(e.Date),
		}
	}
	if mode == format.CSV {
		format.PrintCSV(headers, rows)
	} else {
		format.PrintTable(os.Stdout, headers, rows)
		fmt.Printf("\nPage %d · %d of %d expense(s)\n", page, len(resp.Expenses), resp.Meta.Page.Count)
	}
}

func runExpensesShow(cmd *cobra.Command, args []string) {
	client := getClient()
	mode := getOutputMode()
	data, err := client.Get("/projects/api/v3/expenses/"+args[0]+".json", nil)
	if err != nil {
		exitOnError(err)
	}
	if mode == format.JSON {
		format.PrintJSON(data)
		return
	}
	wrap, _ := decodeMap(data)
	e, _ := wrap["expense"].(map[string]interface{})
	if e == nil {
		format.PrintJSON(data)
		return
	}
	for _, f := range []struct{ label, key string }{
		{"ID", "id"}, {"Name", "name"}, {"Description", "description"},
		{"Cost", "cost"}, {"Currency", "currencyCode"}, {"Date", "date"},
	} {
		if v, ok := e[f.key]; ok && v != nil {
			val := fmt.Sprintf("%v", v)
			if val == "" {
				continue
			}
			fmt.Printf("%-13s %s\n", f.label+":", val)
		}
	}
}
