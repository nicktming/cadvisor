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
