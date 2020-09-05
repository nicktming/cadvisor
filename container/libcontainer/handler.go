package libcontainer

import (
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/google/cadvisor/container"
	info "github.com/google/cadvisor/info/v1"
	"golang.org/x/sys/unix"
	"time"
	"k8s.io/klog"
	"path"
	"strconv"
	"fmt"
	"strings"
	"os"
	"bufio"
)

type Handler struct {
	cgroupManager 		cgroups.Manager
	rootFs 			string
	pid 			int
	includedMetrics 	container.MetricSet
	pidMetricsCache 	map[int]*info.CpuSchedstat
	cycles 			uint64
}

func NewHandler(cgroupManager cgroups.Manager, rootFs string, pid int, includedMetrics container.MetricSet) *Handler {
	return &Handler{
		cgroupManager:   cgroupManager,
		rootFs:          rootFs,
		pid:             pid,
		includedMetrics: includedMetrics,
		pidMetricsCache: make(map[int]*info.CpuSchedstat),
	}
}

// Get cgroup and networking stats of the specified container
func (h *Handler) GetStats() (*info.ContainerStats, error) {
	var cgroupStats *cgroups.Stats
	readCgroupStats := true
	//if cgroups.IsCgroup2UnifiedMode() {
	//	// On cgroup v2 there are no stats at the root cgroup
	//	// so check whether it is the root cgroup
	//	if h.cgroupManager.Path("") == fs2.UnifiedMountpoint {
	//		readCgroupStats = false
	//	}
	//}

	var err error
	if readCgroupStats {
		cgroupStats, err = h.cgroupManager.GetStats()
		if err != nil {
			return nil, err
		}
	}
	libcontainerStats := &libcontainer.Stats {
		CgroupStats: cgroupStats,
	}

	stats := newContainerStats(libcontainerStats, h.includedMetrics)

	//klog.Infof("Handler pid: %v", h.pid)
	// If we know the pid then get network stats from /proc/<pid>/net/dev
	if h.pid > 0 {
		if h.includedMetrics.Has(container.NetworkUsageMetrics) {
			netStats, err := networkStatsFromProc(h.rootFs, h.pid)
			if err != nil {
				klog.Infof("unable to get network stats from pid %d: %v", h.pid, err)
			} else {
				stats.Network.Interfaces = append(stats.Network.Interfaces, netStats...)
			}
		}
	}

	if len(stats.Network.Interfaces) > 0 {
		stats.Network.InterfaceStats = stats.Network.Interfaces[0]
	}

	return stats, nil
}

func networkStatsFromProc(rootFs string, pid int) ([]info.InterfaceStats, error) {
	netStatsFile := path.Join(rootFs, "proc", strconv.Itoa(pid), "/net/dev")

	ifaceStats, err := scanInterfaceStats(netStatsFile)
	if err != nil {
		return []info.InterfaceStats{}, fmt.Errorf("couldn't read network stats: %v", err)
	}

	return ifaceStats, nil
}

var (
	ignoredDevicePrefixes = []string{"lo", "veth", "docker"}
)

func isIgnoredDevice(ifName string) bool {
	for _, prefix := range ignoredDevicePrefixes {
		if strings.HasPrefix(strings.ToLower(ifName), prefix) {
			return true
		}
	}
	return false
}

func scanInterfaceStats(netStatsFile string) ([]info.InterfaceStats, error) {
	file, err := os.Open(netStatsFile)
	if err != nil {
		return nil, fmt.Errorf("failure opening %s: %v", netStatsFile, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// Discard header lines
	for i := 0; i < 2; i++ {
		if b := scanner.Scan(); !b {
			return nil, scanner.Err()
		}
	}

	stats := []info.InterfaceStats{}
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.Replace(line, ":", "", -1)

		fields := strings.Fields(line)
		// If the format of the  line is invalid then don't trust any of the stats
		// in this file.
		if len(fields) != 17 {
			return nil, fmt.Errorf("invalid interface stats line: %v", line)
		}

		devName := fields[0]
		if isIgnoredDevice(devName) {
			continue
		}

		i := info.InterfaceStats{
			Name: devName,
		}

		statFields := append(fields[1:5], fields[9:13]...)
		statPointers := []*uint64{
			&i.RxBytes, &i.RxPackets, &i.RxErrors, &i.RxDropped,
			&i.TxBytes, &i.TxPackets, &i.TxErrors, &i.TxDropped,
		}

		err := setInterfaceStatValues(statFields, statPointers)
		if err != nil {
			return nil, fmt.Errorf("cannot parse interface stats (%v): %v", err, line)
		}

		stats = append(stats, i)
	}

	return stats, nil
}

func setInterfaceStatValues(fields []string, pointers []*uint64) error {
	for i, v := range fields {
		val, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return err
		}
		*pointers[i] = val
	}
	return nil
}

func getNumberOnlineCPUs() (uint32, error) {
	var availableCPUs unix.CPUSet
	if err := unix.SchedGetaffinity(0, &availableCPUs); err != nil {
		return 0, err
	}
	return uint32(availableCPUs.Count()), nil
}

// var to allow unit tests to stub it out
var numCpusFunc = getNumberOnlineCPUs

// Convert libcontainer stats to info.ContainerStats.
func setCPUStats(s *cgroups.Stats, ret *info.ContainerStats, withPerCPU bool) {
	ret.Cpu.Usage.User = s.CpuStats.CpuUsage.UsageInUsermode
	ret.Cpu.Usage.System = s.CpuStats.CpuUsage.UsageInKernelmode
	ret.Cpu.Usage.Total = s.CpuStats.CpuUsage.TotalUsage
	ret.Cpu.CFS.Periods = s.CpuStats.ThrottlingData.Periods
	ret.Cpu.CFS.ThrottledPeriods = s.CpuStats.ThrottlingData.ThrottledPeriods
	ret.Cpu.CFS.ThrottledTime = s.CpuStats.ThrottlingData.ThrottledTime

	if !withPerCPU {
		return
	}
	if len(s.CpuStats.CpuUsage.PercpuUsage) == 0 {
		// libcontainer's 'GetStats' can leave 'PercpuUsage' nil if it skipped the
		// cpuacct subsystem.
		return
	}

	numPossible := uint32(len(s.CpuStats.CpuUsage.PercpuUsage))
	// Note that as of https://patchwork.kernel.org/patch/8607101/ (kernel v4.7),
	// the percpu usage information includes extra zero values for all additional
	// possible CPUs. This is to allow statistic collection after CPU-hotplug.
	// We intentionally ignore these extra zeroes.
	numActual, err := numCpusFunc()
	if err != nil {
		klog.Errorf("unable to determine number of actual cpus; defaulting to maximum possible number: errno %v", err)
		numActual = numPossible
	}
	if numActual > numPossible {
		// The real number of cores should never be greater than the number of
		// datapoints reported in cpu usage.
		klog.Errorf("PercpuUsage had %v cpus, but the actual number is %v; ignoring extra CPUs", numPossible, numActual)
	}
	numActual = minUint32(numPossible, numActual)
	ret.Cpu.Usage.PerCpu = make([]uint64, numActual)

	for i := uint32(0); i < numActual; i++ {
		ret.Cpu.Usage.PerCpu[i] = s.CpuStats.CpuUsage.PercpuUsage[i]
	}

}


func (h *Handler) GetProcesses() ([]int, error) {
	pids, err := h.cgroupManager.GetPids()
	if err != nil {
		return nil, err
	}
	return pids, nil
}

func minUint32(x, y uint32) uint32 {
	if x < y {
		return x
	}
	return y
}

func setDiskIoStats(s *cgroups.Stats, ret *info.ContainerStats) {
	ret.DiskIo.IoServiceBytes = DiskStatsCopy(s.BlkioStats.IoServiceBytesRecursive)
	ret.DiskIo.IoServiced = DiskStatsCopy(s.BlkioStats.IoServicedRecursive)
	ret.DiskIo.IoQueued = DiskStatsCopy(s.BlkioStats.IoQueuedRecursive)
	ret.DiskIo.Sectors = DiskStatsCopy(s.BlkioStats.SectorsRecursive)
	ret.DiskIo.IoServiceTime = DiskStatsCopy(s.BlkioStats.IoServiceTimeRecursive)
	ret.DiskIo.IoWaitTime = DiskStatsCopy(s.BlkioStats.IoWaitTimeRecursive)
	ret.DiskIo.IoMerged = DiskStatsCopy(s.BlkioStats.IoMergedRecursive)
	ret.DiskIo.IoTime = DiskStatsCopy(s.BlkioStats.IoTimeRecursive)
}

func setMemoryStats(s *cgroups.Stats, ret *info.ContainerStats) {
	ret.Memory.Usage = s.MemoryStats.Usage.Usage
	ret.Memory.MaxUsage = s.MemoryStats.Usage.MaxUsage
	ret.Memory.Failcnt = s.MemoryStats.Usage.Failcnt

	if s.MemoryStats.UseHierarchy {
		ret.Memory.Cache = s.MemoryStats.Stats["total_cache"]
		ret.Memory.RSS = s.MemoryStats.Stats["total_rss"]
		ret.Memory.Swap = s.MemoryStats.Stats["total_swap"]
		ret.Memory.MappedFile = s.MemoryStats.Stats["total_mapped_file"]
	} else {
		ret.Memory.Cache = s.MemoryStats.Stats["cache"]
		ret.Memory.RSS = s.MemoryStats.Stats["rss"]
		ret.Memory.Swap = s.MemoryStats.Stats["swap"]
		ret.Memory.MappedFile = s.MemoryStats.Stats["mapped_file"]
	}
	if v, ok := s.MemoryStats.Stats["pgfault"]; ok {
		ret.Memory.ContainerData.Pgfault = v
		ret.Memory.HierarchicalData.Pgfault = v
	}
	if v, ok := s.MemoryStats.Stats["pgmajfault"]; ok {
		ret.Memory.ContainerData.Pgmajfault = v
		ret.Memory.HierarchicalData.Pgmajfault = v
	}

	inactiveFileKeyName := "total_inactive_file"
	if cgroups.IsCgroup2UnifiedMode() {
		inactiveFileKeyName = "inactive_file"
	}

	workingSet := ret.Memory.Usage
	if v, ok := s.MemoryStats.Stats[inactiveFileKeyName]; ok {
		if workingSet < v {
			workingSet = 0
		} else {
			workingSet -= v
		}
	}
	ret.Memory.WorkingSet = workingSet
}

func setNetworkStats(libcontainerStats *libcontainer.Stats, ret *info.ContainerStats) {
	ret.Network.Interfaces = make([]info.InterfaceStats, len(libcontainerStats.Interfaces))
	for i := range libcontainerStats.Interfaces {
		ret.Network.Interfaces[i] = info.InterfaceStats{
			Name:      libcontainerStats.Interfaces[i].Name,
			RxBytes:   libcontainerStats.Interfaces[i].RxBytes,
			RxPackets: libcontainerStats.Interfaces[i].RxPackets,
			RxErrors:  libcontainerStats.Interfaces[i].RxErrors,
			RxDropped: libcontainerStats.Interfaces[i].RxDropped,
			TxBytes:   libcontainerStats.Interfaces[i].TxBytes,
			TxPackets: libcontainerStats.Interfaces[i].TxPackets,
			TxErrors:  libcontainerStats.Interfaces[i].TxErrors,
			TxDropped: libcontainerStats.Interfaces[i].TxDropped,
		}
	}

	// Add to base struct for backwards compatibility.
	if len(ret.Network.Interfaces) > 0 {
		ret.Network.InterfaceStats = ret.Network.Interfaces[0]
	}
}

func newContainerStats(libcontainerStats *libcontainer.Stats, includedMetrics container.MetricSet) *info.ContainerStats {
	ret := &info.ContainerStats{
		Timestamp: time.Now(),
	}

	if s := libcontainerStats.CgroupStats; s != nil {
		setCPUStats(s, ret, includedMetrics.Has(container.PerCpuUsageMetrics))
		if includedMetrics.Has(container.DiskIOMetrics) {
			setDiskIoStats(s, ret)
		}
		setMemoryStats(s, ret)
		// TODO memory_numa hugetlb
	}
	if len(libcontainerStats.Interfaces) > 0 {
		setNetworkStats(libcontainerStats, ret)
	}
	return ret
}