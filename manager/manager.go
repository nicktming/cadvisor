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
	"sync"
	"k8s.io/utils/clock"
	"time"
	info "github.com/google/cadvisor/info/v1"
	"fmt"
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
	containersLock 		sync.RWMutex
	containerWatchers 	[]watcher.ContainerWatcher
	fsInfo 			fs.FsInfo
	includedMetrics 	container.MetricSet
	eventsChannel 		chan watcher.ContainerEvent
	inHostNamespace          bool
	memoryCache              *memory.InMemoryCache
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
		inHostNamespace: 		inHostNamespace,
		memoryCache: 			memoryCache,
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
	watched := make([]watcher.ContainerWatcher, 0)
	for _, watcher := range m.containerWatchers {
		err := watcher.Start(m.eventsChannel)
		if err != nil {
			for _, w := range watched {
				err = w.Stop()
				klog.Warningf("Failed to stop wacher: %v", w)
			}
			return err
		}
		watched = append(watched, watcher)
	}
	// TODO watcher stop

	// There is a race between starting the watch and new container creation so we do a detection before we read new containers.
	err := m.detectSubcontainers("/")
	if err != nil {
		return err
	}

	return nil
}

func (m *manager) getContainersDiff(containerName string) (added []info.ContainerReference, removed []info.ContainerReference, err error) {
	// Get all subcontainers recursively
	m.containersLock.RLock()
	cont, ok := m.containers[namespacedContainerName{
		Name:		containerName,
	}]
	m.containersLock.RUnlock()
	if !ok {
		return nil, nil, fmt.Errorf("failed to find container %q while checking for new containers", containerName)
	}
	allContainers, err := cont.handler.ListContainers(container.ListRecursive)

	if err != nil {
		return nil, nil, err
	}
	allContainers = append(allContainers, info.ContainerReference{Name: containerName})

	klog.Infof("allContainers: %v", allContainers)

	m.containersLock.RLock()
	defer m.containersLock.RUnlock()

	// Determine which were added and which were removed.
	allContainersSet := make(map[string]*containerData)
	for name, d := range m.containers {
		// Only add the canonical name.
		if d.info.Name == name.Name {
			allContainersSet[name.Name] = d
		}
	}

	// Added containers
	for _, c := range allContainers {
		delete(allContainersSet, c.Name)
		_, ok := m.containers[namespacedContainerName{
			Name: c.Name,
		}]
		if !ok {
			added = append(added, c)
		}
	}

	// Removed ones are no longer in the container listing.
	for _, d := range allContainersSet {
		removed = append(removed, d.info.ContainerReference)
	}

	return
}

func (m *manager) detectSubcontainers(containerName string) error {
	added, removed, err := m.getContainersDiff(containerName)
	if err != nil {
		return err
	}

	// Add the new containers.
	for _, cont := range added {
		klog.Infof("Add container name: %v", cont.Name)
		//err = m.createContainer(cont.Name, watcher.Raw)
		//if err != nil {
		//	klog.Errorf("Failed to create existing container: %s: %s", cont.Name, err)
		//}
	}

	// Remove the old containers.
	for _, cont := range removed {
		klog.Infof("Remove container name: %v", cont.Name)
		//err = m.destroyContainer(cont.Name)
		//if err != nil {
		//	klog.Errorf("Failed to destroy existing container: %s: %s", cont.Name, err)
		//}
	}

	return nil
}


func (m *manager) createContainer(containerName string, watchSource watcher.ContainerWatchSource) error {
	m.containersLock.Lock()
	defer m.containersLock.Unlock()

	return m.createContainerLocked(containerName, watchSource)
}

func (m *manager) createContainerLocked(containerName string, watchSource watcher.ContainerWatchSource) error {
	namespacedName := namespacedContainerName{
		Name: 		containerName,
	}

	if _, ok := m.containers[containerName]; ok {
		return nil
	}

	handler, accept, err := container.NewContainerHandler(containerName, watchSource, m.inHostNamespace)
	if err != nil {
		return err
	}

	if !accept {
		klog.Infof("ignoring container %q", containerName)
		return nil
	}

	// TODO logUsage collectManager
	logUsage := false
	cont, err := newContainerData(containerName, m.memoryCache, handler, logUsage, 5 * time.Second, 10 * time.Second, clock.RealClock{})
	if err != nil {
		return err
	}

	// TODO a lot of things
	m.containers[namespacedName] = cont
	// TODO alias
	//klog.Infof("Added container: %q (aliases: %v, namespace: %q)", containerName, cont.info.Aliases, cont.info.Namespace)

	// TODO GetSpec ContainerReference

	return nil
}

























































