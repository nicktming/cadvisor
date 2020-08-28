package manager

import (
	"github.com/google/cadvisor/cache/memory"
	"github.com/google/cadvisor/utils/sysfs"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"k8s.io/klog"
	"os"
)

type Manager interface {
	Start() error
}

type namespacedContainerName struct {
	Namespace 		string
	Name 			string
}

type manager struct {
	containers 		map[namespacedContainerName]*containerData
}

func New(memoryCache *memory.InMemoryCache, sysfs sysfs.SysFs) (Manager, error) {
	//if memoryCache == nil {
	//	return nil, fmt.Errorf("manager requires memory storage")
	//}

	selfContainer := "/"
	//var err error

	if cgroups.IsCgroup2UnifiedMode() {
		klog.Warningf("Cannot detect current cgroup on cgroup v2")
	} else {
		selfContainer, err := cgroups.GetOwnCgroupPath("cpu")
		if err != nil {
			return nil, err
		}
		klog.Infof("cadvisor running in container: %q", selfContainer)
	}

	inHostNamespace := false
	if _, err := os.Stat("/rootfs/proc"); os.IsNotExist(err) {
		inHostNamespace = true
	}

	klog.Infof("cadvisor with inHostNamespace: %v and selfContainer: %v", inHostNamespace, selfContainer)

	newManager := &manager{
		containers: 			make(map[namespacedContainerName]*containerData),
	}

	return newManager, nil

}