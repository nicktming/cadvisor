package raw

import (
	"github.com/google/cadvisor/container/libcontainer"
	"github.com/google/cadvisor/container/common"
	"github.com/google/cadvisor/watcher"
	"fmt"
	"k8s.io/klog"
)

type rawContainerWatcher struct {

	cgroupPaths 		map[string]string

	cgroupSubsystems 	*libcontainer.CgroupSubsystems

	watcher 		*common.InotifyWatcher

	stopWatcher 		chan error

}

func NewRawContainerWatcher() (watcher.ContainerWatcher, error) {
	cgroupSubsystems, err := libcontainer.GetAllCgroupSubsystems()

	klog.Infof("======>NewRawContainerWatcher got cgroupSubsystems")
	cgroupSubsystems.String()

	if err != nil {
		return nil, fmt.Errorf("failed to get cgroup subsystems: %v", err)
	}
	if len(cgroupSubsystems.Mounts) == 0 {
		return nil, fmt.Errorf("failed to find supported cgroup mounts for the raw factory")
	}

	watcher, err := common.NewInotifyWatcher()
	if err != nil {
		return nil, err
	}

	rawWatcher := &rawContainerWatcher{
		cgroupPaths:      common.MakeCgroupPaths(cgroupSubsystems.MountPoints, "/"),
		cgroupSubsystems: &cgroupSubsystems,
		watcher:          watcher,
		stopWatcher:      make(chan error),
	}

	return rawWatcher, nil
}

