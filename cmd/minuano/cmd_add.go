package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/otavio/minuano/internal/db"
	"github.com/spf13/cobra"
)

var (
	addAfter      []string
	addPriority   int
	addCapability string
	addTestCmd    string
	addProject    string
	addBody       string
)

var addCmd = &cobra.Command{
	Use:   "add <title>",
	Short: "Create a task",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := connectDB(); err != nil {
			return err
		}

		title := strings.Join(args, " ")
		id := generateID(title)

		var capability *string
		if addCapability != "" {
			capability = &addCapability
		}

		projectID := addProject
		if projectID == "" {
			projectID = os.Getenv("MINUANO_PROJECT")
		}
		var projPtr *string
		if projectID != "" {
			projPtr = &projectID
		}

		var metadata json.RawMessage
		if addTestCmd != "" {
			m := map[string]string{"test_cmd": addTestCmd}
			metadata, _ = json.Marshal(m)
		}

		if err := db.CreateTask(pool, id, title, addBody, addPriority, capability, projPtr, metadata); err != nil {
			return err
		}

		// Add dependencies.
		for _, dep := range addAfter {
			resolvedDep, err := db.ResolvePartialID(pool, dep)
			if err != nil {
				return fmt.Errorf("resolving dependency %q: %w", dep, err)
			}
			if err := db.AddDependency(pool, id, resolvedDep); err != nil {
				return err
			}
		}

		// Set status based on deps.
		if len(addAfter) > 0 {
			hasUnmet, err := db.HasUnmetDeps(pool, id)
			if err != nil {
				return err
			}
			if !hasUnmet {
				if err := db.SetTaskStatus(pool, id, "ready"); err != nil {
					return err
				}
			}
			// Otherwise stays 'pending' (default).
		} else {
			if err := db.SetTaskStatus(pool, id, "ready"); err != nil {
				return err
			}
		}

		fmt.Printf("Created: %s  %q\n", id, title)
		return nil
	},
}

func init() {
	addCmd.Flags().StringSliceVar(&addAfter, "after", nil, "dependency task ID (partial ok, repeatable)")
	addCmd.Flags().IntVar(&addPriority, "priority", 5, "priority 0-10")
	addCmd.Flags().StringVar(&addCapability, "capability", "", "required agent capability")
	addCmd.Flags().StringVar(&addTestCmd, "test-cmd", "", "test command override")
	addCmd.Flags().StringVar(&addProject, "project", "", "project ID (or MINUANO_PROJECT env)")
	addCmd.Flags().StringVar(&addBody, "body", "", "task body/specification")
	rootCmd.AddCommand(addCmd)
}

// generateID creates a slug from the title plus a random suffix.
func generateID(title string) string {
	slug := slugify(title)
	if len(slug) > 15 {
		slug = slug[:15]
	}
	suffix := randomHex(3)
	return slug + "-" + suffix
}

func slugify(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevDash = false
		} else if !prevDash && b.Len() > 0 {
			b.WriteByte('-')
			prevDash = true
		}
	}
	return strings.TrimRight(b.String(), "-")
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)[:n+2]
}
