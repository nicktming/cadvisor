package libcontainer

import (
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/google/cadvisor/container"
	info "github.com/google/cadvisor/info/v1"
	"time"
	//"k8s.io/klog"
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

	return stats, nil
}



func newContainerStats(libcontainerStats *libcontainer.Stats, includedMetrics container.MetricSet) *info.ContainerStats {
	ret := &info.ContainerStats{
		Timestamp: time.Now(),
	}

	if s := libcontainerStats.CgroupStats; s != nil {

	}

	return ret
}