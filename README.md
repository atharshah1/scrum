# ⚡ scrumX — Developer-First Task System

> **Developer-first, offline-safe task system with Git-like conflict protection — faster than Jira.**

---

## Why scrumX?

Most task tools slow developers down when work leaves the browser.

scrumX focuses on two things first:

- **Speed** — fast CLI, keyboard-first TUI, instant web workflows
- **Safety** — offline-safe local state, visible sync status, conflict protection

This is **not** a Jira wrapper and **not** an AI-first product.
Jira support exists for **migration/import**, not as the product identity.

---

## Core product surfaces

### CLI

Git-like shortcuts for fast daily work:

```bash
sx i c "fix login bug p1 assign me #auth"
sx i l --status in_progress
sx i u <issue-id> --priority high
sx status
sx sync
sx pr create-issue
sx pr link <issue-id>
```

What matters here:

- short muscle-memory-friendly commands
- deterministic smart parsing from command text
- repo-aware issue creation
- visible sync / pending / conflict status

### TUI

Keyboard-first task triage with always-visible sync safety:

- board and issue browsing
- inline edits
- quick transitions
- notifications
- offline/sync/conflict status at the top

### Web

Built for fast issue work instead of heavy admin screens:

- instant issue list
- inline editing
- optimistic updates
- conflict-aware issue detail view
- migration flow for bringing work in from Jira

---

## What makes scrumX different?

### Offline-safe by default

CLI and TUI keep local state and queue changes safely when the network drops.

### Git-like conflict protection

Instead of silently overwriting work, scrumX preserves enough context to review and resolve conflicts deliberately.

### Deterministic smart workflows

scrumX can feel “smart” without requiring AI-first UX:

- parse issue metadata from typed commands
- infer labels from changed files
- create issues from branch and diff context
- default to the current user and current context when possible

### Migration without lock-in

Jira is treated as a source system for import and handoff — not the center of the product.

---

## Example workflows

### Create from terminal

```bash
sx i c "fix login bug p1 assign me #backend"
```

### Inspect trust state

```bash
sx status
```

Example output:

```text
✔ synced
mode: online
pending: 0
conflicts: 0
last sync: 2026-04-21T09:40:00Z
```

### Create an issue from git context

```bash
sx pr create-issue
```

This derives:

- title from the current branch
- description from repo and diff context
- labels from changed areas

### Link current branch work to an existing issue

```bash
sx pr link <issue-id>
```

---

## Product positioning

scrumX is built for teams that want:

- something **cheaper than Jira**
- something **more terminal-native than Linear**
- something **safer under offline + concurrent edits** than both

---

## Jira migration

Jira support is for migration flows such as:

- connect Jira
- preview issues, epics, and users
- map statuses and users
- import with progress feedback
- verify unmatched users or failed records

---

## Repository layout

```text
scrumX/cli       Fast developer CLI
scrumX/tui       Keyboard-first terminal UI
scrumX/frontend  Web app
scrumX/backend   API, workflows, integrations, migration surfaces
cmd/             Legacy Atlassian migration bridge
```

---

## Status

- scrumX is the **primary product surface** in this repository
- the root `scrum` CLI is a **legacy migration helper**
- current work is focused on **speed, trust, sync visibility, and conflict UX**

---

## Contributing

Areas that matter most:

- CLI ergonomics
- sync and conflict resolution
- web issue workflow polish
- migration UX
- TUI responsiveness
