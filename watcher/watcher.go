package watcher

type ContainerEventType int

const (
	ContainerAdd 	ContainerEventType = iota
	ContainerDelete
)

type ContainerWatchSource int

const (
	Raw ContainerWatchSource = iota
)

type ContainerEvent struct {
	EventType 	ContainerEventType

	Name 		string

	WatchSource 	ContainerWatchSource
}

type ContainerWatcher interface {

	//Start(events chan ContainerEvent) error
	//
	//Stop() error
}
