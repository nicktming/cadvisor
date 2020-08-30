package raw

import (
	"github.com/google/cadvisor/fs"
	"github.com/google/cadvisor/container/common"
	"github.com/google/cadvisor/container"
	"github.com/google/cadvisor/container/libcontainer"
	"log"
	info "github.com/google/cadvisor/info/v1"
)

type rawContainerHandler struct {
	// name of the container for this handler
	name 			string
	// TODO machineInfoFactory

	cgroupPaths 		map[string]string
	fsInfo 			fs.FsInfo
	externalMounts 		[]common.Mount
	includedMetrics 	container.MetricSet

	libcontainerHandler 	*libcontainer.Handler
}

func isRootCgroup(name string) bool {
	return name == "/"
}

func (h *rawContainerHandler) ListContainers(listType container.ListType) ([]info.ContainerReference, error) {
	return common.ListContainers(h.name, h.cgroupPaths, listType)
}

// TODO machineInfoFactory info.MachineInfoFactory
func newRawContainerHandler(name string, cgroupSubsystems *libcontainer.CgroupSubsystems, fsInfo fs.FsInfo, watcher *common.InotifyWatcher, rootFs string, includedMetrics container.MetricSet) (container.ContainerHandler, error) {
	cHints, err := common.GetContainerHintsFromFile(*common.ArgContainerHints)
	if err != nil {
		return nil, err
	}

	cgroupPaths := common.MakeCgroupPaths(cgroupSubsystems.MountPoints, name)

	log.Printf("name: %v, cgroupPaths: %v\n", name, cgroupPaths)

	cgroupManager, err := libcontainer.NewCgroupManager(name, cgroupPaths)
	if err != nil {
		return nil, err
	}

	var externalMounts []common.Mount
	for _, container := range cHints.AllHosts {
		if name == container.FullName {
			externalMounts = container.Mounts
			break
		}
	}

	pid := 0
	if isRootCgroup(name) {
		pid = 1

		// delete pids from cgroup paths because /sys/fs/cgroup/pids/pids.current not exist
		delete(cgroupPaths, "pids")
	}

	handler := libcontainer.NewHandler(cgroupManager, rootFs, pid, includedMetrics)

	return &rawContainerHandler{
		name: 			name,
		// TODO machine
		cgroupPaths: 		cgroupPaths,
		fsInfo: 		fsInfo,
		externalMounts: 	externalMounts,
		includedMetrics: 	includedMetrics,
		libcontainerHandler: 	handler,
	}, nil
}

func (h *rawContainerHandler) GetStats() (*info.ContainerStats, error) {
	//if *disableRootCgroupStats && isRootCgroup(h.name) {
	//	return nil, nil
	//}
	stats, err := h.libcontainerHandler.GetStats()
	if err != nil {
		return stats, err
	}

	// Get filesystem stats.
	//err = h.getFsStats(stats)
	//if err != nil {
	//	return stats, err
	//}

	//return stats, nil

	return nil, nil
}

