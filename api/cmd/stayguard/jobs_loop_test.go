package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// `jobs run --every 5m` keeps going after a failed round and stops cleanly when the context ends.
func TestRepeatJobs_KeepsGoingAfterAFailure_FU(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	rounds := 0
	var out bytes.Buffer
	err := repeatJobs(ctx, time.Millisecond, func(context.Context) error {
		rounds++
		if rounds == 3 {
			cancel()
		}
		if rounds == 1 {
			return errors.New("one tenant failed")
		}
		return nil
	}, &out)
	if err != nil || rounds != 3 {
		t.Fatalf("rounds %d, err %v", rounds, err)
	}
	if !strings.Contains(out.String(), "one tenant failed") {
		t.Fatalf("the failure is reported: %q", out.String())
	}
}

func TestParseJobsArgs_FU(t *testing.T) {
	for args, want := range map[string]time.Duration{"jobs run": 0, "jobs run --every 5m": 5 * time.Minute} {
		got, err := parseJobsArgs(strings.Fields(args))
		if err != nil || got != want {
			t.Errorf("%q: %v %v", args, got, err)
		}
	}
	for _, bad := range []string{"jobs", "jobs run --every", "jobs run --every soon", "jobs run --every 1s"} {
		if _, err := parseJobsArgs(strings.Fields(bad)); err == nil {
			t.Errorf("%q must be refused", bad)
		}
	}
}
