// TimeTrac API (Go + Postgres + JWT)
// ----------------------------------
// This single-file API provides:
// - Auth: register, login, logout (JWT w/ revoke list)
// - Time tracking: start/stop a session, list today's sessions, total for today
//
// Environment variables (with safe defaults for local dev):
//   DATABASE_URL  (e.g. postgres://app:apppass@localhost:5432/timetrac?sslmode=disable)
//   CORS_ORIGIN   (e.g. http://localhost:8100)
//   JWT_SECRET    (a long random string)
//   PORT          (default: 8080)
//
// Database tables used (minimal):
//   users(id SERIAL PK, email TEXT UNIQUE, password_hash TEXT, created_at TIMESTAMPTZ DEFAULT now())
//   auth_tokens(jti TEXT PK, user_id BIGINT, expires_at TIMESTAMPTZ, revoked_at TIMESTAMPTZ)
//   sessions(id SERIAL PK, user_id BIGINT, start_time TIMESTAMPTZ, end_time TIMESTAMPTZ NULL, duration_minutes INT NULL)
//
// Notes:
// - JWTs are stored in auth_tokens so we can revoke/expire them centrally.
// - Middleware cors() sets CORS headers; authOnly() validates JWT and injects user info.
// - This code aims to be easy to follow, not a framework.
//
// (Arabic quick tip) ملاحظة:
//  جميع الدوال مقسمة لأقسام واضحة مع تعليقات. إن أردت فصل الملفات لاحقاً
//  (handlers.go, middleware.go, auth.go…) العملية ستكون سهلة جداً.
package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
	"strconv"

	_ "github.com/lib/pq"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

//
// ────────────────────────────── Types & Models ──────────────────────────────
//

// Server holds shared dependencies and config.
type Server struct {
	db         *sql.DB       // Postgres connection
	origin     string        // Allowed CORS origin
	jwtSecret  []byte        // Secret key for signing JWTs
	tokenTTL   time.Duration // Token lifetime (e.g., 24h)
}

// Claims carried inside our JWT.
type claims struct {
	UserID int64     `json:"uid"` // application user id
	JTI    string    `json:"jti"` // token id (so we can revoke it)
	jwt.RegisteredClaims
}

// Requests for auth endpoints.
type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
type registerReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Session is the JSON we return to the app.
type Session struct {
	ID              int64      `json:"id"`
	UserID          int64      `json:"userId"`
	StartTime       time.Time  `json:"startTime"`
	EndTime         *time.Time `json:"endTime,omitempty"`
	DurationMinutes *int       `json:"durationMinutes,omitempty"`
}

//
// ──────────────────────────────── Bootstrap ─────────────────────────────────
//

func main() {
	// Read config from env (with dev-friendly defaults).
	dsn := getenv("DATABASE_URL", "postgres://app:apppass@localhost:5432/timetrac?sslmode=disable")
	origin := getenv("CORS_ORIGIN", "http://localhost:8100")
	secret := getenv("JWT_SECRET", "change_me_now")
	port := getenv("PORT", "8080")

	// Connect to Postgres.
	db, err := sql.Open("postgres", dsn)
	must(err)
	must(db.Ping())

	// Build the server object.
	s := &Server{
		db:        db,
		origin:    origin,
		jwtSecret: []byte(secret),
		tokenTTL:  24 * time.Hour,
	}

	// Plain net/http mux.
	mux := http.NewServeMux()

	// ── Auth endpoints
	mux.HandleFunc("/auth/register", s.cors(s.register))
	mux.HandleFunc("/auth/login",    s.cors(s.login))
	mux.HandleFunc("/auth/logout",   s.cors(s.authOnly(s.logout)))

	// ── Health check (simple readiness probe)
	mux.HandleFunc("/healthz", s.cors(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"status": "ok"})
	}))

	// ── Time tracking (protected)
	mux.HandleFunc("/api/time/start",       s.cors(s.authOnly(s.startSession)))
	mux.HandleFunc("/api/time/stop",        s.cors(s.authOnly(s.stopSession)))
	mux.HandleFunc("/api/time/sessions",    s.cors(s.authOnly(s.listSessions)))
	mux.HandleFunc("/api/time/total-today", s.cors(s.authOnly(s.totalToday)))

	log.Printf("API listening on :%s (CORS origin: %s)", port, origin)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

//
// ─────────────────────────── Helpers: misc small funcs ──────────────────────
//

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" { return v }
	return def
}

func must(err error) {
	if err != nil { log.Fatal(err) }
}

//
// ───────────────────────────── Middleware layer ─────────────────────────────
//

// cors wraps a handler and adds permissive CORS headers for a single origin.
// OPTIONS requests are short-circuited with 204.
func (s *Server) cors(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin",  s.origin)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	}
}

// authOnly verifies a Bearer JWT, ensures it exists and isn't revoked,
// and injects user identity into headers for downstream handlers.
//
// (Simple approach: we attach user info as headers. If you prefer context.Context,
// you can use a request clone with context values.)
func (s *Server) authOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, "missing token", http.StatusUnauthorized)
			return
		}
		tokenStr := strings.TrimPrefix(auth, "Bearer ")

		// Parse and validate JWT signature + claims.
		tkn, err := jwt.ParseWithClaims(tokenStr, &claims{}, func(token *jwt.Token) (interface{}, error) {
			return s.jwtSecret, nil
		})
		if err != nil || !tkn.Valid {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		cl, ok := tkn.Claims.(*claims)
		if !ok {
			http.Error(w, "invalid claims", http.StatusUnauthorized)
			return
		}

		// Server-side validation: token must exist, not expired, not revoked.
		var revokedAt sql.NullTime
		err = s.db.QueryRow(`
			SELECT revoked_at
			FROM auth_tokens
			WHERE jti=$1 AND user_id=$2 AND expires_at > NOW()
		`, cl.JTI, cl.UserID).Scan(&revokedAt)

		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "token not found/expired", http.StatusUnauthorized)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if revokedAt.Valid {
			http.Error(w, "token revoked", http.StatusUnauthorized)
			return
		}

		// Inject identity downstream (header-based for simplicity).
		r.Header.Set("X-UserID", int64ToStr(cl.UserID))
		r.Header.Set("X-JTI",    cl.JTI)

		next.ServeHTTP(w, r)
	}
}

//
// ─────────────────────────────── Auth Handlers ──────────────────────────────
//

// POST /auth/register
// Accepts {email, password}. Password must be >= 6 chars.
// Returns 201 on success; 409 if email already exists.
func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req registerReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if req.Email == "" || len(req.Password) < 6 {
		http.Error(w, "email/password invalid", http.StatusBadRequest)
		return
	}

	// Hash and store
	hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if _, err := s.db.Exec(
		`INSERT INTO users(email, password_hash) VALUES ($1,$2)`,
		req.Email, string(hash),
	); err != nil {
		http.Error(w, "email already used?", http.StatusConflict)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"message": "registered"})
}

// POST /auth/login
// Returns {token, user, exp}. Also stores the token (JTI) to allow revocation.
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req loginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	// Fetch user by email.
	var id int64
	var hash string
	err := s.db.QueryRow(
		`SELECT id, password_hash FROM users WHERE email=$1`,
		req.Email,
	).Scan(&id, &hash)

	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Verify password.
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	// Create a JWT and persist its JTI so we can revoke later.
	jti := uuid.New().String()
	exp := time.Now().Add(s.tokenTTL)

	cl := &claims{
		UserID: id,
		JTI:    jti,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	tkn := jwt.NewWithClaims(jwt.SigningMethodHS256, cl)
	signed, err := tkn.SignedString(s.jwtSecret)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := s.db.Exec(
		`INSERT INTO auth_tokens(jti, user_id, expires_at) VALUES ($1,$2,$3)`,
		jti, id, exp,
	); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"token": signed,
		"user":  map[string]any{"id": id, "email": req.Email},
		"exp":   exp,
	})
}

// POST /auth/logout
// Looks up current token JTI (from middleware) and marks it revoked.
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	jti := r.Header.Get("X-JTI")
	if jti == "" {
		http.Error(w, "no jti", http.StatusBadRequest)
		return
	}

	if _, err := s.db.Exec(
		`UPDATE auth_tokens SET revoked_at=NOW() WHERE jti=$1`,
		jti,
	); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

//
// ───────────────────────────── Time Tracking API ────────────────────────────
//

// POST /api/time/start
// Starts a new session if there is no open session for the user.
func (s *Server) startSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	uid, _ := strToInt64(r.Header.Get("X-UserID"))

	// Reject if there's already an open session.
	var cnt int
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM sessions WHERE user_id=$1 AND end_time IS NULL`,
		uid,
	).Scan(&cnt); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if cnt > 0 {
		http.Error(w, "session already running", http.StatusConflict)
		return
	}

	now := time.Now()
	var id int64
	if err := s.db.QueryRow(
		`INSERT INTO sessions(user_id, start_time) VALUES ($1,$2) RETURNING id`,
		uid, now,
	).Scan(&id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "startTime": now})
}

// POST /api/time/stop
// Stops the oldest open session and records duration (minutes).
func (s *Server) stopSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	uid, _ := strToInt64(r.Header.Get("X-UserID"))

	var id int64
	var start time.Time
	err := s.db.QueryRow(`
		SELECT id, start_time
		FROM sessions
		WHERE user_id=$1 AND end_time IS NULL
		ORDER BY start_time ASC
		LIMIT 1
	`, uid).Scan(&id, &start)

	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "no open session", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	now := time.Now()
	dur := int(now.Sub(start).Minutes())
	if dur < 0 { dur = 0 } // just in case

	if _, err := s.db.Exec(
		`UPDATE sessions SET end_time=$1, duration_minutes=$2 WHERE id=$3`,
		now, dur, id,
	); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id": id, "endTime": now, "durationMinutes": dur,
	})
}

// GET /api/time/sessions
// Returns today’s sessions for current user (ordered by start time).
func (s *Server) listSessions(w http.ResponseWriter, r *http.Request) {
	uid, _ := strToInt64(r.Header.Get("X-UserID"))

	rows, err := s.db.Query(`
		SELECT id, user_id, start_time, end_time, duration_minutes
		FROM sessions
		WHERE user_id=$1 AND start_time::date = CURRENT_DATE
		ORDER BY start_time ASC
	`, uid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var out []Session
	for rows.Next() {
		var sss Session
		if err := rows.Scan(&sss.ID, &sss.UserID, &sss.StartTime, &sss.EndTime, &sss.DurationMinutes); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		out = append(out, sss)
	}
	writeJSON(w, http.StatusOK, out)
}

// GET /api/time/total-today
// Returns {totalMinutes} of all finished sessions today.
func (s *Server) totalToday(w http.ResponseWriter, r *http.Request) {
	uid, _ := strToInt64(r.Header.Get("X-UserID"))

	var total sql.NullInt64
	if err := s.db.QueryRow(`
		SELECT COALESCE(SUM(duration_minutes), 0)
		FROM sessions
		WHERE user_id=$1 AND start_time::date = CURRENT_DATE
	`, uid).Scan(&total); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]int64{"totalMinutes": total.Int64})
}

//
// ───────────────────────────── JSON & tiny utils ────────────────────────────
//

// writeJSON sets JSON header, status code and writes the payload.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func int64ToStr(x int64) string { return strconv.FormatInt(x, 10) }
func strToInt64(s string) (int64, error) { return strconv.ParseInt(s, 10, 64) }
