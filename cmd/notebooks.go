package cmd

import (
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/equisolve/teamwork-cli/internal/api"
	"github.com/equisolve/teamwork-cli/internal/format"
	"github.com/spf13/cobra"
)

var notebooksCmd = &cobra.Command{
	Use:     "notebooks",
	Aliases: []string{"notebook"},
	Short:   "List and view project notebooks",
}

var notebooksListCmd = &cobra.Command{
	Use:   "list",
	Short: "List notebooks",
	Run:   runNotebooksList,
}

var notebooksShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show notebook details",
	Args:  cobra.ExactArgs(1),
	Run:   runNotebooksShow,
}

func init() {
	notebooksListCmd.Flags().StringP("project", "p", "", "Filter by project ID or name")
	notebooksListCmd.Flags().Int("page", 1, "Page number")
	notebooksListCmd.Flags().Int("page-size", 25, "Results per page")

	notebooksShowCmd.Flags().Bool("content", false, "Print only the notebook body (HTML)")
	notebooksShowCmd.Flags().Bool("plain", false, "Strip HTML tags when printing the body")
	notebooksShowCmd.Flags().String("section", "", "Extract a single <h2>-titled section by name (case-insensitive substring)")

	notebooksCmd.AddCommand(notebooksListCmd, notebooksShowCmd)
	rootCmd.AddCommand(notebooksCmd)
}

func runNotebooksList(cmd *cobra.Command, args []string) {
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

	data, err := client.Get("/projects/api/v3/notebooks.json", params)
	if err != nil {
		exitOnError(err)
	}
	if mode == format.JSON {
		format.PrintJSON(data)
		return
	}
	var resp struct {
		Notebooks []struct {
			ID        int    `json:"id"`
			Name      string `json:"name"`
			ProjectID int    `json:"projectId"`
			Locked    bool   `json:"locked"`
			Type      string `json:"type"`
		} `json:"notebooks"`
		Meta struct{ Page struct{ Count int `json:"count"` } `json:"page"` } `json:"meta"`
	}
	_ = json.Unmarshal(data, &resp)
	included := api.ParseIncluded(data)

	headers := []string{"ID", "NAME", "PROJECT", "TYPE", "LOCKED"}
	rows := make([][]string, len(resp.Notebooks))
	for i, n := range resp.Notebooks {
		project := included.LookupString("projects", fmt.Sprintf("%d", n.ProjectID), "name")
		locked := ""
		if n.Locked {
			locked = "Y"
		}
		rows[i] = []string{
			fmt.Sprintf("%d", n.ID),
			format.Truncate(n.Name, 40),
			format.Truncate(project, 25),
			n.Type,
			locked,
		}
	}
	if mode == format.CSV {
		format.PrintCSV(headers, rows)
	} else {
		format.PrintTable(os.Stdout, headers, rows)
		fmt.Printf("\nPage %d · %d of %d notebook(s)\n", page, len(resp.Notebooks), resp.Meta.Page.Count)
	}
}

func runNotebooksShow(cmd *cobra.Command, args []string) {
	client := getClient()
	mode := getOutputMode()
	contentOnly, _ := cmd.Flags().GetBool("content")
	plain, _ := cmd.Flags().GetBool("plain")
	section, _ := cmd.Flags().GetString("section")

	data, err := client.Get("/projects/api/v3/notebooks/"+args[0]+".json", nil)
	if err != nil {
		exitOnError(err)
	}
	if mode == format.JSON {
		format.PrintJSON(data)
		return
	}
	wrap, _ := decodeMap(data)
	n, _ := wrap["notebook"].(map[string]interface{})
	if n == nil {
		format.PrintJSON(data)
		return
	}

	body := notebookBody(n)
	// v3 sometimes omits the body. Fall back to v1 with includeContent=true,
	// which is the original endpoint shape that always returns it.
	if body == "" {
		if v1, err := client.Get("/notebooks/"+args[0]+".json", url.Values{"includeContent": []string{"true"}}); err == nil {
			var alt struct {
				Notebook map[string]interface{} `json:"notebook"`
			}
			if json.Unmarshal(v1, &alt) == nil && alt.Notebook != nil {
				body = notebookBody(alt.Notebook)
			}
		}
	}

	if section != "" {
		body = extractSection(body, section)
	}
	if plain {
		body = htmlToText(body)
	}

	if contentOnly {
		fmt.Print(body)
		if body != "" && !strings.HasSuffix(body, "\n") {
			fmt.Println()
		}
		return
	}

	for _, f := range []struct{ label, key string }{
		{"ID", "id"}, {"Name", "name"}, {"Type", "type"},
		{"Description", "description"}, {"Locked", "locked"},
	} {
		if v, ok := n[f.key]; ok && v != nil {
			val := fmt.Sprintf("%v", v)
			if val == "" {
				continue
			}
			fmt.Printf("%-13s %s\n", f.label+":", val)
		}
	}
	if body != "" {
		fmt.Println("\n" + body)
	}
}

// notebookBody returns the HTML body of a notebook, checking v3 ("contents")
// then v1 ("content") field names.
func notebookBody(n map[string]interface{}) string {
	for _, k := range []string{"contents", "content"} {
		if s, ok := n[k].(string); ok && s != "" {
			return s
		}
	}
	return ""
}

var (
	// reH2Open matches an <h2 ...> opening tag.
	reH2Open = regexp.MustCompile(`(?is)<h2[^>]*>`)
	// reH2Title pulls the title out of a chunk that already starts with <h2>.
	reH2Title   = regexp.MustCompile(`(?is)<h2[^>]*>(.*?)</h2>`)
	reTags      = regexp.MustCompile(`(?s)<[^>]+>`)
	reBlankLine = regexp.MustCompile(`(?m)^\s+$`)
	reMultiNL   = regexp.MustCompile(`\n{3,}`)
)

// extractSection returns the chunk of HTML starting at the first <h2> whose
// title contains `name` (case-insensitive) and ending at the next <h2> (or end
// of body). RE2 has no lookahead, so we slice manually instead of using a
// single regex.
func extractSection(htmlBody, name string) string {
	if htmlBody == "" {
		return ""
	}
	starts := reH2Open.FindAllStringIndex(htmlBody, -1)
	if len(starts) == 0 {
		return ""
	}
	needle := strings.ToLower(name)
	for i, span := range starts {
		end := len(htmlBody)
		if i+1 < len(starts) {
			end = starts[i+1][0]
		}
		section := htmlBody[span[0]:end]
		titleMatch := reH2Title.FindStringSubmatch(section)
		if titleMatch == nil {
			continue
		}
		if strings.Contains(strings.ToLower(htmlToText(titleMatch[1])), needle) {
			return section
		}
	}
	return ""
}

// htmlToText is a deliberately tiny tag-stripper. It's enough for notebook
// dumps (h2 / table / p / br) and avoids pulling in a full HTML parser.
func htmlToText(s string) string {
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "</tr>", "\n")
	s = strings.ReplaceAll(s, "</p>", "\n\n")
	s = strings.ReplaceAll(s, "</h1>", "\n\n")
	s = strings.ReplaceAll(s, "</h2>", "\n\n")
	s = strings.ReplaceAll(s, "</h3>", "\n\n")
	s = strings.ReplaceAll(s, "</li>", "\n")
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")
	s = strings.ReplaceAll(s, "</td>", "\t")
	s = strings.ReplaceAll(s, "</th>", "\t")
	s = reTags.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = reBlankLine.ReplaceAllString(s, "")
	s = reMultiNL.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}
