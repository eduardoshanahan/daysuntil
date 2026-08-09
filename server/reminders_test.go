package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"
)

type sentMail struct{ to, subject, body string }

type fakeMailer struct {
	mu   sync.Mutex
	sent []sentMail
	fail bool
}

func (f *fakeMailer) send(to, subject, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return errors.New("fake send failure")
	}
	f.sent = append(f.sent, sentMail{to, subject, body})
	return nil
}

func createIntervalForTest(t *testing.T, router http.Handler, cookie *http.Cookie, name string) Interval {
	t.Helper()

	rec := performRequest(t, router, http.MethodPost, "/api/intervals", `{
		"name":"`+name+`",
		"start_at":"2026-05-20T00:00:00Z",
		"end_at":"2026-05-21T00:00:00Z",
		"color":"#4f8ef7",
		"share_group_ids":[]
	}`, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create interval: expected 201, got %d (%s)", rec.Code, rec.Body.String())
	}
	var iv Interval
	if err := json.NewDecoder(rec.Body).Decode(&iv); err != nil {
		t.Fatalf("decode interval: %v", err)
	}
	return iv
}

func TestReminderCRUDLifecycle(t *testing.T) {
	db, router, profiles := newTestServer(t)
	_ = db

	cookie, _ := createTestUser(t, db, profiles, "sub-alice-001", "alice")
	interval := createIntervalForTest(t, router, cookie, "Trip")

	create := performRequest(t, router, http.MethodPost, "/api/intervals/"+strconv.FormatInt(interval.ID, 10)+"/reminders", `{
		"remind_at":"2026-05-19T09:00:00Z",
		"repeat_rule":"none",
		"message":"Don't forget!"
	}`, cookie)
	if create.Code != http.StatusCreated {
		t.Fatalf("create reminder: expected 201, got %d (%s)", create.Code, create.Body.String())
	}
	var reminder Reminder
	if err := json.NewDecoder(create.Body).Decode(&reminder); err != nil {
		t.Fatalf("decode reminder: %v", err)
	}
	if reminder.Message != "Don't forget!" {
		t.Fatalf("expected message to round-trip, got %q", reminder.Message)
	}

	list := performRequest(t, router, http.MethodGet, "/api/intervals/"+strconv.FormatInt(interval.ID, 10)+"/reminders", "", cookie)
	if list.Code != http.StatusOK {
		t.Fatalf("list reminders: expected 200, got %d (%s)", list.Code, list.Body.String())
	}
	var reminders []Reminder
	if err := json.NewDecoder(list.Body).Decode(&reminders); err != nil {
		t.Fatalf("decode reminders: %v", err)
	}
	if len(reminders) != 1 || reminders[0].ID != reminder.ID {
		t.Fatalf("expected the created reminder in the list, got %#v", reminders)
	}

	update := performRequest(t, router, http.MethodPut, "/api/reminders/"+strconv.FormatInt(reminder.ID, 10), `{
		"remind_at":"2026-05-19T10:00:00Z",
		"repeat_rule":"yearly",
		"message":"Updated"
	}`, cookie)
	if update.Code != http.StatusOK {
		t.Fatalf("update reminder: expected 200, got %d (%s)", update.Code, update.Body.String())
	}
	var updated Reminder
	json.NewDecoder(update.Body).Decode(&updated)
	if updated.Message != "Updated" || updated.RepeatRule != "yearly" {
		t.Fatalf("expected reminder to be updated, got %#v", updated)
	}

	del := performRequest(t, router, http.MethodDelete, "/api/reminders/"+strconv.FormatInt(reminder.ID, 10), "", cookie)
	if del.Code != http.StatusNoContent {
		t.Fatalf("delete reminder: expected 204, got %d (%s)", del.Code, del.Body.String())
	}

	listAfterDelete := performRequest(t, router, http.MethodGet, "/api/intervals/"+strconv.FormatInt(interval.ID, 10)+"/reminders", "", cookie)
	var afterDelete []Reminder
	json.NewDecoder(listAfterDelete.Body).Decode(&afterDelete)
	if len(afterDelete) != 0 {
		t.Fatalf("expected no reminders after delete, got %#v", afterDelete)
	}
}

func TestCreateReminderRejectsForeignInterval(t *testing.T) {
	db, router, profiles := newTestServer(t)

	aliceCookie, _ := createTestUser(t, db, profiles, "sub-alice-001", "alice")
	bobCookie, _ := createTestUser(t, db, profiles, "sub-bob-001", "bob")
	aliceInterval := createIntervalForTest(t, router, aliceCookie, "Alice Trip")

	rec := performRequest(t, router, http.MethodPost, "/api/intervals/"+strconv.FormatInt(aliceInterval.ID, 10)+"/reminders", `{
		"remind_at":"2026-05-19T09:00:00Z",
		"repeat_rule":"none"
	}`, bobCookie)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 creating a reminder on another user's interval, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestDispatchDueRemindersSendsAndMarksSentForOneTimeReminder(t *testing.T) {
	db, router, profiles := newTestServer(t)
	ctx := context.Background()

	cookie, _ := createTestUser(t, db, profiles, "sub-alice-001", "alice")
	if _, err := profiles.FindOrCreate(ctx, "sub-alice-001", "", "", "alice", "alice@example.com", true); err != nil {
		t.Fatalf("set test email: %v", err)
	}
	interval := createIntervalForTest(t, router, cookie, "Trip")

	create := performRequest(t, router, http.MethodPost, "/api/intervals/"+strconv.FormatInt(interval.ID, 10)+"/reminders", `{
		"remind_at":"2020-01-01T00:00:00Z",
		"repeat_rule":"none",
		"message":"Pack your bags"
	}`, cookie)
	if create.Code != http.StatusCreated {
		t.Fatalf("create reminder: expected 201, got %d (%s)", create.Code, create.Body.String())
	}
	var reminder Reminder
	json.NewDecoder(create.Body).Decode(&reminder)

	fm := &fakeMailer{}
	if err := dispatchDueReminders(ctx, db, fm, profiles); err != nil {
		t.Fatalf("dispatch due reminders: %v", err)
	}

	if len(fm.sent) != 1 {
		t.Fatalf("expected 1 email sent, got %d: %#v", len(fm.sent), fm.sent)
	}
	if fm.sent[0].to != "alice@example.com" {
		t.Fatalf("expected email sent to alice's address, got %q", fm.sent[0].to)
	}

	updated, err := reminderByID(db, 1, reminder.ID)
	if err != nil {
		t.Fatalf("reload reminder: %v", err)
	}
	if updated.SentAt == nil {
		t.Fatal("expected sent_at to be set after dispatch")
	}

	// A second dispatch pass must not resend a one-time reminder.
	if err := dispatchDueReminders(ctx, db, fm, profiles); err != nil {
		t.Fatalf("second dispatch: %v", err)
	}
	if len(fm.sent) != 1 {
		t.Fatalf("expected no additional email on second dispatch, got %d total", len(fm.sent))
	}
}

func TestDispatchDueRemindersAdvancesRecurringReminder(t *testing.T) {
	db, router, profiles := newTestServer(t)
	ctx := context.Background()

	cookie, _ := createTestUser(t, db, profiles, "sub-alice-001", "alice")
	if _, err := profiles.FindOrCreate(ctx, "sub-alice-001", "", "", "alice", "alice@example.com", true); err != nil {
		t.Fatalf("set test email: %v", err)
	}
	interval := createIntervalForTest(t, router, cookie, "Birthday")

	create := performRequest(t, router, http.MethodPost, "/api/intervals/"+strconv.FormatInt(interval.ID, 10)+"/reminders", `{
		"remind_at":"2020-01-01T00:00:00Z",
		"repeat_rule":"yearly"
	}`, cookie)
	var reminder Reminder
	json.NewDecoder(create.Body).Decode(&reminder)

	fm := &fakeMailer{}
	if err := dispatchDueReminders(ctx, db, fm, profiles); err != nil {
		t.Fatalf("dispatch due reminders: %v", err)
	}
	if len(fm.sent) != 1 {
		t.Fatalf("expected 1 email sent, got %d", len(fm.sent))
	}

	updated, err := reminderByID(db, 1, reminder.ID)
	if err != nil {
		t.Fatalf("reload reminder: %v", err)
	}
	if !updated.RemindAt.After(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected remind_at to be rolled forward past 2026, got %v", updated.RemindAt)
	}
	if updated.SentAt == nil {
		t.Fatal("expected sent_at audit timestamp to be set")
	}
}

func TestDispatchDueRemindersSkipsOwnerWithNoEmail(t *testing.T) {
	db, router, profiles := newTestServer(t)
	ctx := context.Background()

	cookie, _ := createTestUser(t, db, profiles, "sub-alice-001", "alice")
	interval := createIntervalForTest(t, router, cookie, "Trip")

	performRequest(t, router, http.MethodPost, "/api/intervals/"+strconv.FormatInt(interval.ID, 10)+"/reminders", `{
		"remind_at":"2020-01-01T00:00:00Z",
		"repeat_rule":"none"
	}`, cookie)

	fm := &fakeMailer{}
	if err := dispatchDueReminders(ctx, db, fm, profiles); err != nil {
		t.Fatalf("dispatch due reminders: %v", err)
	}
	if len(fm.sent) != 0 {
		t.Fatalf("expected no email sent when owner has no email on file, got %d", len(fm.sent))
	}
}

func TestDispatchDueRemindersSkipsOwnerWithUnverifiedEmail(t *testing.T) {
	db, router, profiles := newTestServer(t)
	ctx := context.Background()

	cookie, _ := createTestUser(t, db, profiles, "sub-alice-001", "alice")
	// An email is on file, but the IdP has not confirmed it belongs to
	// this user — must not be trusted as a reminder destination.
	if _, err := profiles.FindOrCreate(ctx, "sub-alice-001", "", "", "alice", "unverified@example.com", false); err != nil {
		t.Fatalf("set test email: %v", err)
	}
	interval := createIntervalForTest(t, router, cookie, "Trip")

	performRequest(t, router, http.MethodPost, "/api/intervals/"+strconv.FormatInt(interval.ID, 10)+"/reminders", `{
		"remind_at":"2020-01-01T00:00:00Z",
		"repeat_rule":"none"
	}`, cookie)

	fm := &fakeMailer{}
	if err := dispatchDueReminders(ctx, db, fm, profiles); err != nil {
		t.Fatalf("dispatch due reminders: %v", err)
	}
	if len(fm.sent) != 0 {
		t.Fatalf("expected no email sent when the owner's email is unverified, got %d: %#v", len(fm.sent), fm.sent)
	}
}
