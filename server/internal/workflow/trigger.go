package workflow

import (
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

// Trigger types. Schedule triggers become cron entries; event triggers are
// evaluated by the automation runner's watcher loop.
const (
	TriggerDaily     = "daily"     // Time + TZ, every day
	TriggerWeekly    = "weekly"    // Time + TZ on Days
	TriggerInterval  = "interval"  // every Minutes
	TriggerCron      = "cron"      // raw cron expression (5 fields, optional CRON_TZ=)
	TriggerMetric    = "metric"    // Metric above Threshold for ForMinutes
	TriggerContainer = "container" // a container matching Pattern stops or turns unhealthy
	TriggerService   = "service"   // a systemd service matching Pattern fails
	TriggerWebhook   = "webhook"   // POST /api/hooks/{Token}
	TriggerManual    = "manual"    // only "Run now"
)

type Trigger struct {
	Type    string `json:"type"`
	Time    string `json:"time,omitempty"`    // "HH:MM"
	TZ      string `json:"tz,omitempty"`      // IANA zone
	Days    []int  `json:"days,omitempty"`    // 0 = Sunday … 6 = Saturday
	Minutes int    `json:"minutes,omitempty"` // interval
	Cron    string `json:"cron,omitempty"`

	Metric     string  `json:"metric,omitempty"` // cpu | mem | disk | load
	Threshold  float64 `json:"threshold,omitempty"`
	ForMinutes int     `json:"forMinutes,omitempty"`

	Pattern string `json:"pattern,omitempty"` // container/service name glob, e.g. "web-*"
	Event   string `json:"event,omitempty"`   // container: stopped | unhealthy | either

	// CooldownMinutes stops an event trigger from re-firing while the
	// condition flaps. Defaults to 15.
	CooldownMinutes int `json:"cooldownMinutes,omitempty"`

	Token string `json:"token,omitempty"` // webhook (generated server-side)
}

// Alert is sent when a run fails, without needing a notify block in every
// workflow.
type Alert struct {
	URL    string `json:"url,omitempty"`
	Format string `json:"format,omitempty"`
}

type Options struct {
	Trigger   Trigger `json:"trigger"`
	OnFailure Alert   `json:"onFailure"`
}

// ParseOptions reads a workflow's options column, falling back to the
// legacy daily trigger (trigger_time + trigger_tz) for rows saved before
// options existed.
func ParseOptions(raw, legacyTime, legacyTZ string) Options {
	var o Options
	if raw != "" && json.Unmarshal([]byte(raw), &o) == nil && o.Trigger.Type != "" {
		return o
	}
	return Options{Trigger: Trigger{Type: TriggerDaily, Time: legacyTime, TZ: legacyTZ}}
}

func (t Trigger) IsEvent() bool {
	return t.Type == TriggerMetric || t.Type == TriggerContainer || t.Type == TriggerService
}

func (t Trigger) Cooldown() time.Duration {
	if t.CooldownMinutes <= 0 {
		return 15 * time.Minute
	}
	return time.Duration(t.CooldownMinutes) * time.Minute
}

var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

func parseHHMM(s string) (int, int, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("time %q must be HH:MM", s)
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, fmt.Errorf("time %q must be HH:MM (24-hour)", s)
	}
	return h, m, nil
}

func tzOr(tz string) (string, error) {
	if tz == "" {
		return "UTC", nil
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return "", fmt.Errorf("unknown timezone %q", tz)
	}
	return tz, nil
}

// CronSpec returns the robfig/cron spec for a schedule trigger, or "" for
// event/webhook/manual triggers. The CRON_TZ= prefix keeps "at midnight
// America/Chicago" firing at local midnight across DST changes.
func (t Trigger) CronSpec() (string, error) {
	switch t.Type {
	case TriggerDaily, TriggerWeekly:
		h, m, err := parseHHMM(t.Time)
		if err != nil {
			return "", err
		}
		tz, err := tzOr(t.TZ)
		if err != nil {
			return "", err
		}
		dow := "*"
		if t.Type == TriggerWeekly {
			if len(t.Days) == 0 {
				return "", fmt.Errorf("pick at least one day")
			}
			ds := make([]string, 0, len(t.Days))
			for _, d := range t.Days {
				if d < 0 || d > 6 {
					return "", fmt.Errorf("invalid day %d", d)
				}
				ds = append(ds, strconv.Itoa(d))
			}
			dow = strings.Join(ds, ",")
		}
		return fmt.Sprintf("CRON_TZ=%s %d %d * * %s", tz, m, h, dow), nil
	case TriggerInterval:
		if t.Minutes < 1 || t.Minutes > 7*24*60 {
			return "", fmt.Errorf("interval must be between 1 minute and 7 days")
		}
		return fmt.Sprintf("@every %dm", t.Minutes), nil
	case TriggerCron:
		spec := strings.TrimSpace(t.Cron)
		body := spec
		if strings.HasPrefix(body, "CRON_TZ=") || strings.HasPrefix(body, "TZ=") {
			if _, rest, ok := strings.Cut(body, " "); ok {
				body = rest
			}
		}
		if _, err := cronParser.Parse(body); err != nil {
			return "", fmt.Errorf("invalid cron expression: %v", err)
		}
		return spec, nil
	}
	return "", nil
}

var tokenPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,128}$`)

// ValidateOptions checks a trigger and alert before they're saved.
func ValidateOptions(o Options) error {
	t := o.Trigger
	switch t.Type {
	case TriggerDaily, TriggerWeekly, TriggerInterval, TriggerCron:
		if _, err := t.CronSpec(); err != nil {
			return err
		}
	case TriggerMetric:
		switch t.Metric {
		case "cpu", "mem", "disk":
			if t.Threshold <= 0 || t.Threshold > 100 {
				return fmt.Errorf("threshold must be between 1 and 100%%")
			}
		case "load":
			if t.Threshold <= 0 {
				return fmt.Errorf("load threshold must be positive")
			}
		default:
			return fmt.Errorf("metric must be cpu, mem, disk, or load")
		}
		if t.ForMinutes < 0 || t.ForMinutes > 1440 {
			return fmt.Errorf("duration must be between 0 and 1440 minutes")
		}
	case TriggerContainer, TriggerService:
		if strings.TrimSpace(t.Pattern) == "" {
			return fmt.Errorf("enter a name or pattern (use * to match any)")
		}
		if _, err := path.Match(t.Pattern, ""); err != nil {
			return fmt.Errorf("invalid pattern %q", t.Pattern)
		}
		if t.Type == TriggerContainer && t.Event != "stopped" && t.Event != "unhealthy" && t.Event != "either" {
			return fmt.Errorf("container event must be stopped, unhealthy, or either")
		}
	case TriggerWebhook:
		if t.Token != "" && !tokenPattern.MatchString(t.Token) {
			return fmt.Errorf("invalid webhook token")
		}
	case TriggerManual:
	default:
		return fmt.Errorf("unknown trigger type %q", t.Type)
	}
	if t.CooldownMinutes < 0 || t.CooldownMinutes > 10080 {
		return fmt.Errorf("cooldown must be between 0 and 10080 minutes")
	}
	if o.OnFailure.URL != "" {
		if !strings.HasPrefix(o.OnFailure.URL, "http://") && !strings.HasPrefix(o.OnFailure.URL, "https://") {
			return fmt.Errorf("failure alert URL must start with http:// or https://")
		}
		if o.OnFailure.Format != "" && !validNotifyFormats[o.OnFailure.Format] {
			return fmt.Errorf("failure alert format must be discord, slack, ntfy, or json")
		}
	}
	return nil
}

// Describe is a short human summary of a trigger for lists and logs.
func (t Trigger) Describe() string {
	days := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	switch t.Type {
	case TriggerDaily:
		return fmt.Sprintf("Daily at %s %s", t.Time, t.TZ)
	case TriggerWeekly:
		names := []string{}
		for _, d := range t.Days {
			if d >= 0 && d <= 6 {
				names = append(names, days[d])
			}
		}
		return fmt.Sprintf("%s at %s %s", strings.Join(names, ", "), t.Time, t.TZ)
	case TriggerInterval:
		if t.Minutes%60 == 0 {
			return fmt.Sprintf("Every %dh", t.Minutes/60)
		}
		return fmt.Sprintf("Every %dm", t.Minutes)
	case TriggerCron:
		return "Cron " + t.Cron
	case TriggerMetric:
		unit := "%"
		if t.Metric == "load" {
			unit = ""
		}
		s := fmt.Sprintf("%s above %g%s", strings.ToUpper(t.Metric), t.Threshold, unit)
		if t.ForMinutes > 0 {
			s += fmt.Sprintf(" for %dm", t.ForMinutes)
		}
		return s
	case TriggerContainer:
		return fmt.Sprintf("Container %s %s", t.Pattern, map[string]string{"stopped": "stops", "unhealthy": "turns unhealthy", "either": "stops or turns unhealthy"}[t.Event])
	case TriggerService:
		return fmt.Sprintf("Service %s fails", t.Pattern)
	case TriggerWebhook:
		return "Webhook"
	case TriggerManual:
		return "Manual only"
	}
	return t.Type
}
