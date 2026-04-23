package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var tasksCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new task in a task list",
	Run:   runTasksCreate,
}

var tasksUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update fields on an existing task",
	Args:  cobra.ExactArgs(1),
	Run:   runTasksUpdate,
}

var tasksDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a task",
	Args:  cobra.ExactArgs(1),
	Run:   runTasksDelete,
}

var tasksUncompleteCmd = &cobra.Command{
	Use:   "uncomplete <id>",
	Short: "Reopen a completed task",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := getClient()
		if _, err := client.Put("/tasks/"+args[0]+"/uncomplete.json", nil, nil); err != nil {
			exitOnError(err)
		}
		fmt.Printf("Task %s reopened.\n", args[0])
	},
}

var tasksSubtasksCmd = &cobra.Command{
	Use:   "subtasks <parent-id>",
	Short: "List subtasks or add several at once",
	Args:  cobra.ExactArgs(1),
	Run:   runTasksSubtasks,
}

func init() {
	tasksCreateCmd.Flags().String("tasklist", "", "Task list ID (required)")
	tasksCreateCmd.Flags().String("name", "", "Task name/content (required)")
	tasksCreateCmd.Flags().String("description", "", "Long description")
	tasksCreateCmd.Flags().String("assignee", "", "Assignee ID, email, name, or 'me'")
	tasksCreateCmd.Flags().String("due", "", "Due date YYYY-MM-DD")
	tasksCreateCmd.Flags().String("start", "", "Start date YYYY-MM-DD")
	tasksCreateCmd.Flags().String("priority", "", "Priority: low, medium, high")
	tasksCreateCmd.Flags().Int("estimate", 0, "Estimated minutes")

	tasksUpdateCmd.Flags().String("name", "", "New task name/content")
	tasksUpdateCmd.Flags().String("description", "", "New description")
	tasksUpdateCmd.Flags().String("assignee", "", "New assignee ID, email, name, or 'me'")
	tasksUpdateCmd.Flags().String("due", "", "New due date YYYY-MM-DD")
	tasksUpdateCmd.Flags().String("start", "", "New start date YYYY-MM-DD")
	tasksUpdateCmd.Flags().String("priority", "", "New priority: low, medium, high")
	tasksUpdateCmd.Flags().Int("estimate", -1, "New estimated minutes (-1 = leave alone)")

	tasksDeleteCmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")

	tasksSubtasksCmd.Flags().String("add", "", "Add subtasks; newline-separated or ~|~")

	tasksCmd.AddCommand(tasksCreateCmd, tasksUpdateCmd, tasksDeleteCmd, tasksUncompleteCmd, tasksSubtasksCmd)
}

func runTasksCreate(cmd *cobra.Command, args []string) {
	client := getClient()
	tasklist, _ := cmd.Flags().GetString("tasklist")
	name, _ := cmd.Flags().GetString("name")
	if tasklist == "" || name == "" {
		fmt.Fprintln(os.Stderr, "Error: --tasklist and --name are required")
		exitFn(1)
	}

	todo := map[string]interface{}{"content": name}
	if v, _ := cmd.Flags().GetString("description"); v != "" {
		todo["description"] = v
	}
	if v, _ := cmd.Flags().GetString("assignee"); v != "" {
		uid, err := getResolver().Person(v)
		if err != nil {
			exitOnError(err)
		}
		todo["responsible-party-id"] = strconv.Itoa(uid)
	}
	if v, _ := cmd.Flags().GetString("due"); v != "" {
		todo["due-date"] = strings.ReplaceAll(v, "-", "")
	}
	if v, _ := cmd.Flags().GetString("start"); v != "" {
		todo["start-date"] = strings.ReplaceAll(v, "-", "")
	}
	if v, _ := cmd.Flags().GetString("priority"); v != "" {
		todo["priority"] = v
	}
	if v, _ := cmd.Flags().GetInt("estimate"); v > 0 {
		todo["estimated-minutes"] = v
	}

	payload := map[string]interface{}{"todo-item": todo}
	data, err := client.Post("/tasklists/"+tasklist+"/tasks.json", nil, payload)
	if err != nil {
		exitOnError(err)
	}
	var resp struct {
		ID       string `json:"id"`
		TaskID   string `json:"taskId"`
	}
	_ = json.Unmarshal(data, &resp)
	id := resp.TaskID
	if id == "" {
		id = resp.ID
	}
	fmt.Printf("Task created: %s\n", id)
}

func runTasksUpdate(cmd *cobra.Command, args []string) {
	client := getClient()
	todo := map[string]interface{}{}
	if v, _ := cmd.Flags().GetString("name"); v != "" {
		todo["content"] = v
	}
	if v, _ := cmd.Flags().GetString("description"); v != "" {
		todo["description"] = v
	}
	if v, _ := cmd.Flags().GetString("assignee"); v != "" {
		uid, err := getResolver().Person(v)
		if err != nil {
			exitOnError(err)
		}
		todo["responsible-party-id"] = strconv.Itoa(uid)
	}
	if v, _ := cmd.Flags().GetString("due"); v != "" {
		todo["due-date"] = strings.ReplaceAll(v, "-", "")
	}
	if v, _ := cmd.Flags().GetString("start"); v != "" {
		todo["start-date"] = strings.ReplaceAll(v, "-", "")
	}
	if v, _ := cmd.Flags().GetString("priority"); v != "" {
		todo["priority"] = v
	}
	if v, _ := cmd.Flags().GetInt("estimate"); v >= 0 {
		todo["estimated-minutes"] = v
	}
	if len(todo) == 0 {
		fmt.Fprintln(os.Stderr, "Error: no updates specified; pass at least one flag")
		exitFn(1)
	}

	payload := map[string]interface{}{"todo-item": todo}
	if _, err := client.Put("/tasks/"+args[0]+".json", nil, payload); err != nil {
		exitOnError(err)
	}
	fmt.Printf("Task %s updated.\n", args[0])
}

func runTasksDelete(cmd *cobra.Command, args []string) {
	client := getClient()
	skip, _ := cmd.Flags().GetBool("yes")
	if !skip && !confirm(fmt.Sprintf("Delete task %s?", args[0])) {
		fmt.Println("Aborted.")
		return
	}
	if _, err := client.Delete("/tasks/"+args[0]+".json", nil); err != nil {
		exitOnError(err)
	}
	fmt.Printf("Task %s deleted.\n", args[0])
}

func runTasksSubtasks(cmd *cobra.Command, args []string) {
	client := getClient()
	add, _ := cmd.Flags().GetString("add")
	if add != "" {
		// Split on newline or literal ~|~; one POST per subtask.
		raw := strings.ReplaceAll(add, "~|~", "\n")
		var names []string
		for _, line := range strings.Split(raw, "\n") {
			if s := strings.TrimSpace(line); s != "" {
				names = append(names, s)
			}
		}
		if len(names) == 0 {
			fmt.Fprintln(os.Stderr, "Error: --add produced no non-empty subtask names")
			exitFn(1)
		}
		path := "/projects/api/v3/tasks/" + args[0] + "/subtasks.json"
		for _, name := range names {
			body := map[string]interface{}{
				"task": map[string]interface{}{"name": name},
			}
			if _, err := client.Post(path, nil, body); err != nil {
				exitOnError(err)
			}
		}
		fmt.Printf("Added %d subtask(s).\n", len(names))
		return
	}

	data, err := client.Get("/tasks/"+args[0]+"/subtasks.json", nil)
	if err != nil {
		exitOnError(err)
	}
	// Reuse v1 show-style raw print — these endpoints are v1 only.
	if string(getOutputMode()) == "json" {
		fmt.Println(string(data))
		return
	}
	var resp struct {
		TodoItems []struct {
			ID      json.Number `json:"id"`
			Content string      `json:"content"`
			Done    bool        `json:"completed"`
			Due     string      `json:"due-date"`
		} `json:"todo-items"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		fmt.Println(string(data))
		return
	}
	if len(resp.TodoItems) == 0 {
		fmt.Println("No subtasks.")
		return
	}
	for _, t := range resp.TodoItems {
		check := "[ ]"
		if t.Done {
			check = "[x]"
		}
		due := ""
		if t.Due != "" {
			due = " (due " + formatDate(t.Due) + ")"
		}
		fmt.Printf("%s %s · %s%s\n", check, t.ID.String(), t.Content, due)
	}
}

// confirm reads a yes/no from stdin. Returns true on yes/y.
func confirm(prompt string) bool {
	fmt.Printf("%s [y/N]: ", prompt)
	reader := bufio.NewReader(os.Stdin)
	resp, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	resp = strings.TrimSpace(strings.ToLower(resp))
	return resp == "y" || resp == "yes"
}
