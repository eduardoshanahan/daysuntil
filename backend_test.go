package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

var shareGroupSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+){3}$`)

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
	return newTestServerWithHandler(t, &handler{authLimiter: newAuthRateLimiter(nil)})
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

func decodeShareGroup(t *testing.T, rec *httptest.ResponseRecorder) ShareGroup {
	t.Helper()

	var group ShareGroup
	if err := json.NewDecoder(rec.Body).Decode(&group); err != nil {
		t.Fatalf("decode share group: %v", err)
	}
	return group
}

// createTestUser inserts a Zitadel-authenticated user directly into the DB and
// returns a valid session cookie and the User record. This replaces the
// register+login flow that no longer exists after switching to Zitadel OIDC.
func createTestUser(t *testing.T, db *sql.DB, sub, username string) (*http.Cookie, User) {
	t.Helper()

	user, err := findOrCreateZitadelUser(db, sub, username+"@test.example", username)
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}

	// Set a real username so tests can use share groups etc.
	updated, err := setUserUsername(db, user.ID, username)
	if err != nil {
		t.Fatalf("set test username: %v", err)
	}

	rawToken, expiresAt, err := createSession(db, updated.ID)
	if err != nil {
		t.Fatalf("create test session: %v", err)
	}

	cookie := &http.Cookie{
		Name:    sessionCookieName,
		Value:   rawToken,
		Expires: expiresAt,
	}
	return cookie, updated
}

func createShareGroupForTest(t *testing.T, router http.Handler, cookie *http.Cookie, name string) ShareGroup {
	t.Helper()

	rec := performRequest(t, router, http.MethodPost, "/api/share-groups", `{"name":"`+name+`"}`, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create share group: expected 201, got %d (%s)", rec.Code, rec.Body.String())
	}
	group := decodeShareGroup(t, rec)
	assertShareGroupSlug(t, group.PublicSlug)
	return group
}

func assertShareGroupSlug(t *testing.T, slug string) {
	t.Helper()
	if !shareGroupSlugPattern.MatchString(slug) {
		t.Fatalf("expected share group slug to have three readable parts and one suffix, got %q", slug)
	}
	parts := strings.Split(slug, "-")
	if len(parts) != 4 {
		t.Fatalf("expected 4 slug parts, got %d in %q", len(parts), slug)
	}
	if len(parts[3]) != shareGroupSlugSuffixLength {
		t.Fatalf("expected suffix length %d, got %d in %q", shareGroupSlugSuffixLength, len(parts[3]), slug)
	}
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

	for _, column := range []string{"color", "user_id"} {
		exists, err := intervalColumnExists(db, column)
		if err != nil {
			t.Fatalf("check %s column: %v", column, err)
		}
		if !exists {
			t.Fatalf("expected %s column after migration", column)
		}
	}
}

func TestInitDBAddsZitadelSubColumnToLegacyUsersSchema(t *testing.T) {
	db := openTestDB(t)

	_, err := db.Exec(`CREATE TABLE users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		display_name TEXT NOT NULL DEFAULT '',
		password_hash TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL
	)`)
	if err != nil {
		t.Fatalf("create legacy users table: %v", err)
	}

	if err := initDB(db); err != nil {
		t.Fatalf("init db: %v", err)
	}

	exists, err := tableColumnExists(db, "users", "zitadel_sub")
	if err != nil {
		t.Fatalf("check zitadel_sub column: %v", err)
	}
	if !exists {
		t.Fatalf("expected zitadel_sub column after migration")
	}
}

func TestValidateIntervalRejectsEndDateBeforeStartDate(t *testing.T) {
	db := openTestDB(t)
	if err := initDB(db); err != nil {
		t.Fatalf("init db: %v", err)
	}

	_, err := validateInterval(intervalInput{
		Name:      "Test",
		StartDate: "2026-05-20",
		EndDate:   "2026-05-19",
	}, db, 1)
	if err == nil {
		t.Fatal("expected validation error when end date is before start date")
	}
}

func TestValidateIntervalAllowsSameDayRange(t *testing.T) {
	db := openTestDB(t)
	if err := initDB(db); err != nil {
		t.Fatalf("init db: %v", err)
	}

	_, err := validateInterval(intervalInput{
		Name:      "Test",
		StartDate: "2026-05-20",
		EndDate:   "2026-05-20",
	}, db, 1)
	if err != nil {
		t.Fatalf("expected same-day interval to be valid, got %v", err)
	}
}

func TestFirstZitadelUserAdoptsLegacyIntervals(t *testing.T) {
	db, router := newTestServer(t)

	_, err := db.Exec(
		`INSERT INTO intervals (name, start_date, end_date, color, user_id) VALUES (?, ?, ?, ?, NULL)`,
		"Legacy Trip", "2026-05-20", "2026-05-30", "#4f8ef7",
	)
	if err != nil {
		t.Fatalf("insert legacy interval: %v", err)
	}

	cookie, user := createTestUser(t, db, "sub-alice-001", "alice")
	if user.Username != "alice" {
		t.Fatalf("expected username alice, got %q", user.Username)
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

func TestStaticResponsesIncludeSecurityHeaders(t *testing.T) {
	_, router := newTestServer(t)

	rec := performRequest(t, router, http.MethodGet, "/", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("expected Content-Security-Policy header")
	}
	if rec.Header().Get("Referrer-Policy") == "" {
		t.Fatal("expected Referrer-Policy header")
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("expected nosniff header, got %q", rec.Header().Get("X-Content-Type-Options"))
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("expected X-Frame-Options DENY, got %q", rec.Header().Get("X-Frame-Options"))
	}
}

func TestAPIResponsesDisableCaching(t *testing.T) {
	_, router := newTestServer(t)

	rec := performRequest(t, router, http.MethodGet, "/api/version", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("expected Cache-Control no-store, got %q", rec.Header().Get("Cache-Control"))
	}
	if rec.Header().Get("Pragma") != "no-cache" {
		t.Fatalf("expected Pragma no-cache, got %q", rec.Header().Get("Pragma"))
	}
	if rec.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("expected Content-Security-Policy header on API response")
	}
}

func TestCurrentUser(t *testing.T) {
	db, router := newTestServer(t)

	cookie, _ := createTestUser(t, db, "sub-alice-001", "alice")

	me := performRequest(t, router, http.MethodGet, "/api/me", "", cookie)
	if me.Code != http.StatusOK {
		t.Fatalf("current user: expected 200, got %d", me.Code)
	}
	user := decodeUser(t, me)
	if user.Username != "alice" {
		t.Fatalf("expected current user alice, got %q", user.Username)
	}
}

func TestUsersOnlySeeTheirOwnIntervals(t *testing.T) {
	db, router := newTestServer(t)

	aliceCookie, _ := createTestUser(t, db, "sub-alice-001", "alice")
	bobCookie, _ := createTestUser(t, db, "sub-bob-001", "bob")
	bobGroup := createShareGroupForTest(t, router, bobCookie, "Bob Trips")

	createAlice := performRequest(t, router, http.MethodPost, "/api/intervals", `{
		"name":"Alice Trip",
		"start_date":"2026-05-20",
		"end_date":"2026-05-21",
		"color":"#4f8ef7",
		"share_group_ids":[]
	}`, aliceCookie)
	if createAlice.Code != http.StatusCreated {
		t.Fatalf("create alice interval: expected 201, got %d (%s)", createAlice.Code, createAlice.Body.String())
	}

	createBob := performRequest(t, router, http.MethodPost, "/api/intervals", `{
		"name":"Bob Trip",
		"start_date":"2026-06-20",
		"end_date":"2026-06-21",
		"color":"#e05c5c",
		"share_group_ids":[`+strconv.FormatInt(bobGroup.ID, 10)+`]
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
	db, router := newTestServer(t)

	cookie, _ := createTestUser(t, db, "sub-alice-001", "alice")

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
	db, router := newTestServer(t)

	cookie, _ := createTestUser(t, db, "sub-alice-001", "alice")
	rec := performRequest(t, router, http.MethodPut, "/api/me/profile", `{"display_name":"Eduardo Shanahan"}`, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 updating profile, got %d (%s)", rec.Code, rec.Body.String())
	}

	user := decodeUser(t, rec)
	if user.DisplayName != "Eduardo Shanahan" {
		t.Fatalf("expected updated display name, got %q", user.DisplayName)
	}
}

func TestShareGroupsListCreateUpdateRotateAndDelete(t *testing.T) {
	db, router := newTestServer(t)

	cookie, _ := createTestUser(t, db, "sub-alice-001", "alice")
	group := createShareGroupForTest(t, router, cookie, "Trips")
	if group.PublicSlug == "" {
		t.Fatal("expected public slug for share group")
	}
	assertShareGroupSlug(t, group.PublicSlug)

	listRec := performRequest(t, router, http.MethodGet, "/api/share-groups", "", cookie)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 listing groups, got %d (%s)", listRec.Code, listRec.Body.String())
	}
	var groups []ShareGroup
	if err := json.NewDecoder(listRec.Body).Decode(&groups); err != nil {
		t.Fatalf("decode groups: %v", err)
	}
	if len(groups) != 1 || groups[0].Name != "Trips" {
		t.Fatalf("expected created group in list, got %#v", groups)
	}

	updateRec := performRequest(t, router, http.MethodPut, "/api/share-groups/"+strconv.FormatInt(group.ID, 10), `{"name":"Summer trips"}`, cookie)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected 200 updating group, got %d (%s)", updateRec.Code, updateRec.Body.String())
	}
	updatedGroup := decodeShareGroup(t, updateRec)
	if updatedGroup.Name != "Summer trips" {
		t.Fatalf("expected renamed group, got %q", updatedGroup.Name)
	}

	rotateRec := performRequest(t, router, http.MethodPost, "/api/share-groups/"+strconv.FormatInt(group.ID, 10)+"/rotate", "", cookie)
	if rotateRec.Code != http.StatusOK {
		t.Fatalf("expected 200 rotating group slug, got %d (%s)", rotateRec.Code, rotateRec.Body.String())
	}
	rotatedGroup := decodeShareGroup(t, rotateRec)
	if rotatedGroup.PublicSlug == "" || rotatedGroup.PublicSlug == group.PublicSlug {
		t.Fatalf("expected rotated slug, got %q from %q", rotatedGroup.PublicSlug, group.PublicSlug)
	}
	assertShareGroupSlug(t, rotatedGroup.PublicSlug)

	deleteRec := performRequest(t, router, http.MethodDelete, "/api/share-groups/"+strconv.FormatInt(group.ID, 10), "", cookie)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 deleting group, got %d (%s)", deleteRec.Code, deleteRec.Body.String())
	}
}

func TestIntervalsCanBeAssignedToShareGroup(t *testing.T) {
	db, router := newTestServer(t)

	cookie, _ := createTestUser(t, db, "sub-alice-001", "alice")
	group := createShareGroupForTest(t, router, cookie, "Trips")

	create := performRequest(t, router, http.MethodPost, "/api/intervals", `{
		"name":"Trip",
		"start_date":"2026-05-20",
		"end_date":"2026-05-21",
		"color":"#4f8ef7",
		"share_group_ids":[`+strconv.FormatInt(group.ID, 10)+`]
	}`, cookie)
	if create.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating interval, got %d (%s)", create.Code, create.Body.String())
	}

	var interval Interval
	if err := json.NewDecoder(create.Body).Decode(&interval); err != nil {
		t.Fatalf("decode interval: %v", err)
	}
	if len(interval.ShareGroups) != 1 || interval.ShareGroups[0].ID != group.ID {
		t.Fatalf("expected share group assignment, got %#v", interval)
	}
	if interval.ShareGroups[0].Name != "Trips" {
		t.Fatalf("expected share group name Trips, got %q", interval.ShareGroups[0].Name)
	}
}

func TestPublicShareGroupShowsOnlyAssignedIntervals(t *testing.T) {
	db, router := newTestServer(t)

	cookie, _ := createTestUser(t, db, "sub-alice-001", "alice")

	updateProfile := performRequest(t, router, http.MethodPut, "/api/me/profile", `{"display_name":"Alice Public"}`, cookie)
	if updateProfile.Code != http.StatusOK {
		t.Fatalf("expected 200 updating profile, got %d", updateProfile.Code)
	}

	group := createShareGroupForTest(t, router, cookie, "Trips")

	privateInterval := performRequest(t, router, http.MethodPost, "/api/intervals", `{
		"name":"Private Trip",
		"start_date":"2026-05-20",
		"end_date":"2026-05-21",
		"color":"#4f8ef7",
		"share_group_ids":[]
	}`, cookie)
	if privateInterval.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating private interval, got %d", privateInterval.Code)
	}

	publicInterval := performRequest(t, router, http.MethodPost, "/api/intervals", `{
		"name":"Public Trip",
		"start_date":"2026-06-20",
		"end_date":"2026-06-21",
		"color":"#e05c5c",
		"share_group_ids":[`+strconv.FormatInt(group.ID, 10)+`]
	}`, cookie)
	if publicInterval.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating grouped interval, got %d", publicInterval.Code)
	}

	rec := performRequest(t, router, http.MethodGet, "/api/public/groups/"+group.PublicSlug, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 loading public group, got %d (%s)", rec.Code, rec.Body.String())
	}

	var profile PublicShareGroup
	if err := json.NewDecoder(rec.Body).Decode(&profile); err != nil {
		t.Fatalf("decode public share group: %v", err)
	}

	if profile.Name != "Trips" {
		t.Fatalf("expected share group name Trips, got %q", profile.Name)
	}
	if profile.OwnerName != "Alice Public" {
		t.Fatalf("expected public owner display name, got %q", profile.OwnerName)
	}
	if len(profile.Intervals) != 1 || profile.Intervals[0].Name != "Public Trip" {
		t.Fatalf("expected only grouped interval, got %#v", profile.Intervals)
	}
}

func TestPublicShareGroupReturnsNotFoundWhenGroupIsEmpty(t *testing.T) {
	db, router := newTestServer(t)

	cookie, _ := createTestUser(t, db, "sub-alice-001", "alice")
	group := createShareGroupForTest(t, router, cookie, "Trips")

	rec := performRequest(t, router, http.MethodGet, "/api/public/groups/"+group.PublicSlug, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when group has no intervals, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestDeletingShareGroupMakesItsIntervalsPrivate(t *testing.T) {
	db, router := newTestServer(t)

	cookie, _ := createTestUser(t, db, "sub-alice-001", "alice")
	group := createShareGroupForTest(t, router, cookie, "Trips")

	create := performRequest(t, router, http.MethodPost, "/api/intervals", `{
		"name":"Trip",
		"start_date":"2026-05-20",
		"end_date":"2026-05-21",
		"color":"#4f8ef7",
		"share_group_ids":[`+strconv.FormatInt(group.ID, 10)+`]
	}`, cookie)
	if create.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating interval, got %d (%s)", create.Code, create.Body.String())
	}

	deleteRec := performRequest(t, router, http.MethodDelete, "/api/share-groups/"+strconv.FormatInt(group.ID, 10), "", cookie)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 deleting group, got %d (%s)", deleteRec.Code, deleteRec.Body.String())
	}

	rec := performRequest(t, router, http.MethodGet, "/api/intervals", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 listing intervals, got %d (%s)", rec.Code, rec.Body.String())
	}

	var intervals []Interval
	if err := json.NewDecoder(rec.Body).Decode(&intervals); err != nil {
		t.Fatalf("decode intervals: %v", err)
	}
	if len(intervals) != 1 {
		t.Fatalf("expected 1 interval, got %d", len(intervals))
	}
	if len(intervals[0].ShareGroups) != 0 {
		t.Fatalf("expected interval to have no groups after deletion, got %#v", intervals[0])
	}
}

func TestMoveIntervalReordersList(t *testing.T) {
	db, router := newTestServer(t)

	cookie, _ := createTestUser(t, db, "sub-alice-001", "alice")

	for _, name := range []string{"One", "Two", "Three"} {
		create := performRequest(t, router, http.MethodPost, "/api/intervals", `{
			"name":"`+name+`",
			"start_date":"2026-05-20",
			"end_date":"2026-05-21",
			"color":"#4f8ef7",
			"share_group_ids":[]
		}`, cookie)
		if create.Code != http.StatusCreated {
			t.Fatalf("expected 201 creating interval %q, got %d (%s)", name, create.Code, create.Body.String())
		}
	}

	move := performRequest(t, router, http.MethodPost, "/api/intervals/3/move", `{"direction":"up"}`, cookie)
	if move.Code != http.StatusOK {
		t.Fatalf("expected 200 moving interval, got %d (%s)", move.Code, move.Body.String())
	}

	rec := performRequest(t, router, http.MethodGet, "/api/intervals", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 listing intervals, got %d (%s)", rec.Code, rec.Body.String())
	}

	var intervals []Interval
	if err := json.NewDecoder(rec.Body).Decode(&intervals); err != nil {
		t.Fatalf("decode intervals: %v", err)
	}
	if got := []string{intervals[0].Name, intervals[1].Name, intervals[2].Name}; strings.Join(got, ",") != "One,Three,Two" {
		t.Fatalf("unexpected interval order: %#v", got)
	}
}

func TestMoveIntervalNormalizesLegacyZeroPositions(t *testing.T) {
	db, router := newTestServer(t)

	cookie, user := createTestUser(t, db, "sub-legacy-001", "legacy")

	for _, name := range []string{"First", "Second"} {
		create := performRequest(t, router, http.MethodPost, "/api/intervals", `{
			"name":"`+name+`",
			"start_date":"2026-05-20",
			"end_date":"2026-05-21",
			"color":"#4f8ef7",
			"share_group_ids":[]
		}`, cookie)
		if create.Code != http.StatusCreated {
			t.Fatalf("expected 201 creating interval %q, got %d (%s)", name, create.Code, create.Body.String())
		}
	}

	if _, err := db.Exec(`UPDATE intervals SET position=0 WHERE user_id=?`, user.ID); err != nil {
		t.Fatalf("reset positions to legacy zero values: %v", err)
	}

	move := performRequest(t, router, http.MethodPost, "/api/intervals/2/move", `{"direction":"up"}`, cookie)
	if move.Code != http.StatusOK {
		t.Fatalf("expected 200 moving interval with legacy positions, got %d (%s)", move.Code, move.Body.String())
	}

	rec := performRequest(t, router, http.MethodGet, "/api/intervals", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 listing intervals, got %d (%s)", rec.Code, rec.Body.String())
	}

	var intervals []Interval
	if err := json.NewDecoder(rec.Body).Decode(&intervals); err != nil {
		t.Fatalf("decode intervals: %v", err)
	}
	if got := []string{intervals[0].Name, intervals[1].Name}; strings.Join(got, ",") != "Second,First" {
		t.Fatalf("unexpected interval order after normalization: %#v", got)
	}
	if intervals[0].Position != 1 || intervals[1].Position != 2 {
		t.Fatalf("expected normalized positions 1 and 2, got %#v", intervals)
	}
}

func TestCurrentUserResponseDoesNotExposeEmail(t *testing.T) {
	db, router := newTestServer(t)

	cookie, _ := createTestUser(t, db, "sub-alice-001", "alice")
	rec := performRequest(t, router, http.MethodGet, "/api/me", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "alice@test.example") {
		t.Fatalf("expected email to stay private, got %q", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "\"email\"") {
		t.Fatalf("expected email field to stay private, got %q", rec.Body.String())
	}
}

func TestDeleteAccountRemovesUserIntervalsGroupsAndSession(t *testing.T) {
	db, router := newTestServer(t)

	cookie, _ := createTestUser(t, db, "sub-alice-001", "alice")
	group := createShareGroupForTest(t, router, cookie, "Trips")

	create := performRequest(t, router, http.MethodPost, "/api/intervals", `{
		"name":"Trip",
		"start_date":"2026-05-20",
		"end_date":"2026-05-21",
		"color":"#4f8ef7",
		"share_group_ids":[`+strconv.FormatInt(group.ID, 10)+`]
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

	publicGroup := performRequest(t, router, http.MethodGet, "/api/public/groups/"+group.PublicSlug, "")
	if publicGroup.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for deleted public group, got %d (%s)", publicGroup.Code, publicGroup.Body.String())
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

	var groupCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM share_groups`).Scan(&groupCount); err != nil {
		t.Fatalf("count share groups: %v", err)
	}
	if groupCount != 0 {
		t.Fatalf("expected no share groups after deletion, got %d", groupCount)
	}

	var sessionCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&sessionCount); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if sessionCount != 0 {
		t.Fatalf("expected no sessions after deletion, got %d", sessionCount)
	}
}

func TestPublicLookupRateLimitReturnsTooManyRequests(t *testing.T) {
	limiter := newAuthRateLimiter(nil)
	limiter.now = func() time.Time {
		return time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	}
	limiter.policies[authActionPublicLookup] = authRatePolicy{limit: 2, window: time.Minute}

	_, router := newTestServerWithHandler(t, &handler{authLimiter: limiter})

	for attempt := 1; attempt <= 2; attempt++ {
		rec := performRequestFromRemoteAddr(t, router, http.MethodGet, "/api/public/groups/nonexistent", "", "198.51.100.10:4567")
		if rec.Code == http.StatusTooManyRequests {
			t.Fatalf("attempt %d: unexpectedly rate-limited, got 429", attempt)
		}
	}

	blocked := performRequestFromRemoteAddr(t, router, http.MethodGet, "/api/public/groups/nonexistent", "", "198.51.100.10:4567")
	if blocked.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after limit, got %d (%s)", blocked.Code, blocked.Body.String())
	}
	if blocked.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header on rate-limited request")
	}
}
