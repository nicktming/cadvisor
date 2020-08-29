package container

import (
	"sync"
	"github.com/google/cadvisor/fs"
	"github.com/google/cadvisor/watcher"
	"k8s.io/klog"
)


type ContainerHandlerFactory interface {

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








































