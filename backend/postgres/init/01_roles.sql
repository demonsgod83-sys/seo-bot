-- ============================================================
--  01_roles.sql — runs once on first DB initialisation
--  Creates a least-privilege application role instead of
--  letting the service connect as the superuser.
-- ============================================================

-- Application user — only the permissions the service actually needs.
-- Password is set via env in docker-compose; change before production.
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'seobot_app') THEN
        CREATE ROLE seobot_app WITH LOGIN PASSWORD 'changeme_before_deploy';
    END IF;
END
$$;

-- Grant connect on the database
GRANT CONNECT ON DATABASE seobot_intake TO seobot_app;

-- Grant usage on the public schema
GRANT USAGE ON SCHEMA public TO seobot_app;

-- Grant CRUD on all current and future tables in public
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO seobot_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO seobot_app;

-- Grant usage on all sequences (needed for serial / uuid_generate_v4 defaults)
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO seobot_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO seobot_app;
