// Package system reports host-level metrics for the dashboard's server overview.
package system

import (
	"path/filepath"
	"sort"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
)

type DiskUsage struct {
	Path        string  `json:"path"`
	TotalBytes  uint64  `json:"totalBytes"`
	UsedBytes   uint64  `json:"usedBytes"`
	UsedPercent float64 `json:"usedPercent"`
}

type Snapshot struct {
	Hostname       string      `json:"hostname"`
	Platform       string      `json:"platform"`
	KernelVersion  string      `json:"kernelVersion"`
	UptimeSeconds  uint64      `json:"uptimeSeconds"`
	CPUPercent     float64     `json:"cpuPercent"`
	CPUCores       int         `json:"cpuCores"`
	LoadAvg1       float64     `json:"loadAvg1"`
	LoadAvg5       float64     `json:"loadAvg5"`
	LoadAvg15      float64     `json:"loadAvg15"`
	MemTotalBytes  uint64      `json:"memTotalBytes"`
	MemUsedBytes   uint64      `json:"memUsedBytes"`
	MemUsedPercent float64     `json:"memUsedPercent"`
	Disks          []DiskUsage `json:"disks"`
	CollectedAt    time.Time   `json:"collectedAt"`
}

// Collect gathers a point-in-time snapshot of host metrics. It degrades
// gracefully: a failure to read one metric doesn't prevent the others from
// being reported, since partial dashboard data beats none.
//
// hostRoot is the path the host's root filesystem is bind-mounted at (e.g.
// "/rootfs" in the container deployment), used to statfs the host's actual
// disks instead of the container's own overlay filesystem; a mountpoint
// discovered as "/boot" is then measured at "<hostRoot>/boot" but still
// reported as "/boot". Pass "" when running directly on the host, where
// mountpoints are already accessible at their real paths.
func Collect(hostRoot string) (Snapshot, error) {
	snap := Snapshot{CollectedAt: time.Now()}

	if hi, err := host.Info(); err == nil {
		snap.Hostname = hi.Hostname
		snap.Platform = hi.Platform + " " + hi.PlatformVersion
		snap.KernelVersion = hi.KernelVersion
		snap.UptimeSeconds = hi.Uptime
	}

	if pct, err := cpu.Percent(0, false); err == nil && len(pct) > 0 {
		snap.CPUPercent = pct[0]
	}
	if counts, err := cpu.Counts(true); err == nil {
		snap.CPUCores = counts
	}

	if avg, err := load.Avg(); err == nil {
		snap.LoadAvg1 = avg.Load1
		snap.LoadAvg5 = avg.Load5
		snap.LoadAvg15 = avg.Load15
	}

	if vm, err := mem.VirtualMemory(); err == nil {
		snap.MemTotalBytes = vm.Total
		snap.MemUsedBytes = vm.Used
		snap.MemUsedPercent = vm.UsedPercent
	}

	if parts, err := disk.Partitions(false); err == nil {
		snap.Disks = collectDisks(parts, hostRoot)
	}

	return snap, nil
}

// collectDisks reports one entry per physical device, keyed on its
// shortest mountpoint. When read via a host /proc bind-mount (as in the
// container deployment), the host's mount table includes every other
// container's single-file bind mounts (/etc/resolv.conf, /etc/hostname, ...)
// sharing the same backing device as its real mountpoint; without
// deduping, those would flood the list with noise.
func collectDisks(parts []disk.PartitionStat, hostRoot string) []DiskUsage {
	byDevice := make(map[string]disk.PartitionStat)
	for _, p := range parts {
		existing, ok := byDevice[p.Device]
		if !ok || len(p.Mountpoint) < len(existing.Mountpoint) {
			byDevice[p.Device] = p
		}
	}

	disks := make([]DiskUsage, 0, len(byDevice))
	for _, p := range byDevice {
		statPath := p.Mountpoint
		if hostRoot != "" {
			statPath = filepath.Join(hostRoot, p.Mountpoint)
		}
		usage, err := disk.Usage(statPath)
		if err != nil {
			continue
		}
		disks = append(disks, DiskUsage{
			Path:        p.Mountpoint,
			TotalBytes:  usage.Total,
			UsedBytes:   usage.Used,
			UsedPercent: usage.UsedPercent,
		})
	}
	sort.Slice(disks, func(i, j int) bool { return disks[i].Path < disks[j].Path })
	return disks
}
