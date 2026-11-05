package main

import (
	"path/filepath"

	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func jsonStructCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "struct [name]",
		Aliases: []string{"s"},
		Short:   "Create json struct",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateName(args[0]); err != nil {
				return err
			}
			return runCommandGeneration(cmd, func() error { return createJsonStruct(args[0]) })
		},
	}
}

func createJsonStruct(name string) (err error) {
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
	return save(filepath.Join(modelDir, toSnake(name)+".go"), "templates/json_struct.tmpl", data)
}
