CREATE TABLE IF NOT EXISTS users (
  id SERIAL PRIMARY KEY,
  email TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- جدول لتتبّع الـ JWT (للـ logout عبر إبطال التوكن)
CREATE TABLE IF NOT EXISTS auth_tokens (
  jti UUID PRIMARY KEY,
  user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at TIMESTAMPTZ NOT NULL,
  revoked_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS sessions (
  id BIGSERIAL PRIMARY KEY,
  user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  start_time TIMESTAMPTZ NOT NULL,
  end_time   TIMESTAMPTZ,
  duration_minutes INT
);

CREATE INDEX IF NOT EXISTS idx_sessions_user_date ON sessions (user_id, start_time);

-- حساب تجريبي (email: demo@demo.io / pass: demo123)
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM users WHERE email='demo@demo.io') THEN
    INSERT INTO users(email, password_hash)
    VALUES ('demo@demo.io', '$2a$10$k1QfQ8m2QW1Jt9y0S1V9N.4j5c9Tnq2E0n6tM3e6F3tFZ0S0h8UFe'); -- bcrypt('demo123')
  END IF;
END $$;
