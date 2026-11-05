package main

import "path/filepath"

func createConnection() error {
	if err := createDir(dbDir); err != nil {
		return err
	}
	if err := saveIfMissing(filepath.Join(dbDir, "utils.go"), "templates/db.utils.tmpl", nil); err != nil {
		return err
	}
	if err := saveIfMissing(filepath.Join(dbDir, "connection.go"), "templates/connection.tmpl", nil); err != nil {
		return err
	}
	if err := saveIfMissing(filepath.Join(dbDir, "helpers.go"), "templates/helpers.tmpl", nil); err != nil {
		return err
	}
	return createMigrate()
}

func createRepo(ent entity) error {
	if ent.empty {
		return nil
	}

	if err := createDir(repoDir); err != nil {
		return err
	}

	data := struct {
		GoModulePath      string
		ModelTypeName     string
		DatabaseTableName string
		ReceiverName      string
	}{
		GoModulePath:      modulePath,
		ModelTypeName:     ent.firstUpperName,
		DatabaseTableName: ent.tableName,
		ReceiverName:      ent.symb,
	}

	tmpl := "templates/repository.tmpl"
	if ent.simple {
		tmpl = "templates/repository.simple.tmpl"
	}
	return save(filepath.Join(repoDir, ent.snakeName+".go"), tmpl, data)
}
