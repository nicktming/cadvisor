package raw

import (
	"github.com/google/cadvisor/fs"
	"github.com/google/cadvisor/container"
	"github.com/google/cadvisor/container/libcontainer"
	"fmt"
	"github.com/google/cadvisor/container/common"
	"k8s.io/klog"
	watch "github.com/google/cadvisor/watcher"
)

type rawFactory struct {
	fsInfo 			fs.FsInfo

	cgroupSubsystems 	*libcontainer.CgroupSubsystems

	watcher 		*common.InotifyWatcher

	includedMetrics 	map[container.MetricKind]struct{}


}

func (f *rawFactory) String() string {
	return "raw"
}

func Register(fsInfo fs.FsInfo, includedMetrics map[container.MetricKind]struct{}) error {
	cgroupSubsystems, err := libcontainer.GetCgroupSubsystems(includedMetrics)
	if err != nil {
		return fmt.Errorf("failed to get cgroup subsystems: %v", err)
	}
	if len(cgroupSubsystems.Mounts) == 0 {
		return fmt.Errorf("failed to find supported cgroup mounts for the raw factory")
	}

	watcher, err := common.NewInotifyWatcher()
	if err != nil {
		return err
	}

	klog.Infof("Registering Raw factory")
	factory := &rawFactory{
		fsInfo: 		fsInfo,
		cgroupSubsystems: 	&cgroupSubsystems,
		watcher:		watcher,
		includedMetrics: 	includedMetrics,
	}
	container.RegisterContainerHandlerFactory(factory, []watch.ContainerWatchSource{watch.Raw})
	return nil
}