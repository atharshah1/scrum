# ⚡ scrumX — Developer-First Task System

> A **Linear + Jira + CLI hybrid** with **offline-first sync**, **AI features**, and **Git-like conflict safety**.

---

## 🧠 Why scrumX?

Most tools:

* ❌ Lose data in conflicts
* ❌ Are slow & UI-heavy
* ❌ Ignore developer workflows

scrumX is built differently:

> 🔥 **CLI + TUI + Web unified system**
> 🔒 **Offline-first with conflict-safe sync**
> 🤖 **AI-powered productivity**

---

## ✨ Core Features

### ⚡ Developer-First UX

* CLI → fast, scriptable workflows
* TUI → keyboard-first interaction
* Web UI → clean, Linear-style interface

---

### 🔄 Offline-First Sync Engine

* Local-first architecture (CLI/TUI)
* Operation queue (create/update/delete)
* Push + Pull sync (delta-based)
* Retry + backoff handling

---

### 🧩 Conflict-Safe Merging (🔥 Differentiator)

* ❌ No data loss
* ✅ “Keep both” merge strategy
* 👥 Attribution (who changed what)

```text
Local change + Remote change → BOTH preserved
```

👉 Git-style safety for task management

---

### 🔍 JQL-like Query System

* `status=done AND assignee=me`
* Works across:

  * CLI
  * TUI
  * Web

---

### 🤖 AI Layer

* Auto-create issues from text
* Summarize tickets
* Suggest priority/labels
* Multi-provider (OpenAI / Gemini)
* Fallback + caching

---

### 📊 Insights Engine

* Bottleneck detection
* Stuck task alerts
* Team velocity
* Cycle time

---

### ⚡ Speed UX (Linear-style)

* Inline editing (no modals)
* Keyboard navigation
* Instant transitions
* Command palette (⌘K)

---

## 🏗️ Architecture

```text
            ┌──────────────┐
            │   CLI / TUI  │
            └──────┬───────┘
                   │
          ┌────────▼────────┐
          │   Local Store   │  ← Source of truth (offline-first)
          └────────┬────────┘
                   │
        ┌──────────▼──────────┐
        │     Sync Engine     │
        │  (Push / Pull / Q)  │
        └──────┬─────┬───────┘
               │     │
        ┌──────▼     ▼──────┐
        │   Backend API     │
        └────────┬──────────┘
                 │
        ┌────────▼────────┐
        │   Web Frontend  │
        └─────────────────┘
```

---

## 🔥 Conflict Handling (Key Innovation)

Instead of overwriting:

```text
User A → "Fix login bug"
User B → "Resolve auth issue"
```

scrumX stores:

```json
{
  "title": "Fix login bug",
  "conflicts": [
    {
      "field": "title",
      "values": [
        { "value": "Fix login bug", "user": "A" },
        { "value": "Resolve auth issue", "user": "B" }
      ]
    }
  ]
}
```

👉 No data is ever lost.

---

## 🖥️ CLI Examples

```bash
# Search issues (JQL-like)
scrumx issue search "status=done AND assignee=me"

# Save query
scrumx issue filter save "my-bugs" "assignee=me AND type=bug"

# Sync
scrumx sync now

# Conflicts
scrumx issue conflicts <id>
scrumx issue resolve <id>
```

---

## 📦 Tech Stack

* **Backend**: Go (Gin/Fiber style APIs)
* **Frontend**: Next.js + Tailwind
* **CLI/TUI**: Go
* **DB**: PostgreSQL + Local Store
* **Sync Engine**: Custom (queue + delta sync)
* **AI**: OpenAI + Gemini

---

## 🚀 What makes this special?

| Feature         | scrumX | Jira | Linear |
| --------------- | ------ | ---- | ------ |
| CLI support     | ✅      | ❌    | ❌      |
| Offline-first   | ✅      | ❌    | ⚠️     |
| Conflict safety | ✅      | ❌    | ❌      |
| AI integration  | ✅      | ⚠️   | ⚠️     |
| Dev-first UX    | ✅      | ❌    | ✅      |

---

## 🧠 Future Work

* Background sync daemon
* CRDT-based merging
* Web offline mode (IndexedDB)
* Multi-user collaboration testing

---

## 📌 Status

> 🚀 **Production-ready architecture (v1)**
> 🔧 Actively evolving

---

## 🤝 Contributing

PRs welcome — especially for:

* sync engine improvements
* UI polish
* performance

---

## 💬 Final Thought

> scrumX is not a Jira clone.
> It’s a **developer operating system for tasks.**
