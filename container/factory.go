package container

import (
	"sync"
	"github.com/google/cadvisor/fs"
	"github.com/google/cadvisor/watcher"
	"k8s.io/klog"
	"fmt"
)


type ContainerHandlerFactory interface {
	// Create a new ContainerHandler using this factory. CanHandleAndAccept() must have returned true.
	NewContainerHandler(name string, inHostNamespace bool) (c ContainerHandler, err error)

	// Returns whether this factory can handle and accept the specified container.
	CanHandleAndAccept(name string) (handle bool, accept bool, err error)

	// Name of the factory.
	String() string
}

type MetricKind string

const (
	CpuUsageMetrics                MetricKind = "cpu"
	ProcessSchedulerMetrics        MetricKind = "sched"
	PerCpuUsageMetrics             MetricKind = "percpu"
	MemoryUsageMetrics             MetricKind = "memory"
	MemoryNumaMetrics              MetricKind = "memory_numa"
	CpuLoadMetrics                 MetricKind = "cpuLoad"
	DiskIOMetrics                  MetricKind = "diskIO"
	DiskUsageMetrics               MetricKind = "disk"
	NetworkUsageMetrics            MetricKind = "network"
	NetworkTcpUsageMetrics         MetricKind = "tcp"
	NetworkAdvancedTcpUsageMetrics MetricKind = "advtcp"
	NetworkUdpUsageMetrics         MetricKind = "udp"
	AcceleratorUsageMetrics        MetricKind = "accelerator"
	AppMetrics                     MetricKind = "app"
	ProcessMetrics                 MetricKind = "process"
	HugetlbUsageMetrics            MetricKind = "hugetlb"
	PerfMetrics                    MetricKind = "perf_event"
	ReferencedMemoryMetrics        MetricKind = "referenced_memory"
	CPUTopologyMetrics             MetricKind = "cpu_topology"
	ResctrlMetrics                 MetricKind = "resctrl"
)

// AllMetrics represents all kinds of metrics that cAdvisor supported.
var AllMetrics = MetricSet{
	CpuUsageMetrics:                struct{}{},
	ProcessSchedulerMetrics:        struct{}{},
	PerCpuUsageMetrics:             struct{}{},
	MemoryUsageMetrics:             struct{}{},
	MemoryNumaMetrics:              struct{}{},
	CpuLoadMetrics:                 struct{}{},
	DiskIOMetrics:                  struct{}{},
	AcceleratorUsageMetrics:        struct{}{},
	DiskUsageMetrics:               struct{}{},
	NetworkUsageMetrics:            struct{}{},
	NetworkTcpUsageMetrics:         struct{}{},
	NetworkAdvancedTcpUsageMetrics: struct{}{},
	NetworkUdpUsageMetrics:         struct{}{},
	ProcessMetrics:                 struct{}{},
	AppMetrics:                     struct{}{},
	HugetlbUsageMetrics:            struct{}{},
	PerfMetrics:                    struct{}{},
	ReferencedMemoryMetrics:        struct{}{},
	CPUTopologyMetrics:             struct{}{},
	ResctrlMetrics:                 struct{}{},
}

func (mk MetricKind) String() string {
	return string(mk)
}

type MetricSet map[MetricKind]struct{}

func (ms MetricSet) Has(mk MetricKind) bool {
	_, exists := ms[mk]
	return exists
}

func (ms MetricSet) Add(mk MetricKind) {
	ms[mk] = struct {}{}
}

func (ms MetricSet) Difference(ms1 MetricSet) MetricSet {
	result := MetricSet{}
	for kind := range ms {
		if !ms1.Has(kind) {
			result.Add(kind)
		}
	}
	return result
}


var pluginsLock sync.Mutex
var plugins = make(map[string]Plugin)

type Plugin interface {

	InitializeFSContext(context *fs.Context) error

	Register(fsInfo fs.FsInfo, includedMetrics MetricSet) (watcher.ContainerWatcher, error)
}

func RegisterPlugin(name string, plugin Plugin) error {
	pluginsLock.Lock()
	defer pluginsLock.Unlock()
	if _, found := plugins[name]; found {
		return fmt.Errorf("Plugin %q was registered twice", name)
	}
	klog.V(4).Infof("Registered Plugin %q", name)
	plugins[name] = plugin
	return nil
}

func InitializePlugins(fsInfo fs.FsInfo, includedMetrics MetricSet) []watcher.ContainerWatcher {
	pluginsLock.Lock()
	defer pluginsLock.Unlock()

	containerWatchers := []watcher.ContainerWatcher{}
	for name, plugin := range plugins {
		watcher, err := plugin.Register(fsInfo, includedMetrics)
		if err != nil {
			klog.Infof("Registration of the %s container factory failed: %v", name, err)
		}
		if watcher != nil {
			containerWatchers = append(containerWatchers, watcher)
		}
	}
	return containerWatchers
}


var (
	factories 	= 	map[watcher.ContainerWatchSource][]ContainerHandlerFactory{}
	factoriesLock 	sync.RWMutex
)

func RegisterContainerHandlerFactory(factory ContainerHandlerFactory, watchTypes []watcher.ContainerWatchSource) {
	factoriesLock.Lock()
	defer factoriesLock.Unlock()

	for _, watchType := range watchTypes {
		factories[watchType] = append(factories[watchType], factory)
	}
}

func HasFactories() bool {
	factoriesLock.Lock()
	defer factoriesLock.Unlock()

	return len(factories) != 0
}

// Create a new ContainerHandler for the specified container.
func NewContainerHandler(name string, watchType watcher.ContainerWatchSource, inHostNamespace bool) (ContainerHandler, bool, error) {
	factoriesLock.RLock()
	defer factoriesLock.RUnlock()

	// Create the ContainerHandler with the first factory that supports it.
	for _, factory := range factories[watchType] {
		canHandle, canAccept, err := factory.CanHandleAndAccept(name)
		if err != nil {
			klog.V(4).Infof("Error trying to work out if we can handle %s: %v", name, err)
		}
		if canHandle {
			if !canAccept {
				klog.V(3).Infof("Factory %q can handle container %q, but ignoring.", factory, name)
				return nil, false, nil
			}
			klog.V(3).Infof("Using factory %q for container %q", factory, name)
			handle, err := factory.NewContainerHandler(name, inHostNamespace)
			return handle, canAccept, err
		}
		klog.V(4).Infof("Factory %q was unable to handle container %q", factory, name)
	}

	return nil, false, fmt.Errorf("no known factory can handle creation of container")
}









































