package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

func newTestServer(t *testing.T) (*sql.DB, http.Handler) {
	t.Helper()
	return newTestServerWithHandler(t, &handler{authLimiter: newAuthRateLimiter()})
}

func newTestServerWithHandler(t *testing.T, h *handler) (*sql.DB, http.Handler) {
	t.Helper()

	db := openTestDB(t)
	if err := initDB(db); err != nil {
		t.Fatalf("init db: %v", err)
	}

	h.db = db
	return db, newRouter(h)
}

func performRequest(t *testing.T, h http.Handler, method, path, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	return performRequestFromRemoteAddr(t, h, method, path, body, "192.0.2.1:1234", cookies...)
}

func performRequestFromRemoteAddr(t *testing.T, h http.Handler, method, path, body, remoteAddr string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr = remoteAddr
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeUser(t *testing.T, rec *httptest.ResponseRecorder) User {
	t.Helper()

	var user User
	if err := json.NewDecoder(rec.Body).Decode(&user); err != nil {
		t.Fatalf("decode user: %v", err)
	}
	return user
}

func registerUser(t *testing.T, h http.Handler, email, username, password string) (*http.Cookie, User) {
	t.Helper()

	rec := performRequest(t, h, http.MethodPost, "/api/register", `{"email":"`+email+`","username":"`+username+`","password":"`+password+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("register user: expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}

	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie after registration")
	}

	return cookies[0], decodeUser(t, rec)
}

func TestInitDBAddsColumnsToLegacySchema(t *testing.T) {
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

	for _, column := range []string{"color", "user_id", "visibility"} {
		exists, err := intervalColumnExists(db, column)
		if err != nil {
			t.Fatalf("check %s column: %v", column, err)
		}
		if !exists {
			t.Fatalf("expected %s column after migration", column)
		}
	}
}

func TestInitDBAddsEmailColumnToLegacyUsersSchema(t *testing.T) {
	db := openTestDB(t)

	_, err := db.Exec(`CREATE TABLE users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		display_name TEXT NOT NULL DEFAULT '',
		password_hash TEXT NOT NULL,
		created_at TEXT NOT NULL
	)`)
	if err != nil {
		t.Fatalf("create legacy users table: %v", err)
	}

	if err := initDB(db); err != nil {
		t.Fatalf("init db: %v", err)
	}

	for _, column := range []string{"email", "auth_provider", "auth_provider_user_id"} {
		exists, err := tableColumnExists(db, "users", column)
		if err != nil {
			t.Fatalf("check %s column: %v", column, err)
		}
		if !exists {
			t.Fatalf("expected %s column after migration", column)
		}
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

func TestRegisterAdoptsLegacyIntervalsForFirstUser(t *testing.T) {
	db, router := newTestServer(t)

	_, err := db.Exec(
		`INSERT INTO intervals (name, start_date, end_date, color, user_id) VALUES (?, ?, ?, ?, NULL)`,
		"Legacy Trip", "2026-05-20", "2026-05-30", "#4f8ef7",
	)
	if err != nil {
		t.Fatalf("insert legacy interval: %v", err)
	}

	cookie, user := registerUser(t, router, "alice@example.com", "alice", "password123")
	if user.Username != "alice" {
		t.Fatalf("expected username alice, got %q", user.Username)
	}
	if user.DisplayName != "alice" {
		t.Fatalf("expected default display name alice, got %q", user.DisplayName)
	}
	if user.PublicSlug == "" {
		t.Fatal("expected public slug to be generated")
	}

	rec := performRequest(t, router, http.MethodGet, "/api/intervals", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("list intervals: expected 200, got %d", rec.Code)
	}

	var intervals []Interval
	if err := json.NewDecoder(rec.Body).Decode(&intervals); err != nil {
		t.Fatalf("decode intervals: %v", err)
	}
	if len(intervals) != 1 || intervals[0].Name != "Legacy Trip" {
		t.Fatalf("expected adopted legacy interval, got %#v", intervals)
	}
}

func TestIntervalsRequireAuthentication(t *testing.T) {
	_, router := newTestServer(t)

	rec := performRequest(t, router, http.MethodGet, "/api/intervals", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestVersionEndpoint(t *testing.T) {
	_, router := newTestServer(t)

	rec := performRequest(t, router, http.MethodGet, "/api/version", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 loading version, got %d (%s)", rec.Code, rec.Body.String())
	}

	var payload versionResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode version: %v", err)
	}
	if payload.Version != currentVersion() {
		t.Fatalf("expected version %q, got %q", currentVersion(), payload.Version)
	}
}

func TestLoginAndCurrentUser(t *testing.T) {
	_, router := newTestServer(t)

	registerUser(t, router, "alice@example.com", "alice", "password123")

	rec := performRequest(t, router, http.MethodPost, "/api/login", `{"email":"alice@example.com","password":"password123"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("login: expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	cookie := rec.Result().Cookies()[0]

	me := performRequest(t, router, http.MethodGet, "/api/me", "", cookie)
	if me.Code != http.StatusOK {
		t.Fatalf("current user: expected 200, got %d", me.Code)
	}
	user := decodeUser(t, me)
	if user.Username != "alice" {
		t.Fatalf("expected current user alice, got %q", user.Username)
	}
	if user.PublicSlug == "" {
		t.Fatal("expected current user response to include public slug")
	}
}

func TestUsersOnlySeeTheirOwnIntervals(t *testing.T) {
	_, router := newTestServer(t)

	aliceCookie, _ := registerUser(t, router, "alice@example.com", "alice", "password123")
	bobCookie, _ := registerUser(t, router, "bob@example.com", "bob", "password123")

	createAlice := performRequest(t, router, http.MethodPost, "/api/intervals", `{
		"name":"Alice Trip",
		"start_date":"2026-05-20",
		"end_date":"2026-05-21",
		"color":"#4f8ef7",
		"visibility":"private"
	}`, aliceCookie)
	if createAlice.Code != http.StatusCreated {
		t.Fatalf("create alice interval: expected 201, got %d (%s)", createAlice.Code, createAlice.Body.String())
	}

	createBob := performRequest(t, router, http.MethodPost, "/api/intervals", `{
		"name":"Bob Trip",
		"start_date":"2026-06-20",
		"end_date":"2026-06-21",
		"color":"#e05c5c",
		"visibility":"public"
	}`, bobCookie)
	if createBob.Code != http.StatusCreated {
		t.Fatalf("create bob interval: expected 201, got %d (%s)", createBob.Code, createBob.Body.String())
	}

	aliceList := performRequest(t, router, http.MethodGet, "/api/intervals", "", aliceCookie)
	bobList := performRequest(t, router, http.MethodGet, "/api/intervals", "", bobCookie)

	var aliceIntervals []Interval
	if err := json.NewDecoder(aliceList.Body).Decode(&aliceIntervals); err != nil {
		t.Fatalf("decode alice intervals: %v", err)
	}
	if len(aliceIntervals) != 1 || aliceIntervals[0].Name != "Alice Trip" {
		t.Fatalf("expected only Alice interval, got %#v", aliceIntervals)
	}

	var bobIntervals []Interval
	if err := json.NewDecoder(bobList.Body).Decode(&bobIntervals); err != nil {
		t.Fatalf("decode bob intervals: %v", err)
	}
	if len(bobIntervals) != 1 || bobIntervals[0].Name != "Bob Trip" {
		t.Fatalf("expected only Bob interval, got %#v", bobIntervals)
	}

	updateBobFromAlice := performRequest(t, router, http.MethodPut, "/api/intervals/2", `{
		"name":"Stolen",
		"start_date":"2026-06-20",
		"end_date":"2026-06-22",
		"color":"#000000"
	}`, aliceCookie)
	if updateBobFromAlice.Code != http.StatusNotFound {
		t.Fatalf("expected 404 updating another user's interval, got %d", updateBobFromAlice.Code)
	}

	deleteAliceFromBob := performRequest(t, router, http.MethodDelete, "/api/intervals/1", "", bobCookie)
	if deleteAliceFromBob.Code != http.StatusNotFound {
		t.Fatalf("expected 404 deleting another user's interval, got %d", deleteAliceFromBob.Code)
	}
}

func TestCreateIntervalRejectsUnknownFields(t *testing.T) {
	_, router := newTestServer(t)

	cookie, _ := registerUser(t, router, "alice@example.com", "alice", "password123")

	rec := performRequest(t, router, http.MethodPost, "/api/intervals", `{
		"name":"Trip",
		"start_date":"2026-05-20",
		"end_date":"2026-05-21",
		"unexpected":true
	}`, cookie)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestUpdateDisplayName(t *testing.T) {
	_, router := newTestServer(t)

	cookie, _ := registerUser(t, router, "alice@example.com", "alice", "password123")
	rec := performRequest(t, router, http.MethodPut, "/api/me/profile", `{"display_name":"Eduardo Shanahan"}`, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 updating profile, got %d (%s)", rec.Code, rec.Body.String())
	}

	user := decodeUser(t, rec)
	if user.DisplayName != "Eduardo Shanahan" {
		t.Fatalf("expected updated display name, got %q", user.DisplayName)
	}
}

func TestPublicProfileOnlyShowsPublicIntervals(t *testing.T) {
	_, router := newTestServer(t)

	cookie, user := registerUser(t, router, "alice@example.com", "alice", "password123")

	updateProfile := performRequest(t, router, http.MethodPut, "/api/me/profile", `{"display_name":"Alice Public"}`, cookie)
	if updateProfile.Code != http.StatusOK {
		t.Fatalf("expected 200 updating profile, got %d", updateProfile.Code)
	}

	privateInterval := performRequest(t, router, http.MethodPost, "/api/intervals", `{
		"name":"Private Trip",
		"start_date":"2026-05-20",
		"end_date":"2026-05-21",
		"color":"#4f8ef7",
		"visibility":"private"
	}`, cookie)
	if privateInterval.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating private interval, got %d", privateInterval.Code)
	}

	publicInterval := performRequest(t, router, http.MethodPost, "/api/intervals", `{
		"name":"Public Trip",
		"start_date":"2026-06-20",
		"end_date":"2026-06-21",
		"color":"#e05c5c",
		"visibility":"public"
	}`, cookie)
	if publicInterval.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating public interval, got %d", publicInterval.Code)
	}

	rec := performRequest(t, router, http.MethodGet, "/api/public/profiles/"+user.PublicSlug, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 loading public profile, got %d (%s)", rec.Code, rec.Body.String())
	}

	var profile PublicProfile
	if err := json.NewDecoder(rec.Body).Decode(&profile); err != nil {
		t.Fatalf("decode public profile: %v", err)
	}

	if profile.DisplayName != "Alice Public" {
		t.Fatalf("expected public display name, got %q", profile.DisplayName)
	}
	if len(profile.Intervals) != 1 || profile.Intervals[0].Name != "Public Trip" {
		t.Fatalf("expected only public interval, got %#v", profile.Intervals)
	}
}

func TestPublicProfileReturnsNotFoundWhenUserHasNoPublicIntervals(t *testing.T) {
	_, router := newTestServer(t)

	cookie, user := registerUser(t, router, "alice@example.com", "alice", "password123")

	privateInterval := performRequest(t, router, http.MethodPost, "/api/intervals", `{
		"name":"Private Trip",
		"start_date":"2026-05-20",
		"end_date":"2026-05-21",
		"color":"#4f8ef7",
		"visibility":"private"
	}`, cookie)
	if privateInterval.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating private interval, got %d (%s)", privateInterval.Code, privateInterval.Body.String())
	}

	rec := performRequest(t, router, http.MethodGet, "/api/public/profiles/"+user.PublicSlug, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when user has no public intervals, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestLoginRateLimitReturnsTooManyRequests(t *testing.T) {
	limiter := newAuthRateLimiter()
	limiter.now = func() time.Time {
		return time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	}
	limiter.policies[authActionLogin] = authRatePolicy{limit: 2, window: time.Minute}

	_, router := newTestServerWithHandler(t, &handler{authLimiter: limiter})
	registerUser(t, router, "alice@example.com", "alice", "password123")

	for attempt := 1; attempt <= 2; attempt++ {
		rec := performRequestFromRemoteAddr(t, router, http.MethodPost, "/api/login", `{"email":"alice@example.com","password":"wrongpass"}`, "198.51.100.10:4567")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401, got %d (%s)", attempt, rec.Code, rec.Body.String())
		}
	}

	blocked := performRequestFromRemoteAddr(t, router, http.MethodPost, "/api/login", `{"email":"alice@example.com","password":"wrongpass"}`, "198.51.100.10:4567")
	if blocked.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after limit, got %d (%s)", blocked.Code, blocked.Body.String())
	}
	if blocked.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header on rate-limited login")
	}
}

func TestRegisterRateLimitReturnsTooManyRequests(t *testing.T) {
	limiter := newAuthRateLimiter()
	limiter.now = func() time.Time {
		return time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	}
	limiter.policies[authActionRegister] = authRatePolicy{limit: 2, window: time.Minute}

	_, router := newTestServerWithHandler(t, &handler{authLimiter: limiter})

	for attempt := 1; attempt <= 2; attempt++ {
		rec := performRequestFromRemoteAddr(t, router, http.MethodPost, "/api/register", `{"email":"user`+string(rune('0'+attempt))+`@example.com","username":"user`+string(rune('0'+attempt))+`","password":"password123"}`, "203.0.113.22:8080")
		if rec.Code != http.StatusOK {
			t.Fatalf("attempt %d: expected 200, got %d (%s)", attempt, rec.Code, rec.Body.String())
		}
	}

	blocked := performRequestFromRemoteAddr(t, router, http.MethodPost, "/api/register", `{"email":"user3@example.com","username":"user3","password":"password123"}`, "203.0.113.22:8080")
	if blocked.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after limit, got %d (%s)", blocked.Code, blocked.Body.String())
	}
	if blocked.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header on rate-limited registration")
	}
}

func TestLoginAndRegisterRateLimitsAreIndependent(t *testing.T) {
	limiter := newAuthRateLimiter()
	limiter.now = func() time.Time {
		return time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	}
	limiter.policies[authActionLogin] = authRatePolicy{limit: 1, window: time.Minute}
	limiter.policies[authActionRegister] = authRatePolicy{limit: 2, window: time.Minute}

	_, router := newTestServerWithHandler(t, &handler{authLimiter: limiter})
	registerUser(t, router, "alice@example.com", "alice", "password123")

	firstLogin := performRequestFromRemoteAddr(t, router, http.MethodPost, "/api/login", `{"email":"alice@example.com","password":"wrongpass"}`, "198.51.100.30:9999")
	if firstLogin.Code != http.StatusUnauthorized {
		t.Fatalf("expected first login attempt to pass through, got %d", firstLogin.Code)
	}

	secondLogin := performRequestFromRemoteAddr(t, router, http.MethodPost, "/api/login", `{"email":"alice@example.com","password":"wrongpass"}`, "198.51.100.30:9999")
	if secondLogin.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second login attempt to be rate-limited, got %d", secondLogin.Code)
	}

	registerAttempt := performRequestFromRemoteAddr(t, router, http.MethodPost, "/api/register", `{"email":"bob@example.com","username":"bob","password":"password123"}`, "198.51.100.30:9999")
	if registerAttempt.Code != http.StatusOK {
		t.Fatalf("expected register limit to remain independent, got %d (%s)", registerAttempt.Code, registerAttempt.Body.String())
	}
}

func TestLoginRejectsUsernameAsIdentifier(t *testing.T) {
	_, router := newTestServer(t)

	registerUser(t, router, "alice@example.com", "alice", "password123")

	rec := performRequest(t, router, http.MethodPost, "/api/login", `{"email":"alice","password":"password123"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d (%s)", rec.Code, rec.Body.String())
	}
	if strings.TrimSpace(rec.Body.String()) != "invalid email or password" {
		t.Fatalf("expected generic auth failure, got %q", strings.TrimSpace(rec.Body.String()))
	}
}

func TestOAuthAccountCannotUseLocalPasswordLogin(t *testing.T) {
	db, router := newTestServer(t)

	_, err := db.Exec(
		`INSERT INTO users (email, username, display_name, password_hash, auth_provider, auth_provider_user_id, created_at) VALUES (?, ?, ?, '', ?, ?, ?)`,
		"oauth@example.com", "gh_alice", "Alice OAuth", "gh", "12345", time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("insert oauth user: %v", err)
	}

	rec := performRequest(t, router, http.MethodPost, "/api/login", `{"email":"oauth@example.com","password":"password123"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d (%s)", rec.Code, rec.Body.String())
	}
	if strings.TrimSpace(rec.Body.String()) != "invalid email or password" {
		t.Fatalf("expected generic auth failure, got %q", strings.TrimSpace(rec.Body.String()))
	}
}

func TestCurrentUserResponseDoesNotExposeEmail(t *testing.T) {
	_, router := newTestServer(t)

	cookie, _ := registerUser(t, router, "alice@example.com", "alice", "password123")
	rec := performRequest(t, router, http.MethodGet, "/api/me", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "alice@example.com") {
		t.Fatalf("expected email to stay private, got %q", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "\"email\"") {
		t.Fatalf("expected email field to stay private, got %q", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "\"public_slug\"") {
		t.Fatalf("expected public slug in current user response, got %q", rec.Body.String())
	}
}

func TestRotatePublicLinkInvalidatesOldSlug(t *testing.T) {
	_, router := newTestServer(t)

	cookie, user := registerUser(t, router, "alice@example.com", "alice", "password123")

	create := performRequest(t, router, http.MethodPost, "/api/intervals", `{
		"name":"Trip",
		"start_date":"2026-05-20",
		"end_date":"2026-05-21",
		"color":"#4f8ef7",
		"visibility":"public"
	}`, cookie)
	if create.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating interval, got %d (%s)", create.Code, create.Body.String())
	}

	oldPublic := performRequest(t, router, http.MethodGet, "/api/public/profiles/"+user.PublicSlug, "")
	if oldPublic.Code != http.StatusOK {
		t.Fatalf("expected 200 for current public slug, got %d (%s)", oldPublic.Code, oldPublic.Body.String())
	}

	rotate := performRequest(t, router, http.MethodPost, "/api/me/public-link/rotate", "", cookie)
	if rotate.Code != http.StatusOK {
		t.Fatalf("expected 200 rotating public link, got %d (%s)", rotate.Code, rotate.Body.String())
	}

	updatedUser := decodeUser(t, rotate)
	if updatedUser.PublicSlug == "" {
		t.Fatal("expected rotated public slug")
	}
	if updatedUser.PublicSlug == user.PublicSlug {
		t.Fatalf("expected a new public slug, got same value %q", updatedUser.PublicSlug)
	}

	oldAfterRotate := performRequest(t, router, http.MethodGet, "/api/public/profiles/"+user.PublicSlug, "")
	if oldAfterRotate.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for old public slug, got %d (%s)", oldAfterRotate.Code, oldAfterRotate.Body.String())
	}

	newAfterRotate := performRequest(t, router, http.MethodGet, "/api/public/profiles/"+updatedUser.PublicSlug, "")
	if newAfterRotate.Code != http.StatusOK {
		t.Fatalf("expected 200 for rotated public slug, got %d (%s)", newAfterRotate.Code, newAfterRotate.Body.String())
	}
}

func TestDeleteAccountRemovesUserIntervalsAndSession(t *testing.T) {
	db, router := newTestServer(t)

	cookie, user := registerUser(t, router, "alice@example.com", "alice", "password123")

	create := performRequest(t, router, http.MethodPost, "/api/intervals", `{
		"name":"Trip",
		"start_date":"2026-05-20",
		"end_date":"2026-05-21",
		"color":"#4f8ef7",
		"visibility":"public"
	}`, cookie)
	if create.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating interval, got %d (%s)", create.Code, create.Body.String())
	}

	del := performRequest(t, router, http.MethodDelete, "/api/me", "", cookie)
	if del.Code != http.StatusNoContent {
		t.Fatalf("expected 204 deleting account, got %d (%s)", del.Code, del.Body.String())
	}

	me := performRequest(t, router, http.MethodGet, "/api/me", "", cookie)
	if me.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 after account deletion, got %d (%s)", me.Code, me.Body.String())
	}

	publicProfile := performRequest(t, router, http.MethodGet, "/api/public/profiles/"+user.PublicSlug, "")
	if publicProfile.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for deleted public profile, got %d (%s)", publicProfile.Code, publicProfile.Body.String())
	}

	var userCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&userCount); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if userCount != 0 {
		t.Fatalf("expected no users after deletion, got %d", userCount)
	}

	var intervalCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM intervals`).Scan(&intervalCount); err != nil {
		t.Fatalf("count intervals: %v", err)
	}
	if intervalCount != 0 {
		t.Fatalf("expected no intervals after deletion, got %d", intervalCount)
	}

	var sessionCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&sessionCount); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if sessionCount != 0 {
		t.Fatalf("expected no sessions after deletion, got %d", sessionCount)
	}
}
