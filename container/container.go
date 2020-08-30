package container

import info "github.com/google/cadvisor/info/v1"

type ListType int

const (
	ListSelf 	ListType = iota
	ListRecursive
)

type ContainerHandler interface {
	// Returns the ContainerReference
	ContainerReference() (info.ContainerReference, error)

	// Returns container's isolation spec.
	GetSpec() (info.ContainerSpec, error)

	GetStats() (*info.ContainerStats, error)

	ListContainers(listType ListType) ([]info.ContainerReference, error)

}