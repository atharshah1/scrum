# scrumX quickstart

scrumX is the primary product surface in this repo: a developer-first task system with offline-safe local state, visible sync status, and conflict review.

## One-command local setup

From `./scrumX`:

```bash
make up
```

What it does:

- creates `./scrumX/infra/.env` from `.env.example` if missing
- starts Postgres, Redis, backend, and frontend with Docker Compose

Default URLs:

- web: `http://localhost:3000`
- backend health: `http://localhost:8080/health`
- api base: `http://localhost:8080/api/v1`

To stop the stack:

```bash
make down
```

## Proven workflow to exercise first

Use one vertical slice before exploring preview surfaces. This is the entire product promise:

CLI shortcut:

```bash
cd ..
go run ./scrumX/cli demo
```

1. Open the issue workspace in the web app.
2. Start the instant demo project or run `sx demo`.
3. Create an issue or open the starter issue.
4. Edit the issue immediately so the trust state shows pending work.
5. Go offline or force the CLI into offline capture, then confirm scrumX keeps your work protected.
6. Open the preloaded conflict issue or trigger a concurrent change from another surface.
7. Use **Resolve with recommended action** for the fastest safe path.
8. Confirm the final state is reflected everywhere.
9. Use the CLI trust surface to inspect offline/sync/conflict state:

```bash
cd .
go run ./scrumX/cli status
go run ./scrumX/cli sync status
go run ./scrumX/cli issue conflicts list
```

## Preview surfaces

The following areas are intentionally marked as preview surfaces until they are proven through the same end-to-end trust path:

- automation
- releases
- incidents

Use them for exploration, but treat issue create → sync → conflict → resolve as the default daily workflow.

Core trust promise to repeat in demos and docs:

- zero data loss
- safe offline
- safe sync
- safe conflict resolution
