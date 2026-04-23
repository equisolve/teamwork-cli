package cmd

import (
	"strings"
	"testing"
)

// Real v1 /workload.json returns {"STATUS":"OK","workload":[<per project+user row>]}.
// Each row reports one user's load on one project — we aggregate to user totals.
const workloadFixture = `{
  "STATUS": "OK",
  "workload": [
    {
      "userId": "100", "userFirstName": "Ada", "userLastName": "Lovelace",
      "projectId": "1",
      "totalEstimatedTime": "60", "totalLoggedTime": "30",
      "numberOfActiveTasks": "2", "numberOfCompletedTasks": "1"
    },
    {
      "userId": "100", "userFirstName": "Ada", "userLastName": "Lovelace",
      "projectId": "2",
      "totalEstimatedTime": "30", "totalLoggedTime": "45",
      "numberOfActiveTasks": "1", "numberOfCompletedTasks": "0"
    },
    {
      "userId": "200", "userFirstName": "Grace", "userLastName": "Hopper",
      "projectId": "1",
      "totalEstimatedTime": "120", "totalLoggedTime": "90",
      "numberOfActiveTasks": "3", "numberOfCompletedTasks": "2"
    }
  ]
}`

func TestWorkload_AggregatesByUser(t *testing.T) {
	srv := newTestServer(t)
	srv.handle("GET", "/workload.json", workloadFixture)

	out, errOut, code := runCLI(t, srv,
		"workload",
		"--from", "2026-04-23",
		"--to", "2026-04-30",
	)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errOut)
	}

	// Ada: 2 projects, 3 active, 1 completed, 90 est, 75 logged.
	// Grace: 1 project, 3 active, 2 completed, 120 est, 90 logged.
	for _, want := range []string{
		"Ada Lovelace",
		"Grace Hopper",
		"2 user(s)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}

	adaLine := findLine(out, "Ada Lovelace")
	if adaLine == "" {
		t.Fatalf("no Ada row in:\n%s", out)
	}
	for _, want := range []string{"2", "3", "1", "90", "75"} {
		if !strings.Contains(adaLine, want) {
			t.Errorf("Ada row missing %q: %q", want, adaLine)
		}
	}

	// v1 workload.json wants compact YYYYMMDD dates.
	var q string
	for _, c := range srv.calls {
		if c.Path == "/workload.json" {
			q = c.Query
		}
	}
	for _, want := range []string{"startDate=20260423", "endDate=20260430"} {
		if !strings.Contains(q, want) {
			t.Errorf("workload query missing %q; got %q", want, q)
		}
	}
}

func findLine(out, needle string) string {
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}
