//go:build integration

package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestPostgres18 exercises the generated code against a disposable Docker container.
func TestPostgres18(t *testing.T) {
	out, err := exec.Command("docker", "run", "--detach", "--rm", "--publish", "127.0.0.1::5432", "--env", "POSTGRES_PASSWORD=capg-test", "--env", "POSTGRES_DB=capg_test", "postgres:18").CombinedOutput()
	if err != nil {
		t.Fatalf("start PostgreSQL 18 container: %v\n%s", err, out)
	}
	container := strings.TrimSpace(string(out))
	t.Cleanup(func() {
		if out, err := exec.Command("docker", "rm", "-f", container).CombinedOutput(); err != nil {
			t.Errorf("remove test container: %v\n%s", err, out)
		}
	})
	deadline := time.Now().Add(90 * time.Second)
	for {
		if err := exec.Command("docker", "exec", container, "pg_isready", "-U", "postgres", "-d", "capg_test").Run(); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("PostgreSQL did not become ready")
		}
		time.Sleep(500 * time.Millisecond)
	}
	out, err = exec.Command("docker", "port", container, "5432/tcp").CombinedOutput()
	if err != nil {
		t.Fatalf("get PostgreSQL port: %v\n%s", err, out)
	}
	_, port, err := net.SplitHostPort(strings.TrimSpace(string(out)))
	if err != nil {
		t.Fatal(err)
	}
	dsn := fmt.Sprintf("postgres://postgres:capg-test@127.0.0.1:%s/capg_test?sslmode=disable", port)

	root := t.TempDir()
	binary := filepath.Join(root, "capg")
	cmd := exec.Command("go", "build", "-o", binary, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	project := filepath.Join(root, "project")
	if err := os.Mkdir(project, 0755); err != nil {
		t.Fatal(err)
	}
	mod := "module example.com/integration\n\ngo 1.25\n\nrequire (\n github.com/jmoiron/sqlx v1.4.0\n github.com/golang-migrate/migrate/v4 v4.18.3\n github.com/lib/pq v1.10.9\n)\n"
	if err := os.WriteFile(filepath.Join(project, "go.mod"), []byte(mod), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"entity", "Product"}, {"entity", "Order", "--table", "order"}, {"entity", "Widget", "-s"}, {"migration", "add_product_price"}} {
		cmd = exec.Command(binary, args...)
		cmd.Dir = project
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("generate %v: %v\n%s", args, err, out)
		}
	}
	for direction, sql := range map[string]string{
		"up":   "CREATE TABLE price_history (id integer PRIMARY KEY);\n",
		"down": "DROP TABLE price_history;\n",
	} {
		matches, err := filepath.Glob(filepath.Join(project, "migrations", "pg", "migrations", "*_add_product_price."+direction+".sql"))
		if err != nil || len(matches) != 1 {
			t.Fatalf("custom migration %s: %v, %v", direction, matches, err)
		}
		if err := os.WriteFile(matches[0], []byte(sql), 0644); err != nil {
			t.Fatal(err)
		}
	}
	fixture, err := os.ReadFile(filepath.Join("testdata", "postgres_integration_test.go.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "integration_test.go"), fixture, 0644); err != nil {
		t.Fatal(err)
	}
	connectionTest, err := os.ReadFile(filepath.Join("testdata", "connection_test.go.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "internal", "db", "connection_test.go"), connectionTest, 0644); err != nil {
		t.Fatal(err)
	}
	migrationTest, err := os.ReadFile(filepath.Join("testdata", "migrate_context_test.go.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "migrations", "pg", "migrate_context_test.go"), migrationTest, 0644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd = exec.CommandContext(ctx, "go", "test", "-mod=mod", "-v", "./...")
	cmd.Dir = project
	cmd.Env = append(os.Environ(), "CAPG_TEST_DATABASE_URL="+dsn)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated project integration test: %v\n%s", err, out)
	} else {
		t.Logf("%s", out)
	}
}
