package main

import (
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateName(t *testing.T) {
	for _, tc := range []struct {
		name  string
		valid bool
	}{
		{"User", true}, {"orderItem", true}, {"Équipe", true},
		{"", false}, {"_", false}, {"_name", false}, {"1name", false},
		{"foo/bar", false}, {"foo-bar", false}, {"type", false}, {"日本", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateName(tc.name); (err == nil) != tc.valid {
				t.Fatalf("validateName(%q) = %v; valid = %v", tc.name, err, tc.valid)
			}
		})
	}
}

func TestCLIDiagnostics(t *testing.T) {
	root := t.TempDir()
	binary := filepath.Join(root, "capg")
	if out, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build generator: %v\n%s", err, out)
	}
	project := filepath.Join(root, "project")
	if err := os.Mkdir(project, 0755); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) (string, error) {
		t.Helper()
		cmd := exec.Command(binary, args...)
		cmd.Dir = project
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	check := func(args []string, parts ...string) {
		t.Helper()
		out, err := run(args...)
		if err == nil {
			t.Fatalf("capg %v: expected an error, got %q", args, out)
		}
		for _, part := range parts {
			if !strings.Contains(out, part) {
				t.Errorf("capg %v: missing %q in %q", args, part, out)
			}
		}
	}
	check([]string{"entity", "Product"}, "go.mod", "go mod init")
	if err := os.WriteFile(filepath.Join(project, "go.mod"), []byte("module example.com/test\n\ngo 1.25\n"), 0644); err != nil {
		t.Fatal(err)
	}
	check([]string{"entity", "bad-name"}, `"bad-name"`, "Product")
	check([]string{"entity", "Product", "--table", "bad-name"}, `"bad-name"`, "lowercase")
	check([]string{"migration", "BadName"}, `"BadName"`, "snake_case")
	if out, err := run("entity", "Product", "--empty"); err != nil {
		t.Fatalf("generate empty entity: %v\n%s", err, out)
	}
	for _, args := range [][]string{{"entity", "Product", "--empty"}, {"entity", "Product", "--empty", "--dry-run"}} {
		check(args, "internal/models/product.go", "already exists", "different name")
	}
	if out, err := run("entity", "Order"); err != nil {
		t.Fatalf("generate order: %v\n%s", err, out)
	}
	check([]string{"entity", "Order"}, "migrations/pg/migrations/", "_create_orders.", "--table")

	blocked := filepath.Join(project, "migrations", "pg", "migrations", "20260101010101_blocked.up.sql")
	if err := os.WriteFile(blocked, []byte("-- existing\n"), 0644); err != nil {
		t.Fatal(err)
	}
	check([]string{"migration", "add_field"}, "20260101010101_blocked.up.sql", "20260101010101", "do not rename applied migrations")
}

func TestGeneratedProject(t *testing.T) {
	root := t.TempDir()
	binary := filepath.Join(root, "capg")
	build := exec.Command("go", "build", "-o", binary, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build generator: %v\n%s", err, out)
	}
	project := filepath.Join(root, "project")
	for _, args := range [][]string{{"--help"}, {"entity", "--help"}, {"help", "entity"}} {
		cmd := exec.Command(binary, args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("help outside module %v: %v\n%s", args, err, out)
		}
	}
	cmd := exec.Command(binary, "entity", "Outside")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err == nil || !strings.Contains(string(out), "go.mod") {
		t.Fatalf("expected missing go.mod error: %v\n%s", err, out)
	}
	if err := os.Mkdir(project, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "go.mod"), []byte("// module example.com/wrong\nmodule \"example.com/demo\"\n\ngo 1.25\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) (string, error) {
		t.Helper()
		cmd := exec.Command(binary, args...)
		cmd.Dir = project
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	out, err := run("entity", "Preview", "--dry-run")
	if err != nil || !strings.Contains(out, "Would create: internal/models/preview.go") || !strings.Contains(out, "_create_previews.up.sql") {
		t.Fatalf("dry run failed: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(project, "internal")); !os.IsNotExist(err) {
		t.Fatalf("dry run created files or directories: %v", err)
	}
	for _, args := range [][]string{{"entity", "User"}, {"entity", "Widget", "-s"}, {"entity", "Empty", "-e"}, {"entity", "Record"}, {"entity", "Équipe"}, {"struct", "Value"}, {"array", "Values"}, {"migration", "add_product_price"}} {
		if out, err := run(args...); err != nil {
			t.Fatalf("capg %v: %v\n%s", args, err, out)
		}
	}
	for _, args := range [][]string{{"entity", "User"}, {"entity", ""}, {"array", ""}, {"struct", "foo-bar"}, {"migration", "../escape"}, {"migration", "types"}, {"entity", "Bad", "--table", "bad;drop"}, {"entity", "Bad", "--table", ""}, {"entity", "Bad", "--empty", "--table", "foo"}} {
		if out, err := run(args...); err == nil || strings.Contains(out, "panic:") {
			t.Errorf("capg %v: expected non-panic failure, got %v\n%s", args, err, out)
		}
	}
	// Existing connection files must not prevent recovery of missing shared files.
	helper := filepath.Join(project, "internal", "db", "helpers.go")
	if err := os.Remove(helper); err != nil {
		t.Fatal(err)
	}
	if out, err := run("entity", "Another"); err != nil {
		t.Fatalf("generate after partial setup: %v\n%s", err, out)
	}
	if _, err := os.Stat(helper); err != nil {
		t.Fatalf("shared helper not restored: %v", err)
	}
	helperContent, err := os.ReadFile(helper)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(helperContent), " DEFAULT VALUES") {
		t.Error("generated Create does not handle entities with no insertable fields")
	}
	for _, pattern := range []string{"*_types.up.sql", "*_create_users.up.sql"} {
		matches, err := filepath.Glob(filepath.Join(project, "migrations", "pg", "migrations", pattern))
		if err != nil || len(matches) != 1 {
			t.Fatalf("expected one migration matching %s, got %v: %v", pattern, matches, err)
		}
		file := matches[0]
		contents, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(strings.ToUpper(string(contents)), "IF NOT EXISTS") {
			t.Errorf("%s can silently adopt existing database objects", file)
		}
	}
	if err := filepath.WalkDir(project, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".go") {
			if _, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.AllErrors); err != nil {
				t.Errorf("generated Go syntax %s: %v", path, err)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	repo, err := os.ReadFile(filepath.Join(project, "internal", "repository", "user.go"))
	if err != nil || !strings.Contains(string(repo), `"example.com/demo/internal/models"`) {
		t.Fatalf("incorrect module path in generated import: %v\n%s", err, repo)
	}
	conn, err := os.ReadFile(filepath.Join(project, "internal", "db", "connection.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(conn), "MigrationUp") || strings.Contains(string(conn), "MigrationDown") {
		t.Error("connection must not apply migrations")
	}
	modelTest := `package models
import "testing"
func TestScanReset(t *testing.T) {
    for _, input := range []any{"{}", []byte("{}"), "[]", []byte("[]")} {
        v := Value{Field1: 42, Field2: "old", Field3: true}
        if err := v.Scan(input); err != nil || v != (Value{}) {
            t.Fatalf("Scan(%q) = %#v, %v", input, v, err)
        }
    }
}
func TestArrayScan(t *testing.T) {
    for _, input := range []any{"null", []byte("null")} {
        v := Values{"old"}
        if err := v.Scan(input); err != nil || v != nil {
            t.Fatalf("Scan(%q) = %#v, %v; want nil", input, v, err)
        }
    }
    for _, input := range []any{"[]", []byte("[]"), "[\"new\"]", []byte("[\"new\"]")} {
        v := Values{"old"}
        if err := v.Scan(input); err != nil { t.Fatalf("Scan(%q): %v", input, err) }
        if len(v) > 0 && v[0] != "new" { t.Fatalf("Scan(%q) = %#v", input, v) }
        if len(v) == 0 && v == nil { t.Fatalf("Scan(%q) = nil; want empty array", input) }
    }
    v := Values{"old"}
    if err := v.Scan("[\"new\", 42]"); err == nil || len(v) != 1 || v[0] != "old" {
        t.Fatalf("invalid Scan changed value: %#v, %v", v, err)
    }
}
`
	if err := os.WriteFile(filepath.Join(project, "internal", "models", "generated_test.go"), []byte(modelTest), 0644); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("go", "test", "./internal/models")
	cmd.Dir = project
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("test generated models: %v\n%s", err, out)
	}
}
