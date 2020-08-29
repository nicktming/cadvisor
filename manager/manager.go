package manager

import (
	"github.com/google/cadvisor/cache/memory"
	"github.com/google/cadvisor/utils/sysfs"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"k8s.io/klog"
	"os"
	"github.com/google/cadvisor/watcher"
	"github.com/google/cadvisor/container"
	"github.com/google/cadvisor/fs"
	"github.com/google/cadvisor/container/raw"
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
	containerWatchers 	[]watcher.ContainerWatcher
	fsInfo 			fs.FsInfo
	includedMetrics 	container.MetricSet
	eventsChannel 		chan watcher.ContainerEvent
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

	eventsChannel := make(chan watcher.ContainerEvent, 16)

	newManager := &manager{
		containers: 			make(map[namespacedContainerName]*containerData),
		eventsChannel: 			eventsChannel,
	}

	return newManager, nil

}

func (m *manager) Start() error {
	m.containerWatchers = container.InitializePlugins(m.fsInfo, m.includedMetrics)

	err := raw.Register(m.fsInfo, m.includedMetrics)
	if err != nil {
		klog.Errorf("Registration of the raw container factory failed: %v", err)
	}

	rawWatcher, err := raw.NewRawContainerWatcher()
	if err != nil {
		return err
	}
	m.containerWatchers = append(m.containerWatchers, rawWatcher)

	if !container.HasFactories() {
		klog.Infof("there is no factory and exit")
		return nil
	}

	// TODO create quit channels

	quitWatcher := make(chan error)
	err = m.watchForNewContainers(quitWatcher)
	if err != nil {
		return err
	}


	return nil
}

func (m *manager) watchForNewContainers(quit chan error) error {
	for _, watcher := range m.containerWatchers {
		err := watcher.Start(m.eventsChannel)
		if err != nil {
			return err
		}
	}
	// TODO watcher stop

	return nil
}


























































