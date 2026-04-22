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

## CLI command system

Binary and alias:

```bash
scrumx
alias sx=scrumx
```

Structured mapping:

| Long | Short |
| --- | --- |
| issues | i |
| project | p |
| sprint | s |
| create | c |
| list | l |
| update | u |

### Long commands

```bash
scrumx issues create "fix login bug p1 assign me #auth"
scrumx issues list --status open
scrumx project create CORE "Core platform"
scrumx sprint start <sprint-id>
```

### Short commands

```bash
sx i c "fix login bug p1 assign me #auth"
sx i l --status open
sx p c CORE "Core platform"
sx s s <sprint-id>
```

### Mixed commands

```bash
scrumx i -c "fix login bug p1 assign me #auth"
scrumx i -l --status open
scrumx p -c CORE "Core platform"
scrumx s -s <sprint-id>
```

### Shell completion

```bash
scrumx completion bash > /etc/bash_completion.d/scrumx
scrumx completion zsh > ~/.zsh/completions/_scrumx
```

To stop the stack:

```bash
make down
```

## ⚠️ Do immediately after merge (don't skip)

Run the instant demo first:

```bash
sx demo
```

That one command is the full proof loop for demo purposes:

1. create a demo project
2. create starter work
3. edit the issue so pending work is visible
4. open the preloaded conflict
5. resolve it with the recommended action
6. confirm the final safe state everywhere

## Proven workflow to exercise first

Use one vertical slice before exploring preview surfaces. This is the entire product promise:

CLI shortcut:

```bash
cd ..
go run ./scrumX/cli demo
sx demo
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
go run ./scrumX/cli status
go run ./scrumX/cli sync status
go run ./scrumX/cli issues conflicts list
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
