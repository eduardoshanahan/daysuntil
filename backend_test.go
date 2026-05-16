package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	return db
}

func TestInitDBAddsColorColumnToLegacySchema(t *testing.T) {
	db := openTestDB(t)

	_, err := db.Exec(`CREATE TABLE intervals (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		start_date TEXT NOT NULL,
		end_date TEXT NOT NULL
	)`)
	if err != nil {
		t.Fatalf("create legacy table: %v", err)
	}

	if err := initDB(db); err != nil {
		t.Fatalf("init db: %v", err)
	}

	hasColorColumn, err := intervalColumnExists(db, "color")
	if err != nil {
		t.Fatalf("check color column: %v", err)
	}
	if !hasColorColumn {
		t.Fatal("expected color column to be present after migration")
	}
}

func TestValidateIntervalRejectsNonIncreasingDates(t *testing.T) {
	err := validateInterval(Interval{
		Name:      "Test",
		StartDate: "2026-05-20",
		EndDate:   "2026-05-20",
	})
	if err == nil {
		t.Fatal("expected validation error for equal start and end date")
	}
}

func TestCreateIntervalRejectsUnknownFields(t *testing.T) {
	db := openTestDB(t)
	if err := initDB(db); err != nil {
		t.Fatalf("init db: %v", err)
	}

	h := &handler{db: db}
	req := httptest.NewRequest(http.MethodPost, "/api/intervals", strings.NewReader(`{
		"name":"Trip",
		"start_date":"2026-05-20",
		"end_date":"2026-05-21",
		"unexpected":true
	}`))
	rec := httptest.NewRecorder()

	h.createInterval(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestUpdateIntervalReturnsNotFound(t *testing.T) {
	db := openTestDB(t)
	if err := initDB(db); err != nil {
		t.Fatalf("init db: %v", err)
	}

	h := &handler{db: db}
	r := chi.NewRouter()
	r.Put("/api/intervals/{id}", h.updateInterval)

	req := httptest.NewRequest(http.MethodPut, "/api/intervals/999", strings.NewReader(`{
		"name":"Trip",
		"start_date":"2026-05-20",
		"end_date":"2026-05-21"
	}`))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestDeleteIntervalReturnsNotFound(t *testing.T) {
	db := openTestDB(t)
	if err := initDB(db); err != nil {
		t.Fatalf("init db: %v", err)
	}

	h := &handler{db: db}
	r := chi.NewRouter()
	r.Delete("/api/intervals/{id}", h.deleteInterval)

	req := httptest.NewRequest(http.MethodDelete, "/api/intervals/999", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
