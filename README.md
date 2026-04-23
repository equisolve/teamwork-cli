# teamwork-cli

Go CLI for [Teamwork.com](https://teamwork.com) projects. Modeled after the
internal `hourglass` CLI — same flag conventions, config layout, and output
modes (`table` / `json` / `csv`). Covers every major Teamwork resource for
reads, with writes for the features a human actually uses from the terminal:
timers, task CRUD, time-entry edit/delete, and comments.

## Install

```
make install                  # builds and copies to /usr/local/bin/teamwork
```

Or build locally:

```
make build                    # produces ./teamwork
```

## Configure

Get your API key: **Profile → Edit My Profile → API & Mobile**.

```
teamwork config set url https://your-company.teamwork.com
teamwork config set token <your-api-key>
teamwork config show
```

Config lives at `~/.config/teamwork/config.yaml`. Override via `--url` /
`--token` flags or `TEAMWORK_URL` / `TEAMWORK_TOKEN` env vars.

A small name→ID cache is kept at `~/.config/teamwork/cache.json` for resolving
`--project "Acme"` → project ID on subsequent runs.

## Command reference

### Identity
```
teamwork me                                          # who am I
```

### Projects, tasks, task lists
```
teamwork projects list [--status active|archived|all] [--company ID]
                       [--search TERM] [--page N] [--page-size N]
teamwork projects show <id>

teamwork tasks list [--project ID|name] [--assignee ID|name|email|me]
                    [--status new|reopened|completed] [--completed]
                    [--due-from YYYY-MM-DD] [--due-to YYYY-MM-DD]
teamwork tasks show <id>
teamwork tasks complete <id>
teamwork tasks uncomplete <id>
teamwork tasks create --tasklist <id> --name "..."
                     [--description] [--assignee me|name|ID]
                     [--due YYYY-MM-DD] [--start YYYY-MM-DD]
                     [--priority low|medium|high] [--estimate MINS]
teamwork tasks update <id> [--name] [--description] [--assignee]
                           [--due] [--start] [--priority] [--estimate]
teamwork tasks delete <id> [--yes]
teamwork tasks subtasks <parent-id> [--add "line1\nline2"]

teamwork tasklists list --project <id|name> [--completed]
teamwork tasklists show <id>
```

### Time & timers
```
teamwork time list [--project ID] [--user ID|me]
                   [--from YYYY-MM-DD] [--to YYYY-MM-DD]
                   [--billable] [--invoiced]
teamwork time log --task <id> --hours 1.5 [--description] [--date] [--start]
teamwork time log --project <id> --hours 0.5
teamwork time update <id> [--hours] [--minutes] [--description]
                          [--date] [--start] [--billable yes|no]
teamwork time delete <id> [--yes]

teamwork timer list
teamwork timer start --task <id|name>  [--description] [--billable=false]
teamwork timer start --project <id|name>
teamwork timer stop <timer-id>
teamwork timer pause <timer-id>
teamwork timer resume <timer-id>
teamwork timer delete <timer-id>            # discard without logging
```

### People & companies
```
teamwork people list [--project ID|name] [--company ID|name] [--search TERM]
teamwork people show <id|me|email|name>

teamwork companies list [--search TERM]
teamwork companies show <id|name>
```

### Milestones, messages, files, notebooks, links, comments
```
teamwork milestones list [--project ID|name] [--completed]
teamwork milestones show <id>

teamwork messages list [--project ID|name]
teamwork messages show <id>

teamwork files list [--project ID|name]
teamwork files show <id>

teamwork notebooks list [--project ID|name]
teamwork notebooks show <id>

teamwork links list --project <id|name>
teamwork links show <id>

teamwork comments list --on task|message|milestone|notebook|link|fileversion --id <id>
teamwork comments add  --on task|message|milestone|notebook|link|fileversion --id <id> "body"
```

### Activity, search, tags, workload, categories
```
teamwork activity [--project ID|name] [--max N]
teamwork search <query> [--type tasks|messages|milestones|notebooks|
                               files|links|people|companies|projects|events]
teamwork tags list [--project ID|name] [--search TERM]
teamwork workload [--from YYYY-MM-DD] [--to YYYY-MM-DD]
teamwork categories list --kind project|message|file|notebook|link [--project ID|name]
```

### Finance, risks, templates, portfolio
```
teamwork invoices list [--project ID|name] [--status open|completed|all]
teamwork invoices show <id>

teamwork expenses list [--project ID|name]
teamwork expenses show <id>

teamwork risks list [--project ID|name]
teamwork risks show <id>

teamwork templates list                       # project templates
teamwork templates show <id>

teamwork portfolio boards list
teamwork portfolio boards show <id>
```

### Global flags

```
-o, --output  table|json|csv   (default: table)
    --json                     shortcut for -o json
    --url URL                  override configured URL
    --token KEY                override configured token
```

## Notes

- List commands for projects, tasks, people, companies, milestones, messages,
  files, notebooks, invoices, expenses use **v3** (`/projects/api/v3/…`) with
  sideloading (`include=…`) so related entity names render in one request.
- Writes (task/time/comment CRUD, timers) use v3 where it exists and v1
  otherwise. HTTP Basic auth with API-key as username, literal `x` as password.
- Name resolution: pass `--project "Accounting"` or `--assignee ada@…` and
  the CLI hits `?searchTerm=` to map it to an ID. Resolutions are cached.
- No file upload, no webhook management, no custom-field writes, no Teamwork
  project create/update/delete — out of scope for this build. Add when needed.

## Tests

```
go test ./...
go test -cover ./...
```

Every `internal/` package and every top-level command has at least one
httptest-backed test. See `cmd/testhelpers_test.go` for the harness.

## API coverage

See `docs/API_COVERAGE.md` for the original gap analysis against the Teamwork
API that drove this build.
