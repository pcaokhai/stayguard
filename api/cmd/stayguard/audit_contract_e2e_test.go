//go:build integration

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres"
	"github.com/pcaokhai/stayguard/api/internal/app"
)

// contracts/audit-actions.json lists every audit action with its category and the detail keys the activity log shows.
// Every end-to-end test records the audit rows its database holds when it ends; after the whole suite ran, the keys
// seen for each action must equal the file, and every listed action must have been seen. With -run (a partial suite)
// only the keys that were seen are compared.

type auditEntry struct {
	Category string   `json:"category"`
	Details  []string `json:"details"`
}

var (
	auditSeenMu sync.Mutex
	auditSeen   = map[string]map[string]bool{}
)

// recordAudit reads the activity log of every tenant of the test database the way listAuditLogs does.
func recordAudit(e *env) {
	ctx := context.Background()
	rows, err := e.owner.Query(ctx, `SELECT id FROM app.tenants`)
	if err != nil {
		return
	}
	var tenants []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			tenants = append(tenants, id)
		}
	}
	rows.Close()
	for _, tenant := range tenants {
		_ = postgres.NewUnitOfWork(e.pool).Do(ctx, tenant, func(ctx context.Context, tx app.Tx) error {
			got, err := postgres.MonitorRepo{}.AuditLogs(ctx, tx, app.AuditFilter{
				From: time.Unix(0, 0), To: time.Now().AddDate(10, 0, 0), Limit: 1_000_000})
			if err != nil {
				return err
			}
			auditSeenMu.Lock()
			defer auditSeenMu.Unlock()
			for _, r := range got {
				keys := auditSeen[r.Action]
				if keys == nil {
					keys = map[string]bool{}
					auditSeen[r.Action] = keys
				}
				for k := range app.AuditDetailsOf(r) {
					keys[k] = true
				}
			}
			return nil
		})
	}
}

// checkAuditContract compares what the suite saw with the file; it returns the problems.
func checkAuditContract() []string {
	if dump := os.Getenv("AUDIT_DUMP"); dump != "" { // writes what was seen, to start or refresh the file by hand
		out := map[string][]string{}
		for a, k := range auditSeen {
			out[a] = sortedKeys(k)
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		_ = os.WriteFile(dump, b, 0o600)
	}
	raw, err := os.ReadFile("../../../contracts/audit-actions.json")
	if err != nil {
		return []string{err.Error()}
	}
	var want map[string]auditEntry
	if err := json.Unmarshal(raw, &want); err != nil {
		return []string{"audit-actions.json: " + err.Error()}
	}
	full := flag.Lookup("test.run").Value.String() == ""
	var bad []string
	for action, keys := range auditSeen {
		w, ok := want[action]
		if !ok {
			bad = append(bad, action+": written by the suite but not in audit-actions.json")
			continue
		}
		seen := sortedKeys(keys)
		if !full { // a partial run may see only some rows of an action: every seen key must be listed
			for _, k := range seen {
				if !contains(w.Details, k) {
					bad = append(bad, fmt.Sprintf("%s: key %q not in audit-actions.json", action, k))
				}
			}
			continue
		}
		if !equal(seen, sortedCopy(w.Details)) {
			bad = append(bad, fmt.Sprintf("%s: suite produced %v, audit-actions.json says %v", action, seen, sortedCopy(w.Details)))
		}
	}
	if full {
		for action := range want {
			if _, ok := auditSeen[action]; !ok {
				bad = append(bad, action+": listed in audit-actions.json but no test produced it")
			}
		}
	}
	sort.Strings(bad)
	return bad
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedCopy(s []string) []string { c := append([]string{}, s...); sort.Strings(c); return c }

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func equal(a, b []string) bool { return strings.Join(a, "\x00") == strings.Join(b, "\x00") }
