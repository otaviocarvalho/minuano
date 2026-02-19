package main

import (
	"fmt"
	"strings"

	"github.com/otavio/minuano/internal/db"
	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Print task spec + full context log",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := connectDB(); err != nil {
			return err
		}

		task, ctxs, err := db.GetTaskWithContext(pool, args[0])
		if err != nil {
			return err
		}

		// Header.
		fmt.Printf("── Task: %s %s\n", task.ID, strings.Repeat("─", max(0, 60-len(task.ID))))
		fmt.Printf("Title:    %s\n", task.Title)
		fmt.Printf("Status:   %s %s", statusSymbol(task.Status), task.Status)
		if task.Status == "claimed" || task.Status == "ready" {
			fmt.Printf(" (attempt %d/%d)", task.Attempt, task.MaxAttempts)
		}
		fmt.Println()
		fmt.Printf("Priority: %d\n", task.Priority)
		if task.Capability != nil {
			fmt.Printf("Capability: %s\n", *task.Capability)
		}
		if task.ClaimedBy != nil {
			fmt.Printf("Claimed by: %s\n", *task.ClaimedBy)
		}
		if task.ProjectID != nil {
			fmt.Printf("Project:  %s\n", *task.ProjectID)
		}

		// Body.
		if task.Body != "" {
			fmt.Println()
			fmt.Println("Body:")
			fmt.Println(task.Body)
		}

		// Context log.
		if len(ctxs) > 0 {
			fmt.Printf("\n── Context %s\n", strings.Repeat("─", 60))
			for _, c := range ctxs {
				ts := c.CreatedAt.Local().Format("15:04:05")
				agent := "—"
				if c.AgentID != nil {
					agent = *c.AgentID
				}
				kindLabel := strings.ToUpper(c.Kind)
				header := fmt.Sprintf("[%s] %s  (%s)", ts, kindLabel, agent)
				if c.SourceTask != nil {
					header += fmt.Sprintf("  from: %s", *c.SourceTask)
				}
				fmt.Println(header)
				// Indent content.
				for _, line := range strings.Split(c.Content, "\n") {
					fmt.Printf("  %s\n", line)
				}
				fmt.Println()
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(showCmd)
}
