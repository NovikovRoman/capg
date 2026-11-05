package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func entityCmd() *cobra.Command {
	var (
		empty  bool
		simple bool
		table  string
	)
	entityCmd := &cobra.Command{
		Use:     "entity [name]",
		Aliases: []string{"e"},
		Short:   "Create entity",
		Args:    cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if empty && simple {
				return fmt.Errorf("flags --simple and --empty cannot be used together; choose one or omit both")
			}
			if cmd.Flags().Changed("table") {
				if empty {
					return fmt.Errorf("--table cannot be used with --empty; remove --table or generate an entity with a table")
				}
				if !regexp.MustCompile(`^[a-z_][a-z0-9_]*$`).MatchString(table) || len(table) > 63 {
					return fmt.Errorf("invalid table name %q: use 1-63 lowercase ASCII letters, digits or underscores, starting with a letter or underscore", table)
				}
			}
			return validateName(args[0])
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommandGeneration(cmd, func() error {
				ent := newEntity(args[0], empty, simple)
				if cmd.Flags().Changed("table") {
					ent.tableName = table
				}
				if len(ent.tableName) > 63 {
					return fmt.Errorf("generated table name %q exceeds PostgreSQL's 63-byte limit; use --table with a shorter name", ent.tableName)
				}
				if err := checkEntityMigration(ent); err != nil {
					return err
				}
				if err := ent.create(); err != nil {
					return err
				}
				if err := createConnection(); err != nil {
					return err
				}
				if err := createRepo(ent); err != nil {
					return err
				}
				return createEntityMigration(ent, time.Now())
			})
		},
	}

	entityCmd.Flags().BoolVarP(&empty, "empty", "e", false, "Create empty entity")
	entityCmd.Flags().BoolVarP(&simple, "simple", "s", false, "Create simple entity")
	entityCmd.Flags().StringVar(&table, "table", "", "Override PostgreSQL table name")
	return entityCmd
}

type entity struct {
	firstLowerName string
	firstUpperName string
	packageName    string
	snakeName      string
	tableName      string
	nameRunes      []rune
	symb           string

	empty  bool
	simple bool
}

func newEntity(name string, empty, simple bool) entity {
	ent := entity{
		nameRunes: []rune(name),
		snakeName: toSnake(name),
		empty:     empty,
		simple:    simple,
	}
	ent.symb = "item"
	ent.firstLowerName = strings.ToLower(string(ent.nameRunes[0])) + string(ent.nameRunes[1:])
	ent.firstUpperName = cases.Title(language.English, cases.NoLower).String(ent.firstLowerName)
	ent.packageName = strings.ReplaceAll(ent.snakeName, "_", "")

	ent.tableName = ent.snakeName
	nameRunes := []rune(ent.tableName)
	switch nameRunes[len(nameRunes)-1] {
	case 's', 'x', 'h':
		ent.tableName += "es"

	case 'y':
		ent.tableName = string(nameRunes[:len(nameRunes)-1]) + "ies"

	default:
		ent.tableName += "s"
	}
	return ent
}

func (ent *entity) create() error {
	tmplEntity := "entity"
	if ent.empty {
		tmplEntity = "entity.empty"

	} else if ent.simple {
		tmplEntity = "entity.simple"
	}

	if err := createDir(modelDir); err != nil {
		return err
	}
	data := struct {
		ModelTypeName string
		ReceiverName  string
	}{
		ModelTypeName: ent.firstUpperName,
		ReceiverName:  ent.symb,
	}
	return save(filepath.Join(modelDir, ent.snakeName+".go"), "templates/"+tmplEntity+".tmpl", data)
}
