package platform

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/argon2"
)

const testToken = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

type memorySetup struct {
	complete        bool
	failure         error
	id, email, hash string
	calls           int
}

func (s *memorySetup) SetupComplete(context.Context) (bool, error) { return s.complete, s.failure }
func (s *memorySetup) CreateAdministrator(_ context.Context, id, email, hash string) error {
	s.calls++
	if s.complete {
		return ErrSetupComplete
	}
	if s.failure != nil {
		return s.failure
	}
	s.id, s.email, s.hash, s.complete = id, email, hash, true
	return nil
}

func setupRequest(store SetupStore, token, supplied, body, origin, contentType string) *httptest.ResponseRecorder {
	handler := Handler(func(context.Context) error { return nil }, &SetupService{Store: store, Token: token})
	request := httptest.NewRequest(http.MethodPost, "http://example.com/api/setup", strings.NewReader(body))
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("X-Setup-Token", supplied)
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestSetupRejectsInvalidRequests(t *testing.T) {
	valid := `{"email":"admin@example.com","password":"a long test password"}`
	cases := []struct {
		name, token, supplied, body, origin, contentType string
		complete                                         bool
		status                                           int
	}{
		{"wrong token", testToken, "wrong", valid, "", "application/json", false, 403},
		{"disabled", "", testToken, valid, "", "application/json", false, 503},
		{"already complete", testToken, testToken, valid, "", "application/json", true, 409},
		{"cross origin", testToken, testToken, valid, "https://other.example", "application/json", false, 403},
		{"non JSON", testToken, testToken, valid, "", "text/plain", false, 415},
		{"display name email", testToken, testToken, `{"email":"Admin <admin@example.com>","password":"a long test password"}`, "", "application/json", false, 400},
		{"short password", testToken, testToken, `{"email":"admin@example.com","password":"short"}`, "", "application/json", false, 400},
		{"unknown field", testToken, testToken, `{"email":"admin@example.com","password":"a long test password","is_admin":true}`, "", "application/json", false, 400},
		{"trailing JSON", testToken, testToken, valid + `{}`, "", "application/json", false, 400},
		{"oversized body", testToken, testToken, strings.Repeat(" ", 17000) + valid, "", "application/json", false, 400},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			store := &memorySetup{complete: test.complete}
			response := setupRequest(store, test.token, test.supplied, test.body, test.origin, test.contentType)
			if response.Code != test.status {
				t.Fatalf("status %d, body %s", response.Code, response.Body)
			}
			if store.calls != 0 {
				t.Fatal("invalid request created an account")
			}
		})
	}
}

func TestSetupCreatesHashedAdministratorAndCloses(t *testing.T) {
	store := &memorySetup{}
	password := "a long test password"
	body := `{"email":" ADMIN@Example.COM ","password":"` + password + `"}`
	response := setupRequest(store, testToken, testToken, body, "http://example.com", "application/json")
	if response.Code != 201 {
		t.Fatalf("status %d: %s", response.Code, response.Body)
	}
	if store.email != "admin@example.com" || len(store.id) != 36 || store.calls != 1 {
		t.Fatalf("unexpected account: %+v", store)
	}
	parts := strings.Split(store.hash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" || parts[3] != "m=65536,t=3,p=1" {
		t.Fatal("incorrect password hash format")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) != 16 {
		t.Fatal("invalid salt")
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		t.Fatal(err)
	}
	expected := argon2.IDKey([]byte(password), salt, 3, 64*1024, 1, 32)
	if subtle.ConstantTimeCompare(hash, expected) != 1 {
		t.Fatal("stored hash does not verify")
	}
	if strings.Contains(response.Body.String(), password) || strings.Contains(response.Body.String(), store.hash) {
		t.Fatal("sensitive response data")
	}
	response = setupRequest(store, testToken, testToken, body, "", "application/json")
	if response.Code != 409 || store.calls != 1 {
		t.Fatal("setup was allowed twice")
	}
}

func TestSetupStatusFailureDoesNotExposeDetails(t *testing.T) {
	store := &memorySetup{failure: errors.New("private database details")}
	h := Handler(func(context.Context) error { return nil }, &SetupService{Store: store, Token: testToken})
	response := httptest.NewRecorder()
	h.ServeHTTP(response, httptest.NewRequest("GET", "/api/setup/status", nil))
	if response.Code != 503 || strings.Contains(response.Body.String(), "private") {
		t.Fatal("unsafe error response")
	}
}

func TestSetupConcurrencyIntegration(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not configured")
	}
	db, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	store := DatabaseSetupStore{DB: db}
	complete, err := store.SetupComplete(ctx)
	if err != nil || complete {
		t.Fatal("requires an empty dedicated test database")
	}
	ids := make([]string, 2)
	for i := range ids {
		ids[i], err = newUUID()
		if err != nil {
			t.Fatal(err)
		}
	}
	defer func() {
		_, err := db.ExecContext(context.Background(), `DELETE FROM instance_setup WHERE administrator_id IN ($1,$2)`, ids[0], ids[1])
		if err != nil {
			t.Error(err)
		}
		_, err = db.ExecContext(context.Background(), `DELETE FROM users WHERE id IN ($1,$2)`, ids[0], ids[1])
		if err != nil {
			t.Error(err)
		}
	}()
	start := make(chan struct{})
	results := make(chan error, 2)
	var group sync.WaitGroup
	for i, id := range ids {
		group.Add(1)
		go func(i int, id string) {
			defer group.Done()
			<-start
			email := []string{"first@example.com", "second@example.com"}[i]
			results <- store.CreateAdministrator(ctx, id, email, "integration-test-hash")
		}(i, id)
	}
	close(start)
	group.Wait()
	close(results)
	successes, conflicts := 0, 0
	for err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, ErrSetupComplete) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d", successes, conflicts)
	}
	complete, err = store.SetupComplete(ctx)
	if err != nil || !complete {
		t.Fatal("setup completion not persisted")
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM users WHERE is_admin`).Scan(&count); err != nil || count != 1 {
		t.Fatal("expected exactly one administrator")
	}
}
