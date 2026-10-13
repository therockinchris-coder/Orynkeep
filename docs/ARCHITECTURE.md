# Foundation decisions

## Layout

`frontend/`: React + TypeScript + Vite. Nginx serves the built frontend and
proxies `/api` to Go, providing one origin for future cookie-based sessions.

`backend/`: Go standard-library HTTP server, pgx database driver via database/sql,
embedded PostgreSQL migrations. Features will be added as separate internal
packages when implemented, rather than speculative empty services.

`compose.yaml`: PostgreSQL, Go backend, and web service with named persistent
volumes. Only the web service is exposed, on localhost by default.

## API currently implemented

| Endpoint | Result |
| --- | --- |
| GET /api/health/live | 200 when the HTTP process responds |
| GET /api/health/ready | 200 if database and writable storage checks pass; otherwise 503 |
| GET /api/info | Project name, development version, and current stage |

The health endpoints expose no connection credentials, file names, or user data.
Unknown API paths return 404. There are no user-content API routes yet.

## Schema boundaries

The initial schema reserves stable UUID identities, per-owner files and
documents, versioned JSON document content, and a photo record referencing its
original file. Ownership is data only at this stage: request authorization will
be implemented before any content endpoint exists. Session, folder, sharing,
and background job tables will be added by future migrations.

Document revision numbers are intended for optimistic concurrency on autosave.
The actual document JSON format and editing library remain to be selected.

File contents and metadata cannot share a database transaction. The upload
milestone must stage bytes, validate size/checksum, atomically promote the file,
and reconcile failures or orphaned data. No file storage logic is implemented
in this foundation beyond provisioning and testing the writable directory.

## Dependency reproducibility

Direct frontend and Go dependencies are pinned. The frontend package lock is
included, and frontend CI/Docker use `npm ci`. Go module checksums are included
in `backend/go.sum`.
Container images currently use version tags rather than immutable digests.
Full dependency locking is required before the first distributed release.
