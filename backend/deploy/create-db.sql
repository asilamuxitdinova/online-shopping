-- Creates (or updates) the application role and database. Idempotent.
-- Run as a PostgreSQL superuser with psql variables db_user, db_password and db_name,
-- e.g. via `make db-create`.
\set ON_ERROR_STOP on

SELECT format('CREATE ROLE %I LOGIN', :'db_user')
WHERE NOT EXISTS (SELECT FROM pg_roles WHERE rolname = :'db_user') \gexec

SELECT format('ALTER ROLE %I WITH LOGIN PASSWORD %L', :'db_user', :'db_password') \gexec

SELECT format('CREATE DATABASE %I OWNER %I', :'db_name', :'db_user')
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = :'db_name') \gexec
