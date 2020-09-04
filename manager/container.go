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
	"k8s.io/klog"
	units "github.com/docker/go-units"
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

	statsLastUpdatedTime     time.Time


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
	go cd.housekeeping()
	return nil
}

// TODO(vmarmol): Implement stats collecting as a custom collector.
func (cd *containerData) housekeeping() {
	// Start any background goroutines - must be cleaned up in cd.handler.Cleanup().
	cd.handler.Start()
	defer cd.handler.Cleanup()

	// Initialize cpuload reader - must be cleaned up in cd.loadReader.Stop()
	//if cd.loadReader != nil {
	//	err := cd.loadReader.Start()
	//	if err != nil {
	//		klog.Warningf("Could not start cpu load stat collector for %q: %s", cd.info.Name, err)
	//	}
	//	defer cd.loadReader.Stop()
	//}

	// Long housekeeping is either 100ms or half of the housekeeping interval.
	longHousekeeping := 100 * time.Millisecond
	if *HousekeepingInterval/2 < longHousekeeping {
		longHousekeeping = *HousekeepingInterval / 2
	}

	// Housekeep every second.
	klog.V(3).Infof("Start housekeeping for container %q\n", cd.info.Name)
	houseKeepingTimer := cd.clock.NewTimer(0 * time.Second)
	defer houseKeepingTimer.Stop()
	for {
		if !cd.housekeepingTick(houseKeepingTimer.C(), longHousekeeping) {
			return
		}
		// Stop and drain the timer so that it is safe to reset it
		if !houseKeepingTimer.Stop() {
			select {
			case <-houseKeepingTimer.C():
			default:
			}
		}
		// Log usage if asked to do so.
		if cd.logUsage {
			const numSamples = 60
			var empty time.Time
			stats, err := cd.memoryCache.RecentStats(cd.info.Name, empty, empty, numSamples)
			if err != nil {
				klog.Warningf("[%s] Failed to get recent stats for logging usage: %v", cd.info.Name, err)

			} else if len(stats) < numSamples {
				// Ignore, not enough stats yet.
			} else {
				usageCPUNs := uint64(0)
				for i := range stats {
					if i > 0 {
						usageCPUNs += (stats[i].Cpu.Usage.Total - stats[i-1].Cpu.Usage.Total)
					}
				}
				usageMemory := stats[numSamples-1].Memory.Usage

				instantUsageInCores := float64(stats[numSamples-1].Cpu.Usage.Total-stats[numSamples-2].Cpu.Usage.Total) / float64(stats[numSamples-1].Timestamp.Sub(stats[numSamples-2].Timestamp).Nanoseconds())
				usageInCores := float64(usageCPUNs) / float64(stats[numSamples-1].Timestamp.Sub(stats[0].Timestamp).Nanoseconds())
				usageInHuman := units.HumanSize(float64(usageMemory))
				// Don't set verbosity since this is already protected by the logUsage flag.
				klog.Infof("[%s] %.3f cores (average: %.3f cores), %s of memory", cd.info.Name, instantUsageInCores, usageInCores, usageInHuman)
			}
		}
		houseKeepingTimer.Reset(5 * time.Second)
	}
}

func (cd *containerData) housekeepingTick(timer <-chan time.Time, longHousekeeping time.Duration) bool {
	//select {
	//case <-cd.stop:
	//// Stop housekeeping when signaled.
	//	return false
	//case finishedChan := <-cd.onDemandChan:
	//// notify the calling function once housekeeping has completed
	//	defer close(finishedChan)
	//case <-timer:
	//}
	start := cd.clock.Now()
	err := cd.updateStats()
	if err != nil {
		klog.Warningf("Failed to update stats for container \"%s\": %s", cd.info.Name, err)
	}
	// Log if housekeeping took too long.
	duration := cd.clock.Since(start)
	if duration >= longHousekeeping {
		klog.V(3).Infof("[%s] Housekeeping took %s", cd.info.Name, duration)
	}
	//cd.notifyOnDemand()
	cd.statsLastUpdatedTime = cd.clock.Now()
	return true
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
	stats, statsErr := cd.handler.GetStats()
	if statsErr != nil {
		// Ignore errors if the container is dead.
		if !cd.handler.Exists() {
			return nil
		}

		// Stats may be partially populated, push those before we return an error.
		statsErr = fmt.Errorf("%v, continuing to push stats", statsErr)
	}
	if stats == nil {
		return statsErr
	}
	// TODO loadReader
	//if cd.loadReader != nil {
	//	// TODO(vmarmol): Cache this path.
	//	path, err := cd.handler.GetCgroupPath("cpu")
	//	if err == nil {
	//		loadStats, err := cd.loadReader.GetCpuLoad(cd.info.Name, path)
	//		if err != nil {
	//			return fmt.Errorf("failed to get load stat for %q - path %q, error %s", cd.info.Name, path, err)
	//		}
	//		stats.TaskStats = loadStats
	//		cd.updateLoad(loadStats.NrRunning)
	//		// convert to 'milliLoad' to avoid floats and preserve precision.
	//		stats.Cpu.LoadAverage = int32(cd.loadAvg * 1000)
	//	}
	//}

	// TODO summaryReader collectorManager nvidiaStatsErr perfCollector resctrlCollector

	ref, err := cd.handler.ContainerReference()
	if err != nil {
		if !cd.handler.Exists() {
			return nil
		}
		return err
	}

	cInfo := info.ContainerInfo{
		ContainerReference: ref,
	}

	err = cd.memoryCache.AddStats(&cInfo, stats)

	// TODO a lot of error

	return err
}