package systemd

import (
	"bufio"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// journalctl -o json emits MESSAGE as a string, or as a byte array when
// it isn't valid UTF-8; json.RawMessage lets both decode.
type rawEntry struct {
	Message  json.RawMessage `json:"MESSAGE"`
	Priority string          `json:"PRIORITY"`
	Unit     string          `json:"_SYSTEMD_UNIT"`
	Ident    string          `json:"SYSLOG_IDENTIFIER"`
	Realtime string          `json:"__REALTIME_TIMESTAMP"`
}

func parseJournal(out string) []LogEntry {
	entries := []LogEntry{}
	sc := bufio.NewScanner(strings.NewReader(out))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		var r rawEntry
		if json.Unmarshal(sc.Bytes(), &r) != nil {
			continue
		}
		e := LogEntry{Unit: r.Unit, Priority: 6, Message: decodeMessage(r.Message)}
		if e.Unit == "" {
			e.Unit = r.Ident
		}
		if p, err := strconv.Atoi(r.Priority); err == nil {
			e.Priority = p
		}
		if us, err := strconv.ParseInt(r.Realtime, 10, 64); err == nil {
			e.Time = time.UnixMicro(us).Format(time.RFC3339)
		}
		entries = append(entries, e)
	}
	return entries
}

func decodeMessage(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var b []byte
	var ints []int
	if json.Unmarshal(raw, &ints) == nil {
		b = make([]byte, len(ints))
		for i, v := range ints {
			b[i] = byte(v)
		}
		return strings.ToValidUTF8(string(b), "�")
	}
	return ""
}
