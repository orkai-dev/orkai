-- Extensions installed at database initialization time (not in migrations).
-- These require PG server-level support and cannot be added at runtime
-- on standard PG builds.

-- pgcrypto: gen_random_uuid() — PG 13+ has it built-in, but kept for compatibility
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- pgmq: durable job queue (requires pgmq-enabled Postgres image)
CREATE EXTENSION IF NOT EXISTS pgmq;

-- pg_cron requires custom PG builds (tembo, supabase-postgres, etc).
-- CREATE EXTENSION IF NOT EXISTS pg_cron;
