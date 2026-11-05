package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/format"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
)

type generationPlan struct {
	files map[string][]byte
	order []string
}

// activePlan is scoped to a single CLI command; commands execute sequentially.
var activePlan *generationPlan

func runGeneration(fn func() error) error {
	return generate(fn, false, io.Discard)
}

func runCommandGeneration(cmd *cobra.Command, fn func() error) error {
	var err error
	modulePath, err = loadModulePath()
	if err != nil {
		return err
	}
	dryRun, err := cmd.Flags().GetBool("dry-run")
	if err != nil {
		return err
	}
	return generate(fn, dryRun, cmd.OutOrStdout())
}

func generate(fn func() error, dryRun bool, output io.Writer) error {
	plan := &generationPlan{files: make(map[string][]byte)}
	activePlan = plan
	defer func() { activePlan = nil }()
	if err := fn(); err != nil {
		return err
	}
	if !dryRun {
		if err := plan.commit(); err != nil {
			return err
		}
	}
	if len(plan.order) == 0 {
		fmt.Fprintln(output, "No files to create.")
		return nil
	}
	label := "Created"
	if dryRun {
		label = "Would create"
	}
	for _, filename := range plan.order {
		fmt.Fprintf(output, "%s: %s\n", label, filename)
	}
	fmt.Fprintln(output, "Next: review generated files and SQL; run go mod tidy if dependencies are missing.")
	return nil
}

func pathExists(filename string) (bool, error) {
	filename = filepath.Clean(filename)
	if activePlan != nil {
		if _, ok := activePlan.files[filename]; ok {
			return true, nil
		}
	}
	_, err := os.Lstat(filename)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect path %s: %w; check the path and permissions", filename, err)
	}
	return true, nil
}

func renderTemplate(filename string, source []byte, data any) ([]byte, error) {
	tmpl, err := template.New(filename).Option("missingkey=error").Parse(string(source))
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", filename, err)
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		return nil, fmt.Errorf("render %s: %w", filename, err)
	}
	b := out.Bytes()
	if strings.HasSuffix(filename, ".go") {
		b, err = format.Source(b)
		if err != nil {
			return nil, fmt.Errorf("format %s: %w", filename, err)
		}
	}
	return b, nil
}

func writeNewFile(filename string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(filename), permDir); err != nil {
		return err
	}
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	n, writeErr := f.Write(content)
	if writeErr == nil && n != len(content) {
		writeErr = io.ErrShortWrite
	}
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		removeErr := os.Remove(filename)
		return errors.Join(writeErr, closeErr, removeErr)
	}
	return nil
}

func (p *generationPlan) commit() error {
	created := make([]string, 0, len(p.order))
	for _, filename := range p.order {
		if err := writeNewFile(filename, p.files[filename]); err != nil {
			var rollbackErrors []string
			for i := len(created) - 1; i >= 0; i-- {
				if e := os.Remove(created[i]); e != nil {
					rollbackErrors = append(rollbackErrors, e.Error())
				}
			}
			if len(rollbackErrors) > 0 {
				return fmt.Errorf("write %s: %w; rollback failed: %s", filename, err, strings.Join(rollbackErrors, "; "))
			}
			if errors.Is(err, os.ErrExist) {
				return fmt.Errorf("cannot create %s: %w; the path appeared during generation; review it and retry with a different name if needed", filename, err)
			}
			return fmt.Errorf("write %s: %w; check the path and permissions before retrying", filename, err)
		}
		created = append(created, filename)
	}
	return nil
}

func migrationFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read migration directory %s: %w; check the path and permissions", dir, err)
	}
	names := make(map[string]bool)
	for _, entry := range entries {
		if !entry.IsDir() {
			names[entry.Name()] = true
		}
	}
	if activePlan != nil {
		for filename := range activePlan.files {
			if filepath.Dir(filename) == filepath.Clean(dir) {
				names[filepath.Base(filename)] = true
			}
		}
	}
	files := make([]string, 0, len(names))
	for name := range names {
		files = append(files, name)
	}
	sort.Strings(files)
	return files, nil
}
