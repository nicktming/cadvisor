package raw

import (
	"github.com/google/cadvisor/fs"
	"github.com/google/cadvisor/container"
	"github.com/google/cadvisor/container/libcontainer"
	"fmt"
	"github.com/google/cadvisor/container/common"
	"k8s.io/klog"
	watch "github.com/google/cadvisor/watcher"
	"flag"
	"strings"
)


var dockerOnly = flag.Bool("docker_only", false, "Only report docker containers in addition to root stats")
var disableRootCgroupStats = flag.Bool("disable_root_cgroup_stats", false, "Disable collecting root Cgroup stats")


type rawFactory struct {
	fsInfo 			fs.FsInfo

	cgroupSubsystems 	*libcontainer.CgroupSubsystems

	watcher 		*common.InotifyWatcher

	includedMetrics 	map[container.MetricKind]struct{}


}

func (f *rawFactory) String() string {
	return "raw"
}

func (f *rawFactory) NewContainerHandler(name string, inHostNamespace bool) (container.ContainerHandler, error) {
	rootFs := "/"
	if !inHostNamespace {
		rootFs = "/rootfs"
	}
	// TODO f.machineInfoFactory
	return newRawContainerHandler(name, f.cgroupSubsystems, f.fsInfo, f.watcher, rootFs, f.includedMetrics)
}

// The raw factory can handle any container. If --docker_only is set to true, non-docker containers are ignored except for "/" and those whitelisted by raw_cgroup_prefix_whitelist flag.
func (f *rawFactory) CanHandleAndAccept(name string) (bool, bool, error) {
	if name == "/" {
		return true, true, nil
	}
	//if *dockerOnly && f.rawPrefixWhiteList[0] == "" {
	//	return true, false, nil
	//}
	//for _, prefix := range f.rawPrefixWhiteList {
	//	if strings.HasPrefix(name, prefix) {
	//		return true, true, nil
	//	}
	//}
	return true, false, nil
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