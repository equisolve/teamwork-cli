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

var invoicesCmd = &cobra.Command{
	Use:     "invoices",
	Aliases: []string{"invoice"},
	Short:   "List and view project invoices",
}

var invoicesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List invoices",
	Run:   runInvoicesList,
}

var invoicesShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show invoice details",
	Args:  cobra.ExactArgs(1),
	Run:   runInvoicesShow,
}

func init() {
	invoicesListCmd.Flags().StringP("project", "p", "", "Filter by project ID or name")
	invoicesListCmd.Flags().String("status", "", "Filter by status: open, completed, all")
	invoicesListCmd.Flags().Int("page", 1, "Page number")
	invoicesListCmd.Flags().Int("page-size", 25, "Results per page")

	invoicesCmd.AddCommand(invoicesListCmd, invoicesShowCmd)
	rootCmd.AddCommand(invoicesCmd)
}

func runInvoicesList(cmd *cobra.Command, args []string) {
	client := getClient()
	mode := getOutputMode()
	params := url.Values{}
	params.Set("include", "projects,companies")

	if v, _ := cmd.Flags().GetString("project"); v != "" {
		pid, err := getResolver().Project(v)
		if err != nil {
			exitOnError(err)
		}
		params.Set("projectIds", fmt.Sprintf("%d", pid))
	}
	if v, _ := cmd.Flags().GetString("status"); v != "" {
		params.Set("status", v)
	}
	page, _ := cmd.Flags().GetInt("page")
	pageSize, _ := cmd.Flags().GetInt("page-size")
	params.Set("page", fmt.Sprintf("%d", page))
	params.Set("pageSize", fmt.Sprintf("%d", pageSize))

	data, err := client.Get("/projects/api/v3/invoices.json", params)
	if err != nil {
		exitOnError(err)
	}
	if mode == format.JSON {
		format.PrintJSON(data)
		return
	}
	var resp struct {
		Invoices []struct {
			ID          int     `json:"id"`
			Number      string  `json:"number"`
			Status      string  `json:"status"`
			ProjectID   int     `json:"projectId"`
			Subtotal    float64 `json:"subtotal"`
			Total       float64 `json:"total"`
			Currency    string  `json:"currencyCode"`
			DisplayDate string  `json:"displayDate"`
		} `json:"invoices"`
		Meta struct{ Page struct{ Count int `json:"count"` } `json:"page"` } `json:"meta"`
	}
	_ = json.Unmarshal(data, &resp)
	included := api.ParseIncluded(data)

	headers := []string{"ID", "NUMBER", "PROJECT", "STATUS", "TOTAL", "DATE"}
	rows := make([][]string, len(resp.Invoices))
	for i, inv := range resp.Invoices {
		project := included.LookupString("projects", fmt.Sprintf("%d", inv.ProjectID), "name")
		rows[i] = []string{
			fmt.Sprintf("%d", inv.ID),
			inv.Number,
			format.Truncate(project, 25),
			inv.Status,
			fmt.Sprintf("%s %.2f", inv.Currency, inv.Total),
			formatDate(inv.DisplayDate),
		}
	}
	if mode == format.CSV {
		format.PrintCSV(headers, rows)
	} else {
		format.PrintTable(os.Stdout, headers, rows)
		fmt.Printf("\nPage %d · %d of %d invoice(s)\n", page, len(resp.Invoices), resp.Meta.Page.Count)
	}
}

func runInvoicesShow(cmd *cobra.Command, args []string) {
	client := getClient()
	mode := getOutputMode()
	data, err := client.Get("/projects/api/v3/invoices/"+args[0]+".json?include=projects,companies", nil)
	if err != nil {
		exitOnError(err)
	}
	if mode == format.JSON {
		format.PrintJSON(data)
		return
	}
	wrap, _ := decodeMap(data)
	inv, _ := wrap["invoice"].(map[string]interface{})
	if inv == nil {
		format.PrintJSON(data)
		return
	}
	for _, f := range []struct{ label, key string }{
		{"ID", "id"}, {"Number", "number"}, {"Status", "status"},
		{"Total", "total"}, {"Currency", "currencyCode"},
		{"Description", "description"}, {"Date", "displayDate"},
	} {
		if v, ok := inv[f.key]; ok && v != nil {
			val := fmt.Sprintf("%v", v)
			if val == "" {
				continue
			}
			fmt.Printf("%-13s %s\n", f.label+":", val)
		}
	}
}
