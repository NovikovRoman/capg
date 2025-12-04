package main

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"text/template"

	"github.com/spf13/cobra"
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
	var err error
	if _, err = os.Stat("go.mod"); os.IsNotExist(err) {
		fmt.Println("go.mod not found.")
		os.Exit(1)
	}

	var b []byte
	if b, err = os.ReadFile("go.mod"); err != nil {
		fmt.Println("go.mod cannot be read.")
		os.Exit(1)
	}

	m := regexp.MustCompile(`(?si)module\s+(.+?)\s`).FindSubmatch(b)
	if len(m) == 0 {
		fmt.Println("Module not found in go.mod.")
		os.Exit(1)
	}

	rootCmd := &cobra.Command{
		Use:   "capg [entity|struct|array|migrate]",
		Short: "",
	}

	modulePath = string(m[1])
	migrateCmd := migrateCmd()
	entityCmd := entityCmd()
	jsonArrayCmd := jsonArrayCmd()
	jsonStructCmd := jsonStructCmd()

	rootCmd.AddCommand(migrateCmd, entityCmd, jsonArrayCmd, jsonStructCmd)

	if err = rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// createDir creates a directory if it does not exist.
func createDir(dir string) (err error) {
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

// saveTemplate creates a file from a template.
func saveTemplate(filename string, tmpl []byte, data any) error {
	var f *os.File
	if _, err := os.Stat(filename); err == nil {
		return errors.New(filename + " file exists.")
	}
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()
	return template.Must(template.New("").Parse(string(tmpl))).Execute(f, data)
}
