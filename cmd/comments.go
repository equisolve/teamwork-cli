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

// commentResources maps the --on flag to the v1 URL segment. Teamwork's v1
// comments endpoint pattern is /{resource}/{id}/comments.json where resource
// is the plural form of the entity type.
var commentResources = map[string]string{
	"task":        "tasks",
	"tasks":       "tasks",
	"message":     "messages",
	"messages":    "messages",
	"milestone":   "milestones",
	"milestones":  "milestones",
	"notebook":    "notebooks",
	"notebooks":   "notebooks",
	"link":        "links",
	"links":       "links",
	"fileversion": "fileVersions",
	"file":        "fileVersions",
}

var commentsCmd = &cobra.Command{
	Use:   "comments",
	Short: "List or add comments on any resource that supports them",
}

var commentsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List comments on a resource",
	Run:   runCommentsList,
}

var commentsAddCmd = &cobra.Command{
	Use:   "add <body>",
	Short: "Post a comment on a resource",
	Args:  cobra.ExactArgs(1),
	Run:   runCommentsAdd,
}

func init() {
	commentsListCmd.Flags().String("on", "", "Resource kind: task|message|milestone|notebook|link|fileversion")
	commentsListCmd.Flags().String("id", "", "Resource ID")
	commentsListCmd.Flags().Int("page", 1, "Page number")
	commentsListCmd.Flags().Int("page-size", 50, "Results per page")

	commentsAddCmd.Flags().String("on", "", "Resource kind: task|message|milestone|notebook|link|fileversion")
	commentsAddCmd.Flags().String("id", "", "Resource ID")
	commentsAddCmd.Flags().String("content-type", "TEXT", "Content type: TEXT or HTML")
	commentsAddCmd.Flags().Bool("notify", false, "Notify followers of this comment")

	commentsCmd.AddCommand(commentsListCmd, commentsAddCmd)
	rootCmd.AddCommand(commentsCmd)
}

func resolveCommentResource(on string) (string, error) {
	if on == "" {
		return "", fmt.Errorf("--on is required (task|message|milestone|notebook|link|fileversion)")
	}
	seg, ok := commentResources[strings.ToLower(on)]
	if !ok {
		return "", fmt.Errorf("unknown resource kind %q", on)
	}
	return seg, nil
}

func runCommentsList(cmd *cobra.Command, args []string) {
	client := getClient()
	mode := getOutputMode()

	on, _ := cmd.Flags().GetString("on")
	id, _ := cmd.Flags().GetString("id")
	if id == "" {
		fmt.Fprintln(os.Stderr, "Error: --id is required")
		exitFn(1)
	}
	seg, err := resolveCommentResource(on)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		exitFn(1)
	}

	params := url.Values{}
	page, _ := cmd.Flags().GetInt("page")
	pageSize, _ := cmd.Flags().GetInt("page-size")
	params.Set("page", fmt.Sprintf("%d", page))
	params.Set("pageSize", fmt.Sprintf("%d", pageSize))

	data, err := client.Get("/"+seg+"/"+id+"/comments.json", params)
	if err != nil {
		exitOnError(err)
	}

	if mode == format.JSON {
		format.PrintJSON(data)
		return
	}

	var resp struct {
		Comments []struct {
			ID         json.Number `json:"id"`
			AuthorName string      `json:"author-fullname"`
			DateTime   string      `json:"datetime"`
			Body       string      `json:"body"`
		} `json:"comments"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		fmt.Fprintln(os.Stderr, "Error parsing response:", err)
		exitFn(1)
	}

	headers := []string{"ID", "AUTHOR", "WHEN", "BODY"}
	rows := make([][]string, len(resp.Comments))
	for i, c := range resp.Comments {
		rows[i] = []string{
			c.ID.String(),
			format.Truncate(c.AuthorName, 25),
			formatDate(c.DateTime),
			format.Truncate(strings.ReplaceAll(c.Body, "\n", " "), 60),
		}
	}

	if mode == format.CSV {
		format.PrintCSV(headers, rows)
	} else {
		format.PrintTable(os.Stdout, headers, rows)
		fmt.Printf("\n%d comment(s)\n", len(resp.Comments))
	}
}

func runCommentsAdd(cmd *cobra.Command, args []string) {
	client := getClient()
	on, _ := cmd.Flags().GetString("on")
	id, _ := cmd.Flags().GetString("id")
	if id == "" {
		fmt.Fprintln(os.Stderr, "Error: --id is required")
		exitFn(1)
	}
	seg, err := resolveCommentResource(on)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		exitFn(1)
	}
	contentType, _ := cmd.Flags().GetString("content-type")
	notify, _ := cmd.Flags().GetBool("notify")

	comment := map[string]interface{}{
		"body":         args[0],
		"content-type": contentType,
	}
	if notify {
		comment["notify"] = true
	}
	payload := map[string]interface{}{"comment": comment}

	data, err := client.Post("/"+seg+"/"+id+"/comments.json", nil, payload)
	if err != nil {
		exitOnError(err)
	}
	var resp struct {
		CommentID string `json:"commentId"`
		ID        string `json:"id"`
	}
	_ = json.Unmarshal(data, &resp)
	newID := resp.CommentID
	if newID == "" {
		newID = resp.ID
	}
	if newID == "" {
		fmt.Printf("Comment posted on %s %s.\n", on, id)
	} else {
		fmt.Printf("Comment %s posted on %s %s.\n", newID, on, id)
	}
}
