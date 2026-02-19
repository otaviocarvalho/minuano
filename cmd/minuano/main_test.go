package main

import (
	"os"
	"testing"
)

func TestRootCommandHelp(t *testing.T) {
	rootCmd.SetArgs([]string{"--help"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("root --help failed: %v", err)
	}
}

func TestPersistentFlags(t *testing.T) {
	f := rootCmd.PersistentFlags()

	if f.Lookup("db") == nil {
		t.Error("expected --db persistent flag to be registered")
	}
	if f.Lookup("session") == nil {
		t.Error("expected --session persistent flag to be registered")
	}
}

func TestGetSessionName_Default(t *testing.T) {
	// Clear both flag and env
	sessionName = ""
	os.Unsetenv("MINUANO_SESSION")

	got := getSessionName()
	if got != "minuano" {
		t.Errorf("getSessionName() = %q, want %q", got, "minuano")
	}
}

func TestGetSessionName_Env(t *testing.T) {
	sessionName = ""
	os.Setenv("MINUANO_SESSION", "test-session")
	defer os.Unsetenv("MINUANO_SESSION")

	got := getSessionName()
	if got != "test-session" {
		t.Errorf("getSessionName() = %q, want %q", got, "test-session")
	}
}

func TestGetSessionName_FlagOverridesEnv(t *testing.T) {
	sessionName = "flag-session"
	os.Setenv("MINUANO_SESSION", "env-session")
	defer func() {
		sessionName = ""
		os.Unsetenv("MINUANO_SESSION")
	}()

	got := getSessionName()
	if got != "flag-session" {
		t.Errorf("getSessionName() = %q, want %q", got, "flag-session")
	}
}

func TestConnectDB_NoURL(t *testing.T) {
	dbURL = ""
	os.Unsetenv("DATABASE_URL")

	err := connectDB()
	if err == nil {
		t.Fatal("connectDB() should fail when no DATABASE_URL is set")
	}
}
