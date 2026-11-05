package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMigrationVersions(t *testing.T) {
	t.Chdir(t.TempDir())
	now := time.Date(2026, 1, 1, 0, 0, 59, 0, time.FixedZone("local", 3*60*60))
	if err := createMigrateAt(now); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join("migrations", "pg", "migrations")
	for _, name := range []string{"202512312359_types.up.sql", "202512312359_types.down.sql"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("expected %s: %v", name, err)
		}
	}
	if err := os.Remove(filepath.Join(dir, "202512312359_types.down.sql")); err != nil {
		t.Fatal(err)
	}
	if err := createMigrateAt(now.Add(2 * time.Hour)); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 2 {
		t.Fatalf("expected two migration files after recovery, got %d: %v", len(entries), err)
	}
	if _, err := os.Stat(filepath.Join(dir, "202512312359_types.down.sql")); err != nil {
		t.Fatal(err)
	}
}

func TestMigrationPartialAndLegacyVersions(t *testing.T) {
	for _, tc := range []struct {
		name, existing, expected string
	}{
		{"users only", "202601010000_users.up.sql", "202512312359_types.up.sql"},
		{"types only", "202512312359_types.down.sql", "202512312359_types.up.sql"},
		{"numeric types", "1_types.up.sql", "1_types.down.sql"},
		{"legacy", "202505201000_users.up.sql", "202505200959_types.up.sql"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			dir := filepath.Join("migrations", "pg", "migrations")
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, tc.existing), []byte("keep"), 0644); err != nil {
				t.Fatal(err)
			}
			if err := createMigrateAt(time.Now()); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(dir, tc.expected)); err != nil {
				t.Fatal(err)
			}
			content, err := os.ReadFile(filepath.Join(dir, tc.existing))
			if err != nil || string(content) != "keep" {
				t.Fatalf("existing migration changed: %q, %v", content, err)
			}
		})
	}
}

func TestEntityMigrations(t *testing.T) {
	t.Chdir(t.TempDir())
	now := time.Date(2026, 1, 1, 0, 0, 42, 0, time.UTC)
	if err := createMigrateAt(now); err != nil {
		t.Fatal(err)
	}
	for _, ent := range []entity{newEntity("User", false, false), newEntity("Widget", false, true), newEntity("Empty", true, false), newEntity("Type", false, false)} {
		if err := createEntityMigration(ent, now); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct{ name, contains, absent string }{
		{"202601010000_create_users.up.sql", `"created_at" TIMESTAMP NOT NULL`, ""},
		{"202601010000_create_users.down.sql", `DROP TABLE "users"`, ""},
		{"202601010001_create_widgets.up.sql", `"id" BIGINT`, "created_at"},
		{"202601010002_create_types.up.sql", `CREATE TABLE "types"`, ""},
	} {
		b, err := os.ReadFile(filepath.Join(migrationDir(), tc.name))
		if err != nil || !strings.Contains(string(b), tc.contains) || (tc.absent != "" && strings.Contains(string(b), tc.absent)) {
			t.Errorf("unexpected migration %s: %s (%v)", tc.name, b, err)
		}
	}
	if matches, _ := filepath.Glob(filepath.Join(migrationDir(), "*_create_empties.*.sql")); len(matches) != 0 {
		t.Fatalf("empty entity created migrations: %v", matches)
	}
	if err := createMigrateAt(now.Add(time.Hour)); err != nil {
		t.Fatalf("types entity must not be mistaken for initial types migration: %v", err)
	}
	if err := createEntityMigration(newEntity("User", false, false), now); err == nil {
		t.Fatal("duplicate table migration should fail")
	}
}

func TestLegacyUsersMigrationConflict(t *testing.T) {
	t.Chdir(t.TempDir())
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := createMigrateAt(now); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(migrationDir(), "202601010000_users.up.sql")
	if err := os.WriteFile(file, []byte("legacy"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := createEntityMigration(newEntity("User", false, false), now); err == nil {
		t.Fatal("expected legacy users conflict")
	}
	b, err := os.ReadFile(file)
	if err != nil || string(b) != "legacy" {
		t.Fatalf("legacy migration changed: %q, %v", b, err)
	}
}

func TestMigrationVersionConflict(t *testing.T) {
	t.Chdir(t.TempDir())
	dir := filepath.Join("migrations", "pg", "migrations")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "202512312359_other.up.sql")
	if err := os.WriteFile(file, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	err := createMigrateAt(time.Date(2026, 1, 1, 0, 0, 5, 0, time.UTC))
	if err != nil {
		t.Fatalf("select later version: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "202601010000_types.up.sql")); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(file)
	if err != nil || string(content) != "keep" {
		t.Fatalf("existing file changed: %q, %v", content, err)
	}
}

func TestNextMigrationVersionFormats(t *testing.T) {
	for _, tc := range []struct {
		name          string
		files         []string
		want, failure string
	}{
		{"sequential", []string{"1_first.up.sql", "002_next.down.sql"}, "202601010000", ""},
		{"later minute", []string{"202701012359_later.up.sql"}, "202701020000", ""},
		{"seconds", []string{"20260101000000_legacy.up.sql"}, "", "20260101000000_legacy.up.sql"},
		{"not numeric", []string{"abc_bad.up.sql"}, "", "abc_bad.up.sql"},
		{"missing separator", []string{"123.up.sql"}, "", "123.up.sql"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if err := os.MkdirAll(migrationDir(), 0755); err != nil {
				t.Fatal(err)
			}
			for _, name := range tc.files {
				if err := os.WriteFile(filepath.Join(migrationDir(), name), []byte("keep"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			got, err := nextMigrationVersion(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
			if tc.failure != "" {
				if err == nil || !strings.Contains(err.Error(), tc.failure) {
					t.Fatalf("expected error mentioning %s, got %v", tc.failure, err)
				}
			} else if err != nil || got != tc.want {
				t.Fatalf("version %q, error %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestIncompatibleMigrationDoesNotWrite(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(migrationDir(), 0755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(migrationDir(), "20260101000000_old.up.sql")
	if err := os.WriteFile(file, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	err := runGeneration(func() error { return createMigrateAt(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) })
	if err == nil || !strings.Contains(err.Error(), filepath.Base(file)) {
		t.Fatalf("expected incompatible version error, got %v", err)
	}
	entries, err := os.ReadDir(migrationDir())
	if err != nil || len(entries) != 1 {
		t.Fatalf("unexpected files after failure: %v, %v", entries, err)
	}
	contents, err := os.ReadFile(file)
	if err != nil || string(contents) != "keep" {
		t.Fatalf("existing file changed: %q, %v", contents, err)
	}
}
