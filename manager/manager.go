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
	"github.com/google/cadvisor/events"
	"strings"
	"strconv"
	"flag"
	"encoding/json"
	"github.com/google/cadvisor/utils/oomparser"
	"github.com/google/cadvisor/machine"
)

var globalHousekeepingInterval = flag.Duration("global_housekeeping_interval", 1*time.Minute, "Interval between global housekeepings")
var updateMachineInfoInterval = flag.Duration("update_machine_info_interval", 5*time.Minute, "Interval between machine info updates.")
var logCadvisorUsage = flag.Bool("log_cadvisor_usage", false, "Whether to log the usage of the cAdvisor container")
var eventStorageAgeLimit = flag.String("event_storage_age_limit", "default=24h", "Max length of time for which to store events (per type). Value is a comma separated list of key values, where the keys are event types (e.g.: creation, oom) or \"default\" and the value is a duration. Default is applied to all non-specified event types")
var eventStorageEventLimit = flag.String("event_storage_event_limit", "default=100000", "Max number of events to store (per type). Value is a comma separated list of key values, where the keys are event types (e.g.: creation, oom) or \"default\" and the value is an integer. Default is applied to all non-specified event types")
var applicationMetricsCountLimit = flag.Int("application_metrics_count_limit", 100, "Max number of application metrics to store (per container)")



type Manager interface {

	Start() error

	CloseEventChannel(watchID int)

	// Get events streamed through passedChannel that fit the request.
	WatchForEvents(request *events.Request) (*events.EventChannel, error)

	// Get past events that have been detected and that fit the request.
	GetPastEvents(request *events.Request) ([]*info.Event, error)

	GetContainerInfo(containerName string, query *info.ContainerInfoRequest) (*info.ContainerInfo, error)

	// Get information about the machine.
	GetMachineInfo() (*info.MachineInfo, error)

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
	eventHandler             events.EventManager
	machineMu                sync.RWMutex // protects machineInfo

	machineInfo              info.MachineInfo

	sysFs                    sysfs.SysFs
}

func New(memoryCache *memory.InMemoryCache, sysfs sysfs.SysFs, includedMetricsSet container.MetricSet) (Manager, error) {
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
		includedMetrics: 		includedMetricsSet,
		sysFs: 				sysfs,
	}

	newManager.eventHandler = events.NewEventManager(parseEventsStoragePolicy())
	return newManager, nil

}

// Parses the events StoragePolicy from the flags.
func parseEventsStoragePolicy() events.StoragePolicy {
	policy := events.DefaultStoragePolicy()

	// Parse max age.
	parts := strings.Split(*eventStorageAgeLimit, ",")
	for _, part := range parts {
		items := strings.Split(part, "=")
		if len(items) != 2 {
			klog.Warningf("Unknown event storage policy %q when parsing max age", part)
			continue
		}
		dur, err := time.ParseDuration(items[1])
		if err != nil {
			klog.Warningf("Unable to parse event max age duration %q: %v", items[1], err)
			continue
		}
		if items[0] == "default" {
			policy.DefaultMaxAge = dur
			continue
		}
		policy.PerTypeMaxAge[info.EventType(items[0])] = dur
	}

	// Parse max number.
	parts = strings.Split(*eventStorageEventLimit, ",")
	for _, part := range parts {
		items := strings.Split(part, "=")
		if len(items) != 2 {
			klog.Warningf("Unknown event storage policy %q when parsing max event limit", part)
			continue
		}
		val, err := strconv.Atoi(items[1])
		if err != nil {
			klog.Warningf("Unable to parse integer from %q: %v", items[1], err)
			continue
		}
		if items[0] == "default" {
			policy.DefaultMaxNumEvents = val
			continue
		}
		policy.PerTypeMaxNumEvents[info.EventType(items[0])] = val
	}

	return policy
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

	err = m.watchForNewOoms()
	if err != nil {
		klog.Warningf("Could not configure a source for OOM detection, disabling OOM events: %v", err)
	}

	if !container.HasFactories() {
		klog.Infof("there is no factory and exit")
		return nil
	}

	// TODO create quit channels
	// Create root and then recover all containers.
	err = m.createContainer("/", watcher.Raw)
	if err != nil {
		return err
	}
	klog.Infof("Starting recovery of all containers")
	//err = m.detectSubcontainers("/")
	//if err != nil {
	//	return err
	//}
	//klog.V(2).Infof("Recovery completed")


	quitWatcher := make(chan error)
	err = m.watchForNewContainers(quitWatcher)
	if err != nil {
		return err
	}
	return nil
}

func (m *manager) watchForNewOoms() error {
	klog.Infof("Started watching for new ooms in manager")
	outStream := make(chan *oomparser.OomInstance, 10)
	oomLog, err := oomparser.New()
	if err != nil {
		return err
	}
	go oomLog.StreamOoms(outStream)
	go func() {
		for oomInstance := range outStream {

			newEvent := &info.Event{
				ContainerName: 		oomInstance.ContainerName,
				Timestamp: 		oomInstance.TimeOfDeath,
				EventType: 		info.EventOom,
			}

			err := m.eventHandler.AddEvent(newEvent)
			if err != nil {
				klog.Errorf("failed to add OOM event for %q: %v", oomInstance.ContainerName, err)
			}

			klog.Infof("Created an OOM event in container %q at %v", oomInstance.ContainerName, oomInstance.TimeOfDeath)

			newEvent = &info.Event{
				ContainerName: 		oomInstance.VictimContainerName,
				Timestamp: 		oomInstance.TimeOfDeath,
				EventType: 		info.EventOomKill,
				EventData: 		info.EventData{
					OomKill: &info.OomKillEventData{
						Pid: 		oomInstance.Pid,
						ProcessName: 	oomInstance.ProcessName,
					},
				},
			}
			err = m.eventHandler.AddEvent(newEvent)
			if err != nil {
				klog.Infof("failed to add OOM kill event for %q: %v", oomInstance.ContainerName, err)
			}
		}
	}()
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
	//err := m.detectSubcontainers("/")
	//if err != nil {
	//	return err
	//}


	go func(){
		for {
			select {
			case event := <-m.eventsChannel:
				switch {
				case event.EventType == watcher.ContainerAdd:
					// TODO create container
					klog.Infof("watchForNewContainers (watcher.ContainerAdd) event name: %v, watchSource: %v", event.Name, event.WatchSource)

				case event.EventType == watcher.ContainerDelete:
					klog.Infof("watchForNewContainers (watcher.ContainerDelete) event name: %v, watchSource: %v", event.Name, event.WatchSource)
				}

			case <-quit:
				var errs partialFailure

				for i, watcher := range m.containerWatchers {
					err := watcher.Stop()
					if err != nil {
						errs.append(fmt.Sprintf("watcher %d", i), "Stop", err)
					}
				}

				if len(errs) > 0 {
					quit <- errs
				} else {
					quit <- nil
					klog.Infof("Exiting thread watching subcontainers")
					return
				}
			}
		}
	}()

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
	_, _, err := m.getContainersDiff(containerName)
	//added, removed, err := m.getContainersDiff(containerName)
	if err != nil {
		return err
	}

	// Add the new containers.
	//for _, cont := range added {
		//klog.Infof("Add container name: %v", cont.Name)
		//err = m.createContainer(cont.Name, watcher.Raw)
		//if err != nil {
		//	klog.Errorf("Failed to create existing container: %s: %s", cont.Name, err)
		//}
	//}

	// Remove the old containers.
	//for _, cont := range removed {
	//	klog.Infof("Remove container name: %v", cont.Name)
		//err = m.destroyContainer(cont.Name)
		//if err != nil {
		//	klog.Errorf("Failed to destroy existing container: %s: %s", cont.Name, err)
		//}
	//}

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

	if _, ok := m.containers[namespacedName]; ok {
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
	cont, err := newContainerData(containerName, m.memoryCache, handler, logUsage, 5 * time.Second, false, clock.RealClock{})
	if err != nil {
		return err
	}

	// TODO a lot of things
	m.containers[namespacedName] = cont
	// TODO alias
	//klog.Infof("Added container: %q (aliases: %v, namespace: %q)", containerName, cont.info.Aliases, cont.info.Namespace)

	// TODO GetSpec ContainerReference
	contSpec, err := cont.handler.GetSpec()
	if err != nil {
		return err
	}

	pretty_contSpec, _ := json.MarshalIndent(contSpec, "", "\t")
	klog.Infof("+++++++++++++++++contSpec: %v", string(pretty_contSpec))

	contRef, err := cont.handler.ContainerReference()
	if err != nil {
		return err
	}

	newEvent := &info.Event{
		ContainerName: contRef.Name,
		Timestamp:     contSpec.CreationTime,
		EventType:     info.EventContainerCreation,
	}
	err = m.eventHandler.AddEvent(newEvent)
	if err != nil {
		return err
	}

	return cont.Start()
}

func (m *manager) GetContainerInfo(containerName string, query *info.ContainerInfoRequest) (*info.ContainerInfo, error) {
	cont, err := m.getContainerData(containerName)
	if err != nil {
		return nil, err
	}
	return m.containerDataToContainerInfo(cont, query)
}

func (m *manager) containerDataToContainerInfo(cont *containerData, query *info.ContainerInfoRequest) (*info.ContainerInfo, error) {
	cinfo, err := cont.GetInfo(true)
	if err != nil {
		return nil, err
	}
	stats, err := m.memoryCache.RecentStats(cinfo.Name, query.Start, query.End, query.NumStats)
	if err != nil {
		return nil, err
	}
	ret := &info.ContainerInfo{
		ContainerReference: 	cinfo.ContainerReference,
		Subcontainers: 		cinfo.Subcontainers,
		Spec:			m.getAdjustedSpec(cinfo),
		Stats:			stats,
	}
	return ret, nil
}

func (m *manager) getAdjustedSpec(cinfo *containerInfo) info.ContainerSpec {
	spec := cinfo.Spec

	if spec.HasMemory {
		if spec.Memory.Limit == 0 {
			klog.Infof("set memory limit since spec.Memory.Limit = 0")
		}
	}
	return spec
}

func (m *manager) getContainerData(containerName string) (*containerData, error) {
	var cont *containerData
	var ok bool
	func() {
		m.containersLock.RLock()
		defer m.containersLock.RUnlock()

		cont, ok = m.containers[namespacedContainerName{
			Name: containerName,
		}]
	}()

	if !ok {
		return nil, fmt.Errorf("unknow container %q", containerName)
	}
	return cont, nil
}


func (m *manager) GetMachineInfo() (*info.MachineInfo, error) {
	m.machineMu.RLock()
	defer m.machineMu.RUnlock()
	return m.machineInfo.Clone(), nil
}

func (m *manager) updateMachineInfo(quit chan error) {
	ticker := time.NewTicker(*updateMachineInfoInterval)
	for {
		select {
		case <-ticker.C:
			info, err := machine.Info(m.sysFs, m.fsInfo, m.inHostNamespace)
			if err != nil {
				klog.Errorf("Could not get machine info: %v", err)
				break
			}
			m.machineMu.Lock()
			m.machineInfo = *info
			m.machineMu.Unlock()
			klog.Infof("Update machine info: %+v", *info)

		case <-quit:
			ticker.Stop()
			quit <- nil
			return
		}
	}
}

func (m *manager) WatchForEvents(request *events.Request) (*events.EventChannel, error) {
	return m.eventHandler.WatchEvents(request)
}

// can be called by the api which will return all events satisfying the request
func (m *manager) GetPastEvents(request *events.Request) ([]*info.Event, error) {
	return m.eventHandler.GetEvents(request)
}

// called by the api when a client is no longer listening to the channel
func (m *manager) CloseEventChannel(watchID int) {
	m.eventHandler.StopWatch(watchID)
}

type partialFailure []string

func (f *partialFailure) append(id, operation string, err error) {
	*f = append(*f, fmt.Sprintf("[%q: %s: %s]", id, operation, err))
}

func (f partialFailure) Error() string {
	return fmt.Sprintf("partial failures: %s", strings.Join(f, ", "))
}

func (f partialFailure) OrNil() error {
	if len(f) == 0 {
		return nil
	}
	return f
}





















































