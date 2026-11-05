package main

import (
	"path/filepath"

	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func jsonArrayCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "array [name]",
		Aliases: []string{"a"},
		Short:   "Create json array",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateName(args[0]); err != nil {
				return err
			}
			return runCommandGeneration(cmd, func() error { return createJsonArray(args[0]) })
		},
	}
}

func createJsonArray(name string) (err error) {
	if err = createDir(modelDir); err != nil {
		return
	}

	data := struct {
		JSONTypeName string
		ReceiverName string
	}{
		JSONTypeName: cases.Title(language.English, cases.NoLower).String(name),
		ReceiverName: "item",
	}
	return save(filepath.Join(modelDir, toSnake(name)+".go"), "templates/json_array.tmpl", data)
}
