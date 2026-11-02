package system

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"syscall"

	psnet "github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

type ProcessInfo struct {
	PID        int32   `json:"pid"`
	Name       string  `json:"name"`
	User       string  `json:"user"`
	Command    string  `json:"command"`
	Status     string  `json:"status"`
	CPUPercent float64 `json:"cpu"`
	MemPercent float32 `json:"mem"`
	RSSBytes   uint64  `json:"rss"`
	Threads    int32   `json:"threads"`
	StartedMs  int64   `json:"startedMs"`
}

// Processes lists host processes sorted by sortBy ("cpu" or "mem"), capped
// at limit (0 = no cap). CPU percent is averaged over each process's
// lifetime, which is what gopsutil can report without a second sample.
func Processes(sortBy string, limit int) ([]ProcessInfo, error) {
	procs, err := process.Processes()
	if err != nil {
		return nil, err
	}
	out := make([]ProcessInfo, 0, len(procs))
	for _, p := range procs {
		name, err := p.Name()
		if err != nil {
			continue // exited between listing and reading
		}
		info := ProcessInfo{PID: p.Pid, Name: name}
		info.User, _ = p.Username()
		info.Command, _ = p.Cmdline()
		if len(info.Command) > 300 {
			info.Command = info.Command[:300] + "…"
		}
		if st, err := p.Status(); err == nil && len(st) > 0 {
			info.Status = st[0]
		}
		info.CPUPercent, _ = p.CPUPercent()
		info.MemPercent, _ = p.MemoryPercent()
		if mi, err := p.MemoryInfo(); err == nil {
			info.RSSBytes = mi.RSS
		}
		info.Threads, _ = p.NumThreads()
		info.StartedMs, _ = p.CreateTime()
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool {
		if sortBy == "mem" {
			return out[i].MemPercent > out[j].MemPercent
		}
		return out[i].CPUPercent > out[j].CPUPercent
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

var signals = map[string]syscall.Signal{
	"TERM": syscall.SIGTERM,
	"KILL": syscall.SIGKILL,
	"HUP":  syscall.SIGHUP,
	"INT":  syscall.SIGINT,
}

// Signal sends a named signal to a host process. PID 1 and ServerDash's
// own process are refused: killing either takes the dashboard down with it.
func Signal(pid int32, name string) error {
	sig, ok := signals[strings.ToUpper(name)]
	if !ok {
		return fmt.Errorf("unsupported signal %q (use TERM, KILL, HUP, or INT)", name)
	}
	if pid <= 1 || int(pid) == os.Getpid() {
		return errors.New("refusing to signal init or ServerDash itself")
	}
	return syscall.Kill(int(pid), sig)
}

type ListeningPort struct {
	Protocol string `json:"protocol"`
	Address  string `json:"address"`
	Port     uint32 `json:"port"`
	PID      int32  `json:"pid"`
	Process  string `json:"process"`
}

// ListeningPorts reports TCP sockets in LISTEN state and bound UDP sockets.
func ListeningPorts() ([]ListeningPort, error) {
	conns, err := psnet.Connections("inet")
	if err != nil {
		return nil, err
	}
	names := map[int32]string{}
	seen := map[string]bool{}
	out := []ListeningPort{}
	for _, c := range conns {
		proto := "tcp"
		if c.Type == syscall.SOCK_DGRAM {
			proto = "udp"
			if c.Raddr.Port != 0 {
				continue
			}
		} else if c.Status != "LISTEN" {
			continue
		}
		if c.Family == syscall.AF_INET6 {
			proto += "6"
		}
		key := fmt.Sprintf("%s|%s|%d", proto, c.Laddr.IP, c.Laddr.Port)
		if seen[key] {
			continue
		}
		seen[key] = true
		lp := ListeningPort{Protocol: proto, Address: c.Laddr.IP, Port: c.Laddr.Port, PID: c.Pid}
		if c.Pid > 0 {
			if n, ok := names[c.Pid]; ok {
				lp.Process = n
			} else if p, err := process.NewProcess(c.Pid); err == nil {
				lp.Process, _ = p.Name()
				names[c.Pid] = lp.Process
			}
		}
		out = append(out, lp)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Port != out[j].Port {
			return out[i].Port < out[j].Port
		}
		return out[i].Protocol < out[j].Protocol
	})
	return out, nil
}

type Interface struct {
	Name      string   `json:"name"`
	MTU       int      `json:"mtu"`
	MAC       string   `json:"mac"`
	Up        bool     `json:"up"`
	Addresses []string `json:"addresses"`
	RxBytes   uint64   `json:"rxBytes"`
	TxBytes   uint64   `json:"txBytes"`
	RxErrors  uint64   `json:"rxErrors"`
	TxErrors  uint64   `json:"txErrors"`
}

func Interfaces() ([]Interface, error) {
	ifaces, err := psnet.Interfaces()
	if err != nil {
		return nil, err
	}
	counters := map[string]psnet.IOCountersStat{}
	if cs, err := psnet.IOCounters(true); err == nil {
		for _, c := range cs {
			counters[c.Name] = c
		}
	}
	out := make([]Interface, 0, len(ifaces))
	for _, i := range ifaces {
		it := Interface{Name: i.Name, MTU: i.MTU, MAC: i.HardwareAddr}
		for _, f := range i.Flags {
			if f == "up" {
				it.Up = true
			}
		}
		for _, a := range i.Addrs {
			it.Addresses = append(it.Addresses, a.Addr)
		}
		if c, ok := counters[i.Name]; ok {
			it.RxBytes, it.TxBytes = c.BytesRecv, c.BytesSent
			it.RxErrors, it.TxErrors = c.Errin, c.Errout
		}
		out = append(out, it)
	}
	return out, nil
}
