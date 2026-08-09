package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"sync"
	"time"
)

// fakeProfileClient is an in-memory ProfileClient used only by tests, so
// they don't require a running profile-service.
type fakeProfileClient struct {
	mu     sync.Mutex
	bySub  map[string]Profile
	byName map[string]string // username -> sub
	nextID int64
}

func newFakeProfileClient() *fakeProfileClient {
	return &fakeProfileClient{
		bySub:  make(map[string]Profile),
		byName: make(map[string]string),
	}
}

func (f *fakeProfileClient) FindOrCreate(ctx context.Context, sub, firstNameHint, lastNameHint, displayNameHint, email string, emailVerified bool) (Profile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	firstName := strings.TrimSpace(firstNameHint)
	lastName := strings.TrimSpace(lastNameHint)

	if p, ok := f.bySub[sub]; ok {
		if email != "" && (email != p.Email || emailVerified != p.EmailVerified) {
			p.Email = email
			p.EmailVerified = emailVerified
			p.UpdatedAt = time.Now().UTC()
		}
		if firstName != "" {
			p.FirstName = firstName
		}
		if lastName != "" {
			p.LastName = lastName
		}
		f.bySub[sub] = p
		return p, nil
	}

	displayName := strings.TrimSpace(displayNameHint)
	if displayName == "" {
		displayName = strings.TrimSpace(firstName + " " + lastName)
	}

	buf := make([]byte, 8)
	rand.Read(buf)
	placeholder := fmt.Sprintf("pending-%x", buf)

	f.nextID++
	now := time.Now().UTC()
	p := Profile{
		ID:            f.nextID,
		OIDCSub:       sub,
		Username:      placeholder,
		FirstName:     firstName,
		LastName:      lastName,
		DisplayName:   displayName,
		Email:         email,
		EmailVerified: emailVerified,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	f.bySub[sub] = p
	f.byName[placeholder] = sub
	return p, nil
}

func (f *fakeProfileClient) GetBySub(ctx context.Context, sub string) (Profile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	p, ok := f.bySub[sub]
	if !ok {
		return Profile{}, ErrProfileNotFound
	}
	return p, nil
}

func (f *fakeProfileClient) GetByUsername(ctx context.Context, username string) (Profile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	sub, ok := f.byName[strings.ToLower(username)]
	if !ok {
		return Profile{}, ErrProfileNotFound
	}
	return f.bySub[sub], nil
}

func (f *fakeProfileClient) GetPublicBySub(ctx context.Context, sub string) (PublicProfile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	p, ok := f.bySub[sub]
	if !ok {
		return PublicProfile{}, ErrProfileNotFound
	}
	return PublicProfile{Username: p.Username, DisplayName: p.DisplayName}, nil
}

func (f *fakeProfileClient) Update(ctx context.Context, sub string, patch ProfilePatch) (Profile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	p, ok := f.bySub[sub]
	if !ok {
		return Profile{}, ErrProfileNotFound
	}

	if patch.Username != nil {
		newUsername := strings.ToLower(*patch.Username)
		if otherSub, taken := f.byName[newUsername]; taken && otherSub != sub {
			return Profile{}, ErrProfileUsernameUsed
		}
		delete(f.byName, p.Username)
		p.Username = newUsername
		p.UsernameSet = true
		f.byName[newUsername] = sub
	}
	if patch.FirstName != nil {
		p.FirstName = *patch.FirstName
	}
	if patch.LastName != nil {
		p.LastName = *patch.LastName
	}
	if patch.DisplayName != nil {
		p.DisplayName = *patch.DisplayName
	}
	if patch.AvatarURL != nil {
		p.AvatarURL = *patch.AvatarURL
	}
	p.UpdatedAt = time.Now().UTC()

	f.bySub[sub] = p
	return p, nil
}
