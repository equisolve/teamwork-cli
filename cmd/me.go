package cmd

import (
	"fmt"
	"os"

	"github.com/equisolve/teamwork-cli/internal/format"
	"github.com/spf13/cobra"
)

var meCmd = &cobra.Command{
	Use:   "me",
	Short: "Show authenticated Teamwork user",
	Run: func(cmd *cobra.Command, args []string) {
		client := getClient()
		mode := getOutputMode()

		data, err := client.Get("/me.json", nil)
		if err != nil {
			exitOnError(err)
		}

		if mode == format.JSON {
			format.PrintJSON(data)
			return
		}

		m, err := decodeMap(data)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error parsing response:", err)
			exitFn(1)
		}

		person, _ := m["person"].(map[string]interface{})
		if person == nil {
			format.PrintJSON(data)
			return
		}

		fields := []struct{ label, key string }{
			{"ID", "id"},
			{"Name", "full-name"},
			{"Email", "email-address"},
			{"Title", "title"},
			{"Company", "company-name"},
			{"Admin", "administrator"},
			{"Timezone", "user-timezone"},
			{"Last login", "last-login"},
		}
		for _, f := range fields {
			if v, ok := person[f.key]; ok && v != nil && fmt.Sprintf("%v", v) != "" {
				fmt.Printf("%-12s %v\n", f.label+":", v)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(meCmd)
}
