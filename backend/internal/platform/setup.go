package platform

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

var ErrSetupComplete = errors.New("setup already completed")

type SetupStore interface {
	SetupComplete(context.Context) (bool, error)
	CreateAdministrator(context.Context, string, string, string) error
}

type DatabaseSetupStore struct{ DB *sql.DB }

func (s DatabaseSetupStore) SetupComplete(ctx context.Context) (bool, error) {
	var complete bool
	err := s.DB.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM instance_setup) OR EXISTS (SELECT 1 FROM users WHERE is_admin)`).Scan(&complete)
	return complete, err
}

func (s DatabaseSetupStore) CreateAdministrator(ctx context.Context, id, email, hash string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Hold a transaction lock so simultaneous requests cannot claim setup twice.
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(734298106)`); err != nil {
		return err
	}
	var complete bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM instance_setup) OR EXISTS (SELECT 1 FROM users WHERE is_admin)`).Scan(&complete); err != nil {
		return err
	}
	if complete {
		return ErrSetupComplete
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO users(id,email,password_hash,is_admin) VALUES ($1,$2,$3,true)`, id, email, hash); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO instance_setup(id,administrator_id) VALUES (1,$1)`, id); err != nil {
		return err
	}
	return tx.Commit()
}

type SetupService struct {
	Store SetupStore
	Token string
}

func (s *SetupService) register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/setup/status", s.status)
	mux.HandleFunc("POST /api/setup", s.create)
}

func (s *SetupService) status(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	complete, err := s.Store.SetupComplete(ctx)
	if err != nil {
		writeJSON(w, 503, map[string]string{"error": "Unable to check setup. Try again."})
		return
	}
	writeJSON(w, 200, map[string]bool{"complete": complete, "enabled": len(s.Token) >= 32 && !complete})
}

func (s *SetupService) create(w http.ResponseWriter, r *http.Request) {
	// JSON plus same-origin checking prevents browser cross-site form submissions.
	if origin := r.Header.Get("Origin"); origin != "" {
		parsed, err := url.Parse(origin)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host != r.Host {
			writeJSON(w, 403, map[string]string{"error": "Setup must be submitted from this server's page."})
			return
		}
	}
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || contentType != "application/json" {
		writeJSON(w, 415, map[string]string{"error": "Expected a JSON request."})
		return
	}
	if len(s.Token) < 32 {
		writeJSON(w, 503, map[string]string{"error": "Set SETUP_TOKEN on the server before creating an account."})
		return
	}
	expected, supplied := sha256.Sum256([]byte(s.Token)), sha256.Sum256([]byte(r.Header.Get("X-Setup-Token")))
	if subtle.ConstantTimeCompare(expected[:], supplied[:]) != 1 {
		writeJSON(w, 403, map[string]string{"error": "Setup token is incorrect."})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	defer r.Body.Close()
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid setup request."})
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeJSON(w, 400, map[string]string{"error": "Invalid setup request."})
		return
	}
	email := strings.ToLower(strings.TrimSpace(input.Email))
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || len(email) > 254 || !strings.Contains(email, "@") {
		writeJSON(w, 400, map[string]string{"error": "Enter a valid email address."})
		return
	}
	if !utf8.ValidString(input.Password) || utf8.RuneCountInString(input.Password) < 12 || len(input.Password) > 1024 {
		writeJSON(w, 400, map[string]string{"error": "Use a password with at least 12 characters and no more than 1024 bytes."})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	complete, err := s.Store.SetupComplete(ctx)
	if err != nil {
		writeJSON(w, 503, map[string]string{"error": "Setup is temporarily unavailable."})
		return
	}
	if complete {
		writeJSON(w, 409, map[string]string{"error": "Administrator setup is already complete."})
		return
	}
	hash, err := hashPassword(input.Password)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Unable to create account."})
		return
	}
	id, err := newUUID()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Unable to create account."})
		return
	}
	err = s.Store.CreateAdministrator(ctx, id, email, hash)
	if errors.Is(err, ErrSetupComplete) {
		writeJSON(w, 409, map[string]string{"error": "Administrator setup is already complete."})
		return
	}
	if err != nil {
		writeJSON(w, 503, map[string]string{"error": "Unable to save account. Check the server and try again."})
		return
	}
	writeJSON(w, 201, map[string]bool{"complete": true})
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, 3, 64*1024, 1, 32)
	return fmt.Sprintf("$argon2id$v=19$m=65536,t=3,p=1$%s$%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash)), nil
}

func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}
