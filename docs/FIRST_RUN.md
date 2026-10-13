# First-run administrator setup

## Upgrade your existing installation

1. Back up your database and storage using the procedure in README.md.
2. Replace source files with this release **inside the existing Orynkeep project
   folder**. Preserve `.env`, your database password, and the existing volumes.
   Keep the Compose project name the same so it reuses your data volumes.
3. Generate a separate setup token on your server:

```sh
openssl rand -hex 32
```

4. Add its output to your existing `.env`:

```dotenv
SETUP_TOKEN=PASTE_THE_GENERATED_TOKEN_HERE
```

The real token must be at least 32 characters; the generation command produces
64 hexadecimal characters. Do not reuse your database password as the token.
No token is shipped in the archive.

5. If you access Orynkeep from another computer on your local network, also add:

```dotenv
ORYNKEEP_BIND_ADDRESS=0.0.0.0
```

The default remains localhost. The new Compose configuration supports this
setting so you do not need to edit its ports section for LAN access.

6. Rebuild and start:

```sh
docker compose up -d --build --wait
```

The backend applies migration 002 to your existing database. No database reset
is required. Do not use `docker compose down --volumes`.

## Create the account

Open http://localhost:8088 on the server, or http://YOUR_SERVER_IP:8088 from
another computer on your local network when LAN access is enabled.

Enter the setup token, administrator email, password, and confirmation.
The password must contain at least 12 characters and no more than 1024 UTF-8
bytes. Email addresses are trimmed and stored in lowercase.

Click **Create administrator account**. On success, setup closes permanently
for this database. Refreshing or restarting containers does not reopen it.

The administrator account is stored, but this milestone does not implement
login, sessions, password reset, or user content endpoints. The page explicitly
shows that login is the next step; account creation does not authenticate you.

After success, remove `SETUP_TOKEN` from `.env` and run
`docker compose up -d --wait` to remove it from the backend's environment.
Setup remains closed because completion is stored in PostgreSQL.

Use the setup form over a trusted local connection, or configure HTTPS through
your own reverse proxy. The bundled HTTP setup does not encrypt credentials
in transit. Do not expose this development release directly to the internet.

## Failure states

- **Enable first-run setup:** SETUP_TOKEN is absent or shorter than 32 characters.
  Set a token and recreate the backend with `docker compose up -d --wait`.
- **Incorrect setup token:** use the exact token configured for the backend.
- **Already complete:** an administrator has already been created. Setup never
  replaces that account or resets its password.
- **Timed-out request:** reload first to check whether creation completed.
- **Server unavailable:** inspect `docker compose logs --tail=80 backend db`.
  Do not post your `.env` or token in support logs.

## Implemented API

| Route | Behavior |
| --- | --- |
| GET /api/setup/status | Reports completion and whether setup is enabled; no account details |
| POST /api/setup | Requires JSON and X-Setup-Token; creates the administrator once |

Creation hashes passwords with salted Argon2id. A database transaction and
advisory lock serialize simultaneous claims. The persistent singleton setup
record prevents reopening setup even if the administrator flag later changes.
Migration 002 closes setup for databases that already have an administrator.
