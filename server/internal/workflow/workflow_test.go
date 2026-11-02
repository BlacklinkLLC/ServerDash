package workflow

import (
	"context"
	"strings"
	"testing"
)

func TestShellVariablesAreQuoted(t *testing.T) {
	e := &Executor{}
	st := &runState{rc: Context{Payload: `x'; echo pwned; '`, Event: "CPU at 93% (threshold 90%)"}}
	out, err := e.runBlock(context.Background(), Block{Type: BlockShell, Command: "printf '%s|%s' {{payload}} {{event}}"}, st)
	if err != nil {
		t.Fatalf("run: %v (%s)", err, out)
	}
	want := `x'; echo pwned; '|CPU at 93% (threshold 90%)`
	if out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

func TestCheckStopsRun(t *testing.T) {
	e := &Executor{}
	log, ok := e.Run(context.Background(), []Block{
		{Type: BlockShell, Command: "echo hello"},
		{Type: BlockCheck, Check: CheckOutputContains, Text: "deploy"},
		{Type: BlockShell, Command: "echo should-not-run"},
	}, Context{Trigger: "manual"})
	if !ok || strings.Contains(log, "should-not-run") || !strings.Contains(log, "stopped") {
		t.Fatalf("ok=%v log:\n%s", ok, log)
	}
}

func TestCronSpecs(t *testing.T) {
	cases := map[string]Trigger{
		"CRON_TZ=UTC 30 3 * * *":            {Type: TriggerDaily, Time: "03:30", TZ: "UTC"},
		"CRON_TZ=Europe/London 0 9 * * 1,5": {Type: TriggerWeekly, Time: "09:00", TZ: "Europe/London", Days: []int{1, 5}},
		"@every 15m":                        {Type: TriggerInterval, Minutes: 15},
		"":                                  {Type: TriggerWebhook},
	}
	for want, tr := range cases {
		got, err := tr.CronSpec()
		if err != nil || got != want {
			t.Errorf("%+v: got %q, %v; want %q", tr, got, err, want)
		}
	}
}
