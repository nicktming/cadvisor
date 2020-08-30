package raw

import (
	"github.com/google/cadvisor/fs"
	"github.com/google/cadvisor/container/common"
	"github.com/google/cadvisor/container"
	"github.com/google/cadvisor/container/libcontainer"
	"log"
	info "github.com/google/cadvisor/info/v1"
	"k8s.io/klog"
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

func (h *rawContainerHandler) ContainerReference() (info.ContainerReference, error) {
	// We only know the container by its one name.
	return info.ContainerReference{
		Name: h.name,
	}, nil
}

// Nothing to start up.
func (h *rawContainerHandler) Start() {}

// Nothing to clean up.
func (h *rawContainerHandler) Cleanup() {}

func (h *rawContainerHandler) GetSpec() (info.ContainerSpec, error) {
	const hasNetwork = false
	hasFilesystem := isRootCgroup(h.name) || len(h.externalMounts) > 0
	// TODO
	spec, err := common.GetSpec(h.cgroupPaths, hasNetwork, hasFilesystem)
	if err != nil {
		return spec, err
	}

	if isRootCgroup(h.name) {
		// Check physical network devices for root container.
		//nd, err := h.GetRootNetworkDevices()
		//if err != nil {
		//	return spec, err
		//}
		//spec.HasNetwork = spec.HasNetwork || len(nd) != 0
		//
		//// Get memory and swap limits of the running machine
		//memLimit, err := machine.GetMachineMemoryCapacity()
		//if err != nil {
		//	klog.Warningf("failed to obtain memory limit for machine container")
		//	spec.HasMemory = false
		//} else {
		//	spec.Memory.Limit = uint64(memLimit)
		//	// Spec is marked to have memory only if the memory limit is set
		//	spec.HasMemory = true
		//}
		//
		//swapLimit, err := machine.GetMachineSwapCapacity()
		//if err != nil {
		//	klog.Warningf("failed to obtain swap limit for machine container")
		//} else {
		//	spec.Memory.SwapLimit = uint64(swapLimit)
		//}
	}

	return spec, nil
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

