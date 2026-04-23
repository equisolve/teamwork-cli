# teamwork-cli API Coverage Analysis

Snapshot of what the CLI exposes today vs. what Teamwork's public API offers.
Based on [apidocs.teamwork.com](https://apidocs.teamwork.com/docs/teamwork)
(v1 + v3), April 2026.

## TL;DR

**Built:** 6 resources, 14 subcommands, roughly **10%** of the API surface.
**Gap:** no write paths outside of time logging and task-complete; no
messages/files/milestones/invoices/timers at all; no search, no activity feed,
no webhooks, no v3 (sideloading, sparse fields, project updates, metrics).

| Area | CLI status | Notes |
|---|---|---|
| Me | ✅ complete | single endpoint |
| Projects | 🟡 read-only, list/show | no create/update/archive/owner/rates/templates |
| Tasks | 🟡 list/show/complete | no create/update/delete/uncomplete/reorder/subtasks/deps |
| Time entries | 🟡 list + log | no update/delete; **no timers at all** |
| People | 🟡 list | no show/create/update/delete |
| Companies | 🟡 list | no show/create/update/delete |
| Task lists | ❌ missing | |
| Milestones | ❌ missing | |
| Messages | ❌ missing | |
| Files / Versions | ❌ missing | |
| Notebooks | ❌ missing | |
| Links | ❌ missing | |
| Comments | ❌ missing | (applies to tasks, messages, notebooks, links, milestones, files) |
| Invoices | ❌ missing | |
| Expenses | ❌ missing | |
| Risks | ❌ missing | |
| Workload | ❌ missing | capacity/utilization across users |
| Calendar events | ❌ missing | |
| Categories | ❌ missing | project/message/file/link categories |
| Tags | ❌ missing | list + assign/unassign to any resource |
| Activity feed | ❌ missing | cross-project recent activity |
| Search | ❌ missing | cross-resource search |
| Project templates | ❌ missing | |
| Portfolio boards | ❌ missing | |
| Webhooks | ❌ missing | create/delete subscriptions |
| Custom fields | ❌ missing | reading + writing values |
| Project updates | ❌ missing | v3-only status updates feature |
| Project metrics | ❌ missing | v3 — open invoices, project counts, etc. |

---

## Resource-by-resource detail

### Me
**Covered.** `GET /me.json`. Nothing else here.

### Projects (`teamwork projects`)
**Have:** `list`, `show`
**Missing v1 endpoints:**
- `POST /projects.json` — **create project**
- `PUT /projects/{id}.json` — **update project**
- `DELETE /projects/{id}.json` — **delete/archive**
- `PUT /projects/{id}/star.json` / `unstar.json`
- `GET/PUT /projects/{id}/rates.json` — project rates
- `PUT /projects/{id}/owner.json` — set project owner
- `GET /projects/{id}/people.json` (we have `people list --project` which hits this)
- `POST /projects/{id}/people.json` — add people to a project
- `GET/POST /projectCategories.json` — project categories
- `POST /projects/template.json` — create from template
- `GET /projects/templates.json` — list templates
**Missing v3:**
- Project updates, project metrics, better filtering via sparse fields

### Tasks (`teamwork tasks`)
**Have:** `list`, `show`, `complete`
**Missing v1 endpoints:**
- `POST /tasklists/{id}/tasks.json` — **create task** (requires a tasklist ID)
- `PUT /tasks/{id}.json` — **update task** (content, priority, due date, assignee, estimated minutes, …)
- `DELETE /tasks/{id}.json` — **delete task**
- `PUT /tasks/{id}/uncomplete.json` — reopen
- `PUT /tasks/{id}/reorder.json`
- `GET /tasks/{id}/subtasks.json`
- `POST /tasks/{id}/quickadd.json` — bulk-add subtasks with `\n` or `~|~` separators
- `GET/POST/DELETE` task dependencies
- `GET/POST` followers
- `GET /completedTasks.json` — last month's completed
- Task reminders
**Missing v3:** `GET /projects/api/v3/tasks.json` with sideloading — one of the more useful v3 improvements.

### Task Lists — **entirely missing**
- `GET /projects/{id}/tasklists.json` — list task lists for a project
- `POST /projects/{id}/tasklists.json` — create
- `PUT /tasklists/{id}.json` — rename/reorder
- `DELETE /tasklists/{id}.json`

Important because **creating a task requires a tasklist ID**. Without tasklist
commands, the `tasks create` command has no ergonomic way to discover the
target.

### Time tracking (`teamwork time`)
**Have:** `list`, `log` (POST)
**Missing v1:**
- `PUT /time_entries/{id}.json` — update an entry
- `DELETE /time_entries/{id}.json`
- `GET /time_entries/total.json` — totals by filter
- `GET /timers.json` — **running timers**
- `POST /timers.json` — start timer
- `PUT /timers/{id}/pause.json` / `resume.json` / `stop.json`
- `DELETE /timers/{id}.json`
**Missing v3:** timers API got a full v3 refresh (`/projects/api/v3/me/timers/…`) which is the recommended path going forward.

### People (`teamwork people`)
**Have:** `list` (with project/company scoping via path)
**Missing:**
- `GET /people/{id}.json` — **show person** (we have no `people show`)
- `POST /people.json` — create user
- `PUT /people/{id}.json` — update
- `DELETE /people/{id}.json`
- Permissions endpoints (`/projects/{id}/people/{id}/permissions.json`)

### Companies (`teamwork companies`)
**Have:** `list`
**Missing:**
- `GET /companies/{id}.json` — **show company**
- `POST /companies.json` — create
- `PUT /companies/{id}.json` — update
- `DELETE /companies/{id}.json`

### Milestones — **entirely missing**
`GET /milestones.json`, `POST /projects/{id}/milestones.json`, update, delete, complete/uncomplete, `GET /projects/{id}/milestones.json`. Useful for project status reporting.

### Messages — **entirely missing**
List, create, update, delete, reply, archive, categories. Relevant if we ever want the CLI to post project updates from terminal/CI.

### Files, File Versions, Categories — **entirely missing**
Upload/download, list, delete, version management. File upload has two-step flow (pendingfile then attach) — worth noting if we ever add.

### Notebooks — **entirely missing**
Create/update/lock/unlock, comments, categories.

### Links — **entirely missing**
Simple CRUD, but often used to pin external docs/URLs to projects.

### Comments — **entirely missing**
The `/{resource}/{id}/comments.json` pattern applies to tasks, messages, notebooks, links, milestones, fileversions. Adding `teamwork comment add --task <id> "text"` would be one small command that works across all of them.

### Invoices, Expenses — **entirely missing**
Billing-side features. Invoices also have line items. Expenses are simpler CRUD.

### Risks — **entirely missing**
`GET /risks.json`. We use this almost never, skip.

### Workload — **entirely missing**
`GET /workload.json` returns capacity % per user over a window. Could be handy for the planning side, but Hourglass already tracks utilization.

### Calendar events — **entirely missing**
Not the Google Calendar — Teamwork's internal events feature. Low value for us.

### Tags — **entirely missing**
List all tags, filter by resource, assign/unassign. Small surface, easy win if we start tagging projects programmatically.

### Activity feed — **entirely missing**
`GET /latestActivity.json` — recent activity across all projects, or scoped to one. Would power a "what happened today" command nicely.

### Search — **entirely missing**
`GET /search.json?searchFor=tasks|messages|…&searchTerm=foo`. Cross-resource. Worth adding as `teamwork search`.

### Portfolio boards — **entirely missing**
Low priority — portfolio view isn't something we use heavily.

### Webhooks — **entirely missing**
`GET /webhooks.json`, create/delete. Useful if we ever want to subscribe Hourglass or another system to Teamwork events.

### Custom fields — **entirely missing**
Custom field definitions + values on projects and tasks. If you have org-specific fields (project phase, billing code, etc.) the CLI can't read them today.

### v3-only features we skip entirely
- **Sparse fieldsets** (`fields[projects]=id,name`) — major perf/pagination win on big queries
- **Sideloading** (`include=company,tags,owner`) — returns related entities in one call
- **Project updates** — v3 status-update feature, like a mini activity/message hybrid
- **Project metrics** — counts of open invoices, overdue tasks, etc., in one call
- **Consistent pagination meta** — v3 returns a standardized `meta.page` object

---

## Ergonomics gaps (not endpoint coverage)

Things the CLI *could* do with endpoints we already have but doesn't:

1. **Project name resolution** — every write command takes numeric IDs. Hourglass CLI resolves names; ours could do `--project "Acme"` → look up ID via `searchTerm`.
2. **`me` shortcut in filters** — `--assigned-to me` is hinted at in help, but we don't actually resolve it to the authenticated user's ID.
3. **Paginated iteration** — our list commands return one page; we have no `--all` flag that follows pagination.
4. **Output field selection** — we hardcode columns; no `--fields id,name,status` flag.
5. **Interactive selection** — hourglass CLI has dashboard views; nothing equivalent here. Could add `teamwork tasks today` as a friendlier preset.
6. **No tasklist discovery** for `tasks create` (which we haven't built).
7. **No timer verbs** (`teamwork timer start|stop|status`) — biggest missing-with-real-user-value item.

---

## Recommended priority to close gaps

Ranked by likely value for how we'd actually use this:

1. **Timers** (`start`, `stop`, `status`, `resume`) — this is the killer feature of a Teamwork CLI. Currently we have to log time after the fact.
2. **Task mutations** — create, update (priority/assignee/due), delete, uncomplete. Round out the existing tasks command.
3. **Name → ID resolution** for projects and people (incl. `me`), so flags accept names.
4. **Task lists** — required precursor to `tasks create`.
5. **Comments** — one small cross-resource command for quick notes from terminal.
6. **Activity feed / search** — read-only, useful for context gathering.
7. **v3 sideloading** — rewrite list commands to use v3 with `include=` to cut round-trips.
8. **Time entry update/delete** — fixing logging mistakes.
9. **Projects write** — create/archive.
10. **Webhooks** — only if we start integrating Teamwork events with other systems.

Everything else (files, notebooks, risks, expenses, portfolio, calendar, custom fields) is low priority unless a specific use case shows up.
