package main

import (
	"testing"
)

func TestShowCommandRegistered(t *testing.T) {
	for _, c := range rootCmd.Commands() {
		if c.Use == "show <id>" {
			return
		}
	}
	t.Error("expected 'show' command to be registered")
}

func TestShowRequiresArg(t *testing.T) {
	rootCmd.SetArgs([]string{"show"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected show to fail without argument")
	}
}
