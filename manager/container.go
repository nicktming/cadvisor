package manager

import (
	"github.com/google/cadvisor/cache/memory"
	"github.com/google/cadvisor/container"
	"time"
	"fmt"
	"k8s.io/utils/clock"
	info "github.com/google/cadvisor/info/v1"
	"sync"
	"flag"
)

var enableLoadReader = flag.Bool("enable_load_reader", false, "Whether to enable cpu load reader")
var HousekeepingInterval = flag.Duration("housekeeping_interval", 1*time.Second, "Interval between container housekeepings")


type containerInfo struct {
	info.ContainerReference
	Subcontainers 		[]info.ContainerReference
	// TODO Spec
	Spec 			info.ContainerSpec
}


type containerData struct {
	handler                  container.ContainerHandler
	info                     containerInfo
	memoryCache              *memory.InMemoryCache
	lock 			 sync.Mutex

	housekeepingInterval     time.Duration
	maxHousekeepingInterval  time.Duration
	allowDynamicHousekeeping bool

	// Whether to log the usage of this container when it is updated.
	logUsage 		 bool
	loadAvg                  float64 // smoothed load average seen so far.

	// Tells the container to stop.
	stop chan struct{}

	//  used to track time
	clock clock.Clock

}

// TODO collectorManager collector.CollectorManager
func newContainerData(containerName string, memoryCache *memory.InMemoryCache, handler container.ContainerHandler, logUsage bool, maxHousekeepingInterval time.Duration, allowDynamicHousekeeping bool, clock clock.Clock) (*containerData, error) {
	//if memoryCache == nil {
	//	return nil, fmt.Errorf("nil memory storage")
	//}
	if handler == nil {
		return nil, fmt.Errorf("nil container handler")
	}
	ref, err := handler.ContainerReference()
	if err != nil {
		return nil, err
	}

	cont := &containerData{
		handler:                  handler,
		memoryCache:              memoryCache,
		housekeepingInterval:     *HousekeepingInterval,
		maxHousekeepingInterval:  maxHousekeepingInterval,
		allowDynamicHousekeeping: allowDynamicHousekeeping,
		logUsage:                 logUsage,
		loadAvg:                  -1.0, // negative value indicates uninitialized.
		stop:                     make(chan struct{}),
		//collectorManager:         collectorManager,
		//onDemandChan:             make(chan chan struct{}, 100),
		clock:                    clock,
		//perfCollector:            &stats.NoopCollector{},
		//nvidiaCollector:          &stats.NoopCollector{},
		//resctrlCollector:         &stats.NoopCollector{},
	}
	cont.info.ContainerReference = ref
	//
	//cont.loadDecay = math.Exp(float64(-cont.housekeepingInterval.Seconds() / 10))
	//
	//if *enableLoadReader {
	//	// Create cpu load reader.
	//	loadReader, err := cpuload.New()
	//	if err != nil {
	//		klog.Warningf("Could not initialize cpu load reader for %q: %s", ref.Name, err)
	//	} else {
	//		cont.loadReader = loadReader
	//	}
	//}
	//
	err = cont.updateSpec()
	if err != nil {
		return nil, err
	}
	//cont.summaryReader, err = summary.New(cont.info.Spec)
	//if err != nil {
	//	cont.summaryReader = nil
	//	klog.V(5).Infof("Failed to create summary reader for %q: %v", ref.Name, err)
	//}

	return cont, nil
}

func (cd *containerData) Start() error {
	//go cd.housekeeping()
	return nil
}

func (cd *containerData) updateSpec() error {
	spec, err := cd.handler.GetSpec()
	if err != nil {
		// TODO exists
		return err
	}
	// TODO customMetrics
	cd.lock.Lock()
	defer cd.lock.Unlock()

	cd.info.Spec = spec
	return nil
}

func (cd *containerData) updateStats() error {

	return nil
}