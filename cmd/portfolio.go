package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/equisolve/teamwork-cli/internal/format"
	"github.com/spf13/cobra"
)

var portfolioCmd = &cobra.Command{
	Use:   "portfolio",
	Short: "Portfolio boards, columns, and cards",
}

var portfolioBoardsCmd = &cobra.Command{
	Use:   "boards",
	Short: "List or show portfolio boards",
}

var portfolioBoardsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List portfolio boards",
	Run:   runPortfolioBoardsList,
}

var portfolioBoardsShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show a portfolio board",
	Args:  cobra.ExactArgs(1),
	Run:   runPortfolioBoardsShow,
}

func init() {
	portfolioBoardsCmd.AddCommand(portfolioBoardsListCmd, portfolioBoardsShowCmd)
	portfolioCmd.AddCommand(portfolioBoardsCmd)
	rootCmd.AddCommand(portfolioCmd)
}

func runPortfolioBoardsList(cmd *cobra.Command, args []string) {
	client := getClient()
	mode := getOutputMode()
	data, err := client.Get("/portfolio/boards.json", nil)
	if err != nil {
		exitOnError(err)
	}
	if mode == format.JSON {
		format.PrintJSON(data)
		return
	}
	var resp struct {
		Boards []struct {
			ID          json.Number `json:"id"`
			Name        string      `json:"name"`
			Description string      `json:"description"`
		} `json:"boards"`
	}
	_ = json.Unmarshal(data, &resp)
	headers := []string{"ID", "NAME", "DESCRIPTION"}
	rows := make([][]string, len(resp.Boards))
	for i, b := range resp.Boards {
		rows[i] = []string{
			b.ID.String(),
			format.Truncate(b.Name, 30),
			format.Truncate(b.Description, 50),
		}
	}
	if mode == format.CSV {
		format.PrintCSV(headers, rows)
	} else {
		format.PrintTable(os.Stdout, headers, rows)
		fmt.Printf("\n%d board(s)\n", len(rows))
	}
}

func runPortfolioBoardsShow(cmd *cobra.Command, args []string) {
	client := getClient()
	mode := getOutputMode()
	data, err := client.Get("/portfolio/boards/"+args[0]+".json", nil)
	if err != nil {
		exitOnError(err)
	}
	if mode == format.JSON {
		format.PrintJSON(data)
		return
	}
	wrap, _ := decodeMap(data)
	b, _ := wrap["board"].(map[string]interface{})
	if b == nil {
		format.PrintJSON(data)
		return
	}
	for _, f := range []struct{ label, key string }{
		{"ID", "id"}, {"Name", "name"}, {"Description", "description"},
	} {
		if v, ok := b[f.key]; ok && v != nil {
			val := fmt.Sprintf("%v", v)
			if val == "" {
				continue
			}
			fmt.Printf("%-13s %s\n", f.label+":", val)
		}
	}
}
