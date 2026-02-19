package main

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/otavio/minuano/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"
)

var (
	dbURL       string
	sessionName string
	pool        *pgxpool.Pool
)

var rootCmd = &cobra.Command{
	Use:   "minuano",
	Short: "Agent task coordination via tmux + PostgreSQL",
	Long:  "Minuano coordinates Claude Code agents via tmux, using PostgreSQL as the coordination substrate.",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&dbURL, "db", "", "database URL (overrides DATABASE_URL)")
	rootCmd.PersistentFlags().StringVar(&sessionName, "session", "", "tmux session name (overrides MINUANO_SESSION)")
}

// connectDB initializes the database pool. Call from subcommands that need DB access.
func connectDB() error {
	url := dbURL
	if url == "" {
		url = os.Getenv("DATABASE_URL")
	}
	if url == "" {
		return fmt.Errorf("DATABASE_URL not set (use --db flag or .env)")
	}

	var err error
	pool, err = db.Connect(url)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	return nil
}

// getSessionName returns the tmux session name from flag, env, or default.
func getSessionName() string {
	if sessionName != "" {
		return sessionName
	}
	if s := os.Getenv("MINUANO_SESSION"); s != "" {
		return s
	}
	return "minuano"
}

func main() {
	_ = godotenv.Load()

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}

	if pool != nil {
		pool.Close()
	}
}
