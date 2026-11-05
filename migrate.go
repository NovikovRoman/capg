package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const migrationTimeFormat = "200601021504"

func migrateCmd() *cobra.Command {
	return &cobra.Command{
		Use: "migrate", Aliases: []string{"m"}, Short: "Create migrate", Args: cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error { return runCommandGeneration(cmd, createMigrate) },
	}
}

func migrationCmd() *cobra.Command {
	return &cobra.Command{
		Use: "migration [name]", Short: "Create an empty SQL migration", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !regexp.MustCompile(`^[a-z][a-z0-9_]*$`).MatchString(args[0]) || args[0] == "types" || args[0] == "users" {
				return fmt.Errorf("invalid migration name %q: use lowercase snake_case (types and users are reserved)", args[0])
			}
			return runCommandGeneration(cmd, func() error {
				now := time.Now()
				if err := createMigrateAt(now); err != nil {
					return err
				}
				version, err := nextMigrationVersion(now)
				if err != nil {
					return err
				}
				for _, direction := range []string{"up", "down"} {
					file := filepath.Join(migrationDir(), version+"_"+args[0]+"."+direction+".sql")
					if err := save(file, "templates/migrate.empty.tmpl", nil); err != nil {
						return err
					}
				}
				return nil
			})
		},
	}
}

func migrationDir() string { return filepath.Join("migrations", db, "migrations") }

// migrationClock represents local wall-clock components without timezone transitions.
func migrationClock(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), 0, 0, time.UTC)
}

func createMigrate() error { return createMigrateAt(time.Now()) }

func createMigrateAt(now time.Time) error {
	path := filepath.Join("migrations", db)
	dir := migrationDir()
	if err := createDir(dir); err != nil {
		return err
	}
	entries, err := migrationFiles(dir)
	if err != nil {
		return err
	}
	// Reject incompatible existing versions before planning any scaffolding.
	if _, err := nextMigrationVersion(now); err != nil {
		return err
	}

	var version, versionFile string
	for _, entry := range entries {
		for _, direction := range []string{"up", "down"} {
			suffix := "_types." + direction + ".sql"
			if !strings.HasSuffix(entry, suffix) {
				continue
			}
			v := strings.TrimSuffix(entry, suffix)
			if strings.Contains(v, "_") {
				continue
			}
			if v == "" || strings.Trim(v, "0123456789") != "" {
				return fmt.Errorf("invalid types migration version %q: expected a numeric version", v)
			}
			if _, err := strconv.ParseUint(v, 10, 64); err != nil {
				return fmt.Errorf("invalid types migration version %q: %w", v, err)
			}
			if version != "" && version != v {
				return fmt.Errorf("multiple versions of types migration: %s (version %s) and %s (version %s); inspect both migration pairs and reconcile their history before retrying", filepath.Join(dir, versionFile), version, filepath.Join(dir, entry), v)
			}
			version, versionFile = v, entry
		}
	}
	if version == "" {
		// Earlier releases created users one minute after types; recover the same
		// order if only the original users migration remains.
		for _, entry := range entries {
			for _, direction := range []string{"up", "down"} {
				suffix := "_users." + direction + ".sql"
				if !strings.HasSuffix(entry, suffix) {
					continue
				}
				v := strings.TrimSuffix(entry, suffix)
				if strings.Contains(v, "_") {
					continue
				}
				at, err := time.Parse(migrationTimeFormat, v)
				if err != nil {
					// Other numeric conventions are handled by the shared version selector.
					continue
				}
				candidate := at.Add(-time.Minute).Format(migrationTimeFormat)
				if version != "" && version != candidate {
					return fmt.Errorf("multiple versions of users migration: %s and %s; inspect both migration pairs and reconcile their history before retrying", filepath.Join(dir, versionFile), filepath.Join(dir, entry))
				}
				version, versionFile = candidate, entry
			}
		}
	}
	if version == "" {
		version, err = nextMigrationVersion(migrationClock(now).Add(-time.Minute))
		if err != nil {
			return err
		}
	}
	typesVersion, _ := strconv.ParseUint(version, 10, 64)
	for _, entry := range entries {
		if !strings.HasSuffix(entry, ".up.sql") && !strings.HasSuffix(entry, ".down.sql") {
			continue
		}
		part, _, ok := strings.Cut(entry, "_")
		if !ok {
			continue
		}
		n, err := strconv.ParseUint(part, 10, 64)
		if err == nil && n == typesVersion && entry != version+"_types.up.sql" && entry != version+"_types.down.sql" {
			return fmt.Errorf("migration version %s is already used by %s; inspect the migration history and resolve the version conflict before retrying", version, filepath.Join(dir, entry))
		}
	}
	if err := saveIfMissing(filepath.Join(path, "migrate.go"), "templates/migrate.tmpl", nil); err != nil {
		return err
	}
	for _, direction := range []string{"up", "down"} {
		if err := saveIfMissing(filepath.Join(dir, version+"_types."+direction+".sql"), "templates/migrate.types."+direction+".tmpl", nil); err != nil {
			return err
		}
	}
	return nil
}

func checkEntityMigration(ent entity) error {
	if ent.empty {
		return nil
	}
	entries, err := migrationFiles(migrationDir())
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry
		if strings.HasSuffix(name, "_create_"+ent.tableName+".up.sql") || strings.HasSuffix(name, "_create_"+ent.tableName+".down.sql") ||
			(ent.tableName == "users" && (strings.HasSuffix(name, "_users.up.sql") || strings.HasSuffix(name, "_users.down.sql"))) {
			return fmt.Errorf("migration for table %s already exists: %s; review it before generating another or choose a different table with --table", ent.tableName, filepath.Join(migrationDir(), name))
		}
	}
	return nil
}

func nextMigrationVersion(now time.Time) (string, error) {
	entries, err := migrationFiles(migrationDir())
	if err != nil {
		return "", err
	}
	version := migrationClock(now).Format(migrationTimeFormat)
	candidate, _ := strconv.ParseUint(version, 10, 64)
	var maxVersion uint64
	var maxFile string
	for _, name := range entries {
		if !strings.HasSuffix(name, ".up.sql") && !strings.HasSuffix(name, ".down.sql") {
			continue
		}
		part, _, ok := strings.Cut(name, "_")
		if !ok || part == "" || strings.Trim(part, "0123456789") != "" {
			return "", fmt.Errorf("invalid migration filename %q: version must be numeric and followed by an underscore; inspect the file and migration history before correcting its name", filepath.Join(migrationDir(), name))
		}
		n, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return "", fmt.Errorf("invalid numeric migration version %q in %q: %w; inspect the file and migration history before correcting its version", part, filepath.Join(migrationDir(), name), err)
		}
		if maxFile == "" || n > maxVersion {
			maxVersion, maxFile = n, name
		}
	}
	if maxFile != "" && maxVersion >= candidate {
		maxText := strconv.FormatUint(maxVersion, 10)
		last, err := time.Parse(migrationTimeFormat, maxText)
		if err != nil {
			return "", fmt.Errorf("migration %q has version %s, which cannot be followed by an automatic YYYYMMDDHHMM version; inspect the migration history and use a separate versioning strategy before retrying; do not rename applied migrations without a database migration plan", filepath.Join(migrationDir(), maxFile), maxText)
		}
		version = last.Add(time.Minute).Format(migrationTimeFormat)
		if next, err := strconv.ParseUint(version, 10, 64); err != nil || next <= maxVersion || len(version) != len(migrationTimeFormat) {
			return "", fmt.Errorf("cannot generate a later YYYYMMDDHHMM version after migration %q (version %s); inspect the migration history and use a separate versioning strategy", filepath.Join(migrationDir(), maxFile), maxText)
		}
	}
	return version, nil
}

func createEntityMigration(ent entity, now time.Time) error {
	if ent.empty {
		return nil
	}
	if err := checkEntityMigration(ent); err != nil {
		return err
	}
	dir := migrationDir()
	version, err := nextMigrationVersion(now)
	if err != nil {
		return err
	}
	data := struct {
		DatabaseTableName string
		Simple            bool
	}{ent.tableName, ent.simple}
	for _, direction := range []string{"up", "down"} {
		filename := version + "_create_" + ent.tableName + "." + direction + ".sql"
		if err := save(filepath.Join(dir, filename), "templates/migrate.entity."+direction+".tmpl", data); err != nil {
			return err
		}
	}
	return nil
}
