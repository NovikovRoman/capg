package main

import (
	"fmt"
	"path/filepath"
)

func createConnection() error {
	if fileNotExists(dbDir) {
		if err := createDir(dbDir); err != nil {
			return err
		}
	}

	fileUtils := filepath.Join(dbDir, "utils.go")
	if fileNotExists(fileUtils) {
		if err := save(fileUtils, "templates/db.utils.tmpl", nil); err != nil {
			return err
		}
	}

	connFile := filepath.Join(dbDir, "connecton.go")
	if !fileNotExists(connFile) {
		fmt.Println("File already exists:", connFile)
		return nil
	}

	data := struct {
		ModulePath string
	}{
		ModulePath: modulePath,
	}
	err := save(connFile, "templates/connection.tmpl", data)
	if err != nil {
		return err
	}

	fileHelper := filepath.Join(dbDir, "helpers.go")
	if fileNotExists(fileHelper) {
		if err := save(fileHelper, "templates/helpers.tmpl", nil); err != nil {
			return err
		}
	}

	if fileNotExists("migrations") {
		err = createMigrate()
	}
	return err
}

func createRepo(ent entity) error {
	if ent.empty {
		return nil
	}

	if err := createDir(repoDir); err != nil {
		return err
	}

	data := struct {
		ModulePath  string
		EntityName  string
		EntityTable string
		EntitySymb  string
	}{
		ModulePath:  modulePath,
		EntityName:  ent.firstUpperName,
		EntityTable: ent.tableName,
		EntitySymb:  ent.symb,
	}

	tmpl := "templates/repository.tmpl"
	if ent.simple {
		tmpl = "templates/repository.simple.tmpl"
	}
	return save(filepath.Join(repoDir, ent.snakeName+".go"), tmpl, data)
}
