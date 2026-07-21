package main

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// testSockDir is the unix socket directory of an ephemeral, local-only
// Postgres cluster started for the duration of this test binary's run (see
// TestMain). Each test creates and drops its own database on this cluster
// for isolation (openTestDB in backend_test.go), matching this project's
// existing testing convention of hitting a real database rather than a
// mock (the old SQLite tests used a fresh :memory: database per test).
var testSockDir string

func TestMain(m *testing.M) {
	cluster, err := startTestPostgres()
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to start ephemeral test postgres:", err)
		os.Exit(1)
	}
	testSockDir = cluster.sockDir

	code := m.Run()
	cluster.stop()
	os.Exit(code)
}

type testPostgresCluster struct {
	baseDir string
	dataDir string
	sockDir string
}

func startTestPostgres() (*testPostgresCluster, error) {
	baseDir, err := os.MkdirTemp("", "daysuntil-pg-")
	if err != nil {
		return nil, err
	}

	dataDir := filepath.Join(baseDir, "data")
	sockDir := filepath.Join(baseDir, "sock")
	if err := os.MkdirAll(sockDir, 0o700); err != nil {
		os.RemoveAll(baseDir)
		return nil, err
	}

	initdb := exec.Command("initdb", "--pgdata="+dataDir, "--username=postgres", "--auth=trust", "--no-sync")
	if out, err := initdb.CombinedOutput(); err != nil {
		os.RemoveAll(baseDir)
		return nil, fmt.Errorf("initdb: %w\n%s", err, out)
	}

	// -l redirects the postgres server's own stdout/stderr to a log file.
	// Without it, CombinedOutput()'s pipe gets inherited by postgres's
	// long-running background workers (checkpointer, bgwriter, ...), which
	// never close it — so Wait() blocks forever even after pg_ctl itself
	// (and the -w readiness wait) has successfully finished.
	logFile := filepath.Join(baseDir, "postgres.log")
	postgresOpts := fmt.Sprintf("-c listen_addresses= -c unix_socket_directories=%s -c fsync=off -c full_page_writes=off", sockDir)
	start := exec.Command("pg_ctl", "start", "-D", dataDir, "-w", "-l", logFile, "-o", postgresOpts)
	if out, err := start.CombinedOutput(); err != nil {
		logContents, _ := os.ReadFile(logFile)
		os.RemoveAll(baseDir)
		return nil, fmt.Errorf("pg_ctl start: %w\n%s\n%s", err, out, logContents)
	}

	return &testPostgresCluster{baseDir: baseDir, dataDir: dataDir, sockDir: sockDir}, nil
}

func testDatabaseDSN(sockDir, dbName string) string {
	return fmt.Sprintf("postgres://postgres@?host=%s&dbname=%s&sslmode=disable", url.QueryEscape(sockDir), dbName)
}

func (c *testPostgresCluster) stop() {
	exec.Command("pg_ctl", "stop", "-D", c.dataDir, "-m", "fast").Run()
	os.RemoveAll(c.baseDir)
}
