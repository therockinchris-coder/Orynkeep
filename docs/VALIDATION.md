# Validation of the step 2 source package

## Startup correction

User-provided first-start logs showed database connection failures during
PostgreSQL initialization, followed by a successful backend startup. The initial
socket-based pg_isready health check could succeed against the temporary setup
server. Compose and CI now check TCP on 127.0.0.1 instead, so that temporary
socket-only server cannot mark the database ready. The correction has been
checked structurally; Docker runtime validation remains outstanding.

Checks performed on 2026-10-13:

| Check | Result |
| --- | --- |
| Frontend dependency installation | Passed; package-lock.json generated |
| TypeScript check and Vite production build | Passed (`npm run build`) |
| Go module resolution | Passed; go.sum generated |
| Go formatting | Applied with gofmt |
| Go static checks | Passed (`go vet ./...`) |
| Backend tests | Passed (`go test -v ./...`); database integration test skipped |
| Backend executable build | Passed (`go build ./cmd/server`) |
| Compose and GitHub Actions YAML parsing | Passed; structural parsing only |
| PostgreSQL migration execution | Not run locally: no PostgreSQL server available |
| Docker image build and full-stack startup | Not run locally: Docker unavailable |
| Backup and restore | Documented, not executed locally |
| Browser rendering and interaction | Not visually inspected |

Backend validation used Go 1.24.13 downloaded into the scratch workspace; its
archive checksum matched the official download checksum. Scratch toolchains,
dependency caches, compiled output, and node_modules are excluded from the source
archive. Both dependency lock/checksum files are included.

The CI workflow is configured to run database integration tests and Docker
startup checks on GitHub, but has not been executed there. Before treating this
foundation as verified end-to-end, run the workflow or equivalent checks on a
Docker-capable host.
