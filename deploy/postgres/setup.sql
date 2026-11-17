-- Gegen die Datenbank "schmutzfink" als Superuser ausfuehren,
-- nachdem Rolle und Datenbank angelegt wurden (siehe setup.ps1 / setup.sh).

CREATE EXTENSION IF NOT EXISTS vector;
GRANT ALL ON SCHEMA public TO schmutzfink;
