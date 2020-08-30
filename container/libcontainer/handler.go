package libcontainer

import (
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/google/cadvisor/container"
	info "github.com/google/cadvisor/info/v1"
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
	return nil, nil
}