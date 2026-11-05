package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerationPlanningFailureLeavesNoFiles(t *testing.T) {
	t.Chdir(t.TempDir())
	err := runGeneration(func() error {
		if err := saveTemplate("one.txt", []byte("one"), nil); err != nil {
			return err
		}
		return errors.New("planning failed")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if _, err := os.Stat("one.txt"); !os.IsNotExist(err) {
		t.Fatalf("file created on planning failure: %v", err)
	}
}

func TestGenerationRollbackPreservesExistingFiles(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("existing.txt", []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	err := runGeneration(func() error {
		if err := saveTemplate("new.txt", []byte("new"), nil); err != nil {
			return err
		}
		if err := saveTemplate("late.txt", []byte("planned"), nil); err != nil {
			return err
		}
		// Simulate another process creating a file between planning and commit.
		return os.WriteFile("late.txt", []byte("external"), 0644)
	})
	if err == nil {
		t.Fatal("expected write conflict")
	}
	if _, err := os.Stat("new.txt"); !os.IsNotExist(err) {
		t.Fatalf("new file not rolled back: %v", err)
	}
	for file, want := range map[string]string{"existing.txt": "original", "late.txt": "external"} {
		got, err := os.ReadFile(filepath.Clean(file))
		if err != nil || string(got) != want {
			t.Fatalf("%s changed: %q, %v", file, got, err)
		}
	}
}
