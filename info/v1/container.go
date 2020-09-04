package v1

import "time"

type ContainerReference struct {
	// The container id
	Id string `json:"id,omitempty"`

	// The absolute name of the container. This is unique on the machine.
	Name string `json:"name"`

	// Other names by which the container is known within a certain namespace.
	// This is unique within that namespace.
	Aliases []string `json:"aliases,omitempty"`

	// Namespace under which the aliases of a container are unique.
	// An example of a namespace is "docker" for Docker containers.
	Namespace string `json:"namespace,omitempty"`
}

// CPU usage time statistics.
type CpuUsage struct {
	// Total CPU usage.
	// Unit: nanoseconds.
	Total uint64 `json:"total"`

	// Per CPU/core usage of the container.
	// Unit: nanoseconds.
	PerCpu []uint64 `json:"per_cpu_usage,omitempty"`

	// Time spent in user space.
	// Unit: nanoseconds.
	User uint64 `json:"user"`

	// Time spent in kernel space.
	// Unit: nanoseconds.
	System uint64 `json:"system"`
}

// Cpu Completely Fair Scheduler statistics.
type CpuCFS struct {
	// Total number of elapsed enforcement intervals.
	Periods uint64 `json:"periods"`

	// Total number of times tasks in the cgroup have been throttled.
	ThrottledPeriods uint64 `json:"throttled_periods"`

	// Total time duration for which tasks in the cgroup have been throttled.
	// Unit: nanoseconds.
	ThrottledTime uint64 `json:"throttled_time"`
}

// All CPU usage metrics are cumulative from the creation of the container
type CpuStats struct {
	Usage     CpuUsage     `json:"usage"`
	CFS       CpuCFS       `json:"cfs"`
	Schedstat CpuSchedstat `json:"schedstat"`
	// Smoothed average of number of runnable threads x 1000.
	// We multiply by thousand to avoid using floats, but preserving precision.
	// Load is smoothed over the last 10 seconds. Instantaneous value can be read
	// from LoadStats.NrRunning.
	LoadAverage int32 `json:"load_average"`
}

type MemoryStats struct {
	// Current memory usage, this includes all memory regardless of when it was
	// accessed.
	// Units: Bytes.
	Usage uint64 `json:"usage"`

	// Maximum memory usage recorded.
	// Units: Bytes.
	MaxUsage uint64 `json:"max_usage"`

	// Number of bytes of page cache memory.
	// Units: Bytes.
	Cache uint64 `json:"cache"`

	// The amount of anonymous and swap cache memory (includes transparent
	// hugepages).
	// Units: Bytes.
	RSS uint64 `json:"rss"`

	// The amount of swap currently used by the processes in this cgroup
	// Units: Bytes.
	Swap uint64 `json:"swap"`

	// The amount of memory used for mapped files (includes tmpfs/shmem)
	MappedFile uint64 `json:"mapped_file"`

	// The amount of working set memory, this includes recently accessed memory,
	// dirty memory, and kernel memory. Working set is <= "usage".
	// Units: Bytes.
	WorkingSet uint64 `json:"working_set"`

	Failcnt uint64 `json:"failcnt"`

	ContainerData    MemoryStatsMemoryData `json:"container_data,omitempty"`
	HierarchicalData MemoryStatsMemoryData `json:"hierarchical_data,omitempty"`
}

type MemoryNumaStats struct {
	File        map[uint8]uint64 `json:"file,omitempty"`
	Anon        map[uint8]uint64 `json:"anon,omitempty"`
	Unevictable map[uint8]uint64 `json:"unevictable,omitempty"`
}

type MemoryStatsMemoryData struct {
	Pgfault    uint64          `json:"pgfault"`
	Pgmajfault uint64          `json:"pgmajfault"`
	NumaStats  MemoryNumaStats `json:"numa_stats,omitempty"`
}

type ContainerStats struct {
	Timestamp time.Time               `json:"timestamp"`
	Cpu       CpuStats                `json:"cpu,omitempty"`
	Memory    MemoryStats             `json:"memory,omitempty"`
}

// Cpu Aggregated scheduler statistics
type CpuSchedstat struct {
	// https://www.kernel.org/doc/Documentation/scheduler/sched-stats.txt

	// time spent on the cpu
	RunTime uint64 `json:"run_time"`
	// time spent waiting on a runqueue
	RunqueueTime uint64 `json:"runqueue_time"`
	// # of timeslices run on this cpu
	RunPeriods uint64 `json:"run_periods"`
}

type CpuSpec struct {
	Limit    uint64 `json:"limit"`
	MaxLimit uint64 `json:"max_limit"`
	Mask     string `json:"mask,omitempty"`
	Quota    uint64 `json:"quota,omitempty"`
	Period   uint64 `json:"period,omitempty"`
}

type MemorySpec struct {
	// The amount of memory requested. Default is unlimited (-1).
	// Units: bytes.
	Limit uint64 `json:"limit,omitempty"`

	// The amount of guaranteed memory.  Default is 0.
	// Units: bytes.
	Reservation uint64 `json:"reservation,omitempty"`

	// The amount of swap space requested. Default is unlimited (-1).
	// Units: bytes.
	SwapLimit uint64 `json:"swap_limit,omitempty"`
}

type ProcessSpec struct {
	Limit uint64 `json:"limit,omitempty"`
}

type ContainerSpec struct {
	// Time at which the container was created.
	CreationTime time.Time `json:"creation_time,omitempty"`

	// Metadata labels associated with this container.
	Labels map[string]string `json:"labels,omitempty"`
	// Metadata envs associated with this container. Only whitelisted envs are added.
	Envs map[string]string `json:"envs,omitempty"`

	HasCpu bool    `json:"has_cpu"`
	Cpu    CpuSpec `json:"cpu,omitempty"`

	HasMemory bool       `json:"has_memory"`
	Memory    MemorySpec `json:"memory,omitempty"`

	HasHugetlb bool `json:"has_hugetlb"`

	HasNetwork bool `json:"has_network"`

	HasProcesses bool        `json:"has_processes"`
	Processes    ProcessSpec `json:"processes,omitempty"`

	HasFilesystem bool `json:"has_filesystem"`

	// HasDiskIo when true, indicates that DiskIo stats will be available.
	HasDiskIo bool `json:"has_diskio"`

	HasCustomMetrics bool         `json:"has_custom_metrics"`
	CustomMetrics    []MetricSpec `json:"custom_metrics,omitempty"`

	// Image name used for this container.
	Image string `json:"image,omitempty"`
}

// Event contains information general to events such as the time at which they
// occurred, their specific type, and the actual event. Event types are
// differentiated by the EventType field of Event.
type Event struct {
	// the absolute container name for which the event occurred
	ContainerName string `json:"container_name"`

	// the time at which the event occurred
	Timestamp time.Time `json:"timestamp"`

	// the type of event. EventType is an enumerated type
	EventType EventType `json:"event_type"`

	// the original event object and all of its extraneous data, ex. an
	// OomInstance
	EventData EventData `json:"event_data,omitempty"`
}

// EventType is an enumerated type which lists the categories under which
// events may fall. The Event field EventType is populated by this enum.
type EventType string

const (
	EventOom               EventType = "oom"
	EventOomKill           EventType = "oomKill"
	EventContainerCreation EventType = "containerCreation"
	EventContainerDeletion EventType = "containerDeletion"
)

// Extra information about an event. Only one type will be set.
type EventData struct {
	// Information about an OOM kill event.
	OomKill *OomKillEventData `json:"oom,omitempty"`
}

// Information related to an OOM kill instance
type OomKillEventData struct {
	// process id of the killed process
	Pid int `json:"pid"`

	// The name of the killed process
	ProcessName string `json:"process_name"`
}

type ContainerInfo struct {
	ContainerReference

	// The direct subcontainers of the current container.
	Subcontainers []ContainerReference `json:"subcontainers,omitempty"`

	// The isolation used in the container.
	Spec ContainerSpec `json:"spec,omitempty"`

	// Historical statistics gathered from the container.
	Stats []*ContainerStats `json:"stats,omitempty"`
}
