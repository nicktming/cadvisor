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

type ContainerStats struct {
	Timestamp time.Time               `json:"timestamp"`
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
