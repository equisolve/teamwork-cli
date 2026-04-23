# Changelog

All notable changes to `teamwork-cli`. Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v0.2.2] — 2026-04-23

Seven mutation-surface bugs surfaced by end-to-end live testing against Teamwork.

### Fixed
- **`tasks show`** — the `Estimated min` field never rendered. The code read `estimatedMinutes`; the v3 task resource returns `estimateMinutes`.
- **`tasks subtasks --add`** — posted `{"todo-item":{"content":…}}` to the v1 `/quickadd.json` endpoint, which rejected it (`'content' must be passed and be a string`). Now splits `--add` on newline / `~|~` and POSTs one subtask per line to `/projects/api/v3/tasks/<id>/subtasks.json`.
- **`timer start`** — sent `"billable": <bool>` in the payload; v3 rejects it with `unknown field "billable"`. Correct key is `isBillable`.
- **`timer start`** — v3 doesn't auto-derive a timer's project from its task, and `/complete.json` later refuses timers where `projectId == 0` (`"A timer must belong to a project, invalid project ID"`). The CLI now fetches the task with `?include=projects` and lifts the project out of the sideload before starting the timer.
- **`timer stop`** — PUT `/stop.json` returned 404. The v3 "stop and log" operation is `/complete.json`.
- **`time log`** — success message rendered the raw `YYYYMMDD` the API expects (e.g. `on 20260423`). Pretty-prints ISO now.
- **`comments list`** — `AUTHOR` column was always empty. The CLI read `author-fullname`; the real v1 response carries `author-firstname` + `author-lastname`, which are now joined.

### Note
Four of the twelve bugs fixed across v0.2.x had unit tests that mocked *fictional* response shapes matching the CLI's mistaken expectations — so the tests passed while the real API failed. Fixtures for `workload`, `activity`, `tasks subtasks`, and `comments list` were rewritten against captured live responses.

## [v0.2.1] — 2026-04-23

### Fixed
- **`activity`** — `USER`, `ACTION`, and `TYPE` columns were always blank. The parser read `activity-type` / `action` / `for-user-name`; the v1 response uses `activitytype` (the verb: new/updated/completed/reopened), `type` (the object: task/message/comment), and `forusername`. Both distinct columns populate now.
- **`activity` (unscoped)** — `/latestActivity.json` without a project scope hangs for 60s+ on tenants with many active projects. Detection: a timeout error for an unscoped call now prints a stderr hint suggesting `--project <name>`.

## [v0.2.0] — 2026-04-23

### Fixed
- **`tasks list --assignee`** — v3 silently ignores `assignedToUserIds` (assignee queries returned everything). Now passes `responsiblePartyIds`.
- **`tasks list --due-from` / `--due-to`** — v3 `tasks.json` requires ISO `YYYY-MM-DD`, not the v1 compact `YYYYMMDD` format; date filters were being silently dropped.
- **`tasks list --completed`** — meant "include completed tasks alongside non-completed" (`includeCompletedTasks=true` is additive on v3). Now shows **only** completed tasks: maps `--due-from` / `--due-to` to `completedAfter` / `completedBefore` (completion-date filter) and drops any non-completed row client-side as a safety net.
- **`workload`** — always printed `0 user(s)`. The parser expected `workload.userCapacities[]`; the v1 response is `workload: [<project+user row>, …]`. Output reshaped to `USER / PROJECTS / ACTIVE / COMPLETED / EST (min) / LOGGED (min)`, aggregated by user.

### Breaking
- `workload` columns changed (`CAPACITY % / AVAILABLE` → `PROJECTS / ACTIVE / COMPLETED`). The old columns never populated anyway.
- `tasks list --completed` semantics changed from "also include completed" to "only completed". Scripts that expected the mixed output will need `--completed` dropped.

## [v0.1.0] — 2024

Initial release.
