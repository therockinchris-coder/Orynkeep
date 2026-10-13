# Orynkeep

A self-hosted home for files, simple documents, and photos.

## Current milestone: first-run administrator setup

Implemented: React/TypeScript web shell, Go HTTP API, PostgreSQL schema and
transactional migrations, writable local storage checks, Docker Compose,
health endpoints, first-run administrator creation, and CI definitions.

File operations, login, document editing, and photo browsing
are not implemented yet. The web shell labels these areas as planned.

## Start with Docker

Requirements: Docker Engine with Docker Compose v2, and internet access to fetch
images and build dependencies. Run commands from this directory.

1. Copy `.env.example` to `.env`.
2. Set `POSTGRES_PASSWORD` to a new random hexadecimal password. For example,
   generate one with `openssl rand -hex 32`. Paste it into `.env`; do not commit it.
3. Generate a second value with `openssl rand -hex 32` and set `SETUP_TOKEN` in
   `.env`. This token protects first-run setup; enter it in the setup form.
4. Start the services:

```sh
docker compose up --build --wait
```

Open http://localhost:8088. The system connection should report ready.
Create your administrator account using the setup form. Existing installations:
follow [the upgrade and setup instructions](docs/FIRST_RUN.md); preserve your
existing `.env` and data volumes.
The port binds to localhost; PostgreSQL and the API have no published ports.

```sh
curl --fail http://localhost:8088/api/health/ready
docker compose logs -f backend
docker compose down
```

`down` keeps the named database and storage volumes. **`down --volumes` deletes
both.** Changing `POSTGRES_PASSWORD` after the database volume exists does not
change its stored role password; rotate it in PostgreSQL as well.

## Local development

Requirements: Go 1.24+, Node.js 22.13+, and PostgreSQL 17. Configure a local
database separately, then in `backend/`:

```sh
export DATABASE_URL='postgres://orynkeep:YOUR_PASSWORD@localhost:5432/orynkeep?sslmode=disable'
export STORAGE_PATH='./data'
export SETUP_TOKEN='YOUR_SEPARATELY_GENERATED_SETUP_TOKEN'
go mod tidy
go run ./cmd/server
```

In another terminal, in `frontend/`:

```sh
npm install
npm run dev
```

Open the URL printed by Vite. `/api` is proxied to the backend on port 8080.
The database must be reachable before starting the backend. Startup fails if
the database, migrations, or storage checks fail.

## Checks

In `backend/`: `go vet ./...` and `go test -race ./...`.
Set `TEST_DATABASE_URL` to a **dedicated disposable test database** to run the
migration integration test; otherwise it skips. It checks repeatable migration
application, expected tables, and rejection of changed migration checksums.

In `frontend/`: `npm run build` checks TypeScript and builds the web app.
GitHub Actions also builds and starts the complete Compose stack and probes it.

The source package was generated in an environment without Docker or PostgreSQL;
a local Go toolchain was obtained to validate the backend.
See `docs/VALIDATION.md` for the checks actually performed. CI definitions are
provided but have not been run on GitHub.

## Data and migrations

- PostgreSQL stores users, file metadata, document JSON, and photo metadata.
- The storage volume will hold original file bytes under generated keys, rather
  than names supplied by users. Actual upload routes arrive in the next stage.
- SQL migrations are embedded in the backend and applied under a transaction
  and advisory lock. Never edit an already-applied migration: add the next
  numbered SQL file.
- Readiness checks PostgreSQL and the ability to create a temporary file in
  storage. Liveness only checks that the HTTP process responds.

## Back up and restore the foundation

Stop application writes before taking a coordinated database/storage backup.
At this milestone there are no user write endpoints, but use the same procedure
as the app grows. Protect `.env` separately because it contains a secret.

```sh
docker compose stop web backend
docker compose exec -T db pg_dump -U orynkeep -d orynkeep --format=custom > database.dump
docker compose run --rm --no-deps --user root --entrypoint tar backend -C /data -czf - . > storage.tar.gz
docker compose start backend web
```

To restore into a running but otherwise empty database/storage installation:

```sh
docker compose stop web backend
docker compose exec -T db pg_restore -U orynkeep -d orynkeep --clean --if-exists < database.dump
docker compose run --rm --no-deps -T --user root --entrypoint tar backend -C /data -xzf - < storage.tar.gz
docker compose run --rm --no-deps --user root --entrypoint chown backend -R 10001:10001 /data
docker compose start backend web
```

Restore only trusted archives. Use an empty storage volume so files created
after the backup cannot remain as stale leftovers. Test restoration on a
separate installation before relying on backups. These commands need validation
on a Docker-capable host.

## Next milestone

Login, sessions, logout, then an authenticated
upload → list → download flow. Access control tests must pass before user data
endpoints are added. Folders, rename, deletion, a simple document editor, and
the photo gallery follow within v0.1.

## License

Project license not yet selected. Choose and add an explicit license before
publishing this as an open source release. Dependency licenses are separate.
