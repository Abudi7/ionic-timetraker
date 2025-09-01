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

type Server struct {
	db         *sql.DB
	origin     string
	jwtSecret  []byte
	tokenTTL   time.Duration
}

type claims struct {
	UserID int64     `json:"uid"`
	JTI    string    `json:"jti"`
	jwt.RegisteredClaims
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type registerReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type Session struct {
	ID              int64      `json:"id"`
	UserID          int64      `json:"userId"`
	StartTime       time.Time  `json:"startTime"`
	EndTime         *time.Time `json:"endTime,omitempty"`
	DurationMinutes *int       `json:"durationMinutes,omitempty"`
}

func main() {
	dsn := getenv("DATABASE_URL", "postgres://app:apppass@localhost:5432/timetrac?sslmode=disable")
	origin := getenv("CORS_ORIGIN", "http://localhost:8100")
	secret := getenv("JWT_SECRET", "change_me_now")
	port := getenv("PORT", "8080")

	db, err := sql.Open("postgres", dsn)
	must(err)
	must(db.Ping())

	s := &Server{
		db:        db,
		origin:    origin,
		jwtSecret: []byte(secret),
		tokenTTL:  24 * time.Hour,
	}

	mux := http.NewServeMux()

	// Auth
	mux.HandleFunc("/auth/register", s.cors(s.register))
	mux.HandleFunc("/auth/login", s.cors(s.login))
	mux.HandleFunc("/auth/logout", s.cors(s.authOnly(s.logout)))

	// Health
	mux.HandleFunc("/healthz", s.cors(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"status": "ok"})
	}))

	// Time tracking (محمي)
	mux.HandleFunc("/api/time/start", s.cors(s.authOnly(s.startSession)))
	mux.HandleFunc("/api/time/stop", s.cors(s.authOnly(s.stopSession)))
	mux.HandleFunc("/api/time/sessions", s.cors(s.authOnly(s.listSessions)))
	mux.HandleFunc("/api/time/total-today", s.cors(s.authOnly(s.totalToday)))

	log.Printf("API on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func getenv(k, def string) string { if v := os.Getenv(k); v != "" { return v }; return def }
func must(err error) { if err != nil { log.Fatal(err) } }

// ====== CORS & Middleware ======
func (s *Server) cors(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", s.origin)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions { w.WriteHeader(http.StatusNoContent); return }
		next.ServeHTTP(w, r)
	}
}
func (s *Server) authOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, "missing token", 401); return
		}
		tokenStr := strings.TrimPrefix(auth, "Bearer ")

		tkn, err := jwt.ParseWithClaims(tokenStr, &claims{}, func(token *jwt.Token) (interface{}, error) {
			return s.jwtSecret, nil
		})
		if err != nil || !tkn.Valid {
			http.Error(w, "invalid token", 401); return
		}
		cl, ok := tkn.Claims.(*claims)
		if !ok {
			http.Error(w, "invalid claims", 401); return
		}

		// تحقق من عدم إلغاء/انتهاء التوكن في DB
		var revokedAt sql.NullTime
		err = s.db.QueryRow(`SELECT revoked_at FROM auth_tokens WHERE jti=$1 AND user_id=$2 AND expires_at > NOW()`, cl.JTI, cl.UserID).Scan(&revokedAt)
		if errors.Is(err, sql.ErrNoRows) { http.Error(w, "token not found/expired", 401); return }
		if err != nil { http.Error(w, err.Error(), 500); return }
		if revokedAt.Valid { http.Error(w, "token revoked", 401); return }

		// inject context via Request clone (بسيطة بدون context package لتعقيد أقل)
		r.Header.Set("X-UserID", int64ToStr(cl.UserID))
		r.Header.Set("X-JTI", cl.JTI)

		next.ServeHTTP(w, r)
	}
}

// ====== Auth Handlers ======
func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", 405); return }
	var req registerReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { http.Error(w, "bad json", 400); return }

	if req.Email == "" || len(req.Password) < 6 {
		http.Error(w, "email/password invalid", 400); return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)

	_, err := s.db.Exec(`INSERT INTO users(email, password_hash) VALUES ($1,$2)`, req.Email, string(hash))
	if err != nil {
		http.Error(w, "email already used?", 409); return
	}
	writeJSON(w, 201, map[string]string{"message":"registered"})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", 405); return }
	var req loginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { http.Error(w, "bad json", 400); return }

	var id int64
	var hash string
	err := s.db.QueryRow(`SELECT id, password_hash FROM users WHERE email=$1`, req.Email).Scan(&id, &hash)
	if errors.Is(err, sql.ErrNoRows) { http.Error(w, "invalid credentials", 401); return }
	if err != nil { http.Error(w, err.Error(), 500); return }

	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
		http.Error(w, "invalid credentials", 401); return
	}

	// اصنع JWT + سجّله في auth_tokens
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
	if err != nil { http.Error(w, err.Error(), 500); return }

	_, err = s.db.Exec(`INSERT INTO auth_tokens(jti, user_id, expires_at) VALUES ($1,$2,$3)`, jti, id, exp)
	if err != nil { http.Error(w, err.Error(), 500); return }

	writeJSON(w, 200, map[string]any{
		"token": signed,
		"user":  map[string]any{"id": id, "email": req.Email},
		"exp":   exp,
	})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", 405); return }
	jti := r.Header.Get("X-JTI")
	if jti == "" { http.Error(w, "no jti", 400); return }

	_, err := s.db.Exec(`UPDATE auth_tokens SET revoked_at=NOW() WHERE jti=$1`, jti)
	if err != nil { http.Error(w, err.Error(), 500); return }
	writeJSON(w, 200, map[string]string{"message":"logged out"})
}

// ====== Time Tracking ======
func (s *Server) startSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", 405); return }
	uid, _ := strToInt64(r.Header.Get("X-UserID"))

	// هل يوجد جلسة مفتوحة؟
	var cnt int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE user_id=$1 AND end_time IS NULL`, uid).Scan(&cnt); err != nil {
		http.Error(w, err.Error(), 500); return
	}
	if cnt > 0 { http.Error(w, "session already running", 409); return }

	now := time.Now()
	var id int64
	if err := s.db.QueryRow(`INSERT INTO sessions(user_id, start_time) VALUES ($1,$2) RETURNING id`, uid, now).Scan(&id); err != nil {
		http.Error(w, err.Error(), 500); return
	}
	writeJSON(w, 201, map[string]any{"id": id, "startTime": now})
}

func (s *Server) stopSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", 405); return }
	uid, _ := strToInt64(r.Header.Get("X-UserID"))

	var id int64
	var start time.Time
	err := s.db.QueryRow(`
		SELECT id, start_time FROM sessions
		WHERE user_id=$1 AND end_time IS NULL
		ORDER BY start_time ASC LIMIT 1
	`, uid).Scan(&id, &start)
	if errors.Is(err, sql.ErrNoRows) { http.Error(w, "no open session", 404); return }
	if err != nil { http.Error(w, err.Error(), 500); return }

	now := time.Now()
	dur := int(now.Sub(start).Minutes())
	if dur < 0 { dur = 0 }

	if _, err := s.db.Exec(`UPDATE sessions SET end_time=$1, duration_minutes=$2 WHERE id=$3`, now, dur, id); err != nil {
		http.Error(w, err.Error(), 500); return
	}
	writeJSON(w, 200, map[string]any{"id": id, "endTime": now, "durationMinutes": dur})
}

func (s *Server) listSessions(w http.ResponseWriter, r *http.Request) {
	uid, _ := strToInt64(r.Header.Get("X-UserID"))
	rows, err := s.db.Query(`
		SELECT id, user_id, start_time, end_time, duration_minutes
		FROM sessions
		WHERE user_id=$1 AND start_time::date=CURRENT_DATE
		ORDER BY start_time ASC
	`, uid)
	if err != nil { http.Error(w, err.Error(), 500); return }
	defer rows.Close()

	var out []Session
	for rows.Next() {
		var sss Session
		if err := rows.Scan(&sss.ID, &sss.UserID, &sss.StartTime, &sss.EndTime, &sss.DurationMinutes); err != nil {
			http.Error(w, err.Error(), 500); return
		}
		out = append(out, sss)
	}
	writeJSON(w, 200, out)
}

func (s *Server) totalToday(w http.ResponseWriter, r *http.Request) {
	uid, _ := strToInt64(r.Header.Get("X-UserID"))
	var total sql.NullInt64
	if err := s.db.QueryRow(`
		SELECT COALESCE(SUM(duration_minutes),0)
		FROM sessions
		WHERE user_id=$1 AND start_time::date=CURRENT_DATE
	`, uid).Scan(&total); err != nil {
		http.Error(w, err.Error(), 500); return
	}
	writeJSON(w, 200, map[string]int64{"totalMinutes": total.Int64})
}

// ====== Helpers ======
// ====== Helpers ======
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func int64ToStr(x int64) string {
	return strconv.FormatInt(x, 10)
}

func strToInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}



