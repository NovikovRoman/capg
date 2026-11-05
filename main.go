package main

import (
	"embed"
	"fmt"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"unicode"
	"unicode/utf8"

	"github.com/spf13/cobra"
	"golang.org/x/mod/modfile"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const (
	permDir         = 0755
	db       string = "pg"
	modelDir        = "internal/models"
	dbDir           = "internal/db"
	repoDir         = "internal/repository"
)

//go:embed templates/*
var embeddedFiles embed.FS

var (
	modulePath string
)

func main() {
	rootCmd := &cobra.Command{
		Use:           "capg [command]",
		Short:         "Generate Go models, repositories, and PostgreSQL migrations",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	rootCmd.PersistentFlags().Bool("dry-run", false, "Show files that would be created without writing them")

	migrateCmd := migrateCmd()
	entityCmd := entityCmd()
	jsonArrayCmd := jsonArrayCmd()
	jsonStructCmd := jsonStructCmd()

	rootCmd.AddCommand(migrateCmd, migrationCmd(), entityCmd, jsonArrayCmd, jsonStructCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func loadModulePath() (string, error) {
	b, err := os.ReadFile("go.mod")
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("read go.mod in current directory: %w; run capg from the root of an existing Go module or initialize one with go mod init", err)
		}
		return "", fmt.Errorf("read go.mod in current directory: %w; check access to go.mod", err)
	}
	mod, err := modfile.Parse("go.mod", b, nil)
	if err != nil {
		return "", fmt.Errorf("parse go.mod: %w", err)
	}
	if mod.Module == nil || mod.Module.Mod.Path == "" {
		return "", fmt.Errorf("go.mod has no module directive")
	}
	return mod.Module.Mod.Path, nil
}

func validateName(name string) error {
	if !token.IsIdentifier(name) || name == "_" || name[0] == '_' {
		return fmt.Errorf("invalid name %q: expected a Go identifier beginning with a letter; for example, Product or orderItem", name)
	}
	exported := cases.Title(language.English, cases.NoLower).String(name)
	first, _ := utf8.DecodeRuneInString(exported)
	if !token.IsIdentifier(exported) || !unicode.IsUpper(first) {
		return fmt.Errorf("invalid name %q: cannot form an exported Go identifier; choose a name such as Product or orderItem", name)
	}
	return nil
}

// createDir creates a directory if it does not exist.
func createDir(dir string) (err error) {
	if activePlan != nil {
		return nil
	}
	if fileNotExists(dir) {
		err = os.MkdirAll(dir, permDir)
	}
	return
}

func fileNotExists(filepath string) bool {
	_, err := os.Stat(filepath)
	return os.IsNotExist(err)
}

func save(filename, tmpl string, data any) error {
	b, err := fs.ReadFile(embeddedFiles, tmpl)
	if err != nil {
		return err
	}
	return saveTemplate(filename, b, data)
}

func saveIfMissing(filename, tmpl string, data any) error {
	exists, err := pathExists(filename)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return save(filename, tmpl, data)
}

// saveTemplate renders and plans a new file without overwriting existing files.
func saveTemplate(filename string, tmpl []byte, data any) error {
	exists, err := pathExists(filename)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("cannot create %s: path already exists; review the existing path and choose a different name, or move it only if it is safe to do so", filename)
	}
	b, err := renderTemplate(filename, tmpl, data)
	if err != nil {
		return err
	}
	if activePlan == nil {
		return writeNewFile(filename, b)
	}
	filename = filepath.Clean(filename)
	activePlan.files[filename] = b
	activePlan.order = append(activePlan.order, filename)
	return nil
}
