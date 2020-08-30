package container

import info "github.com/google/cadvisor/info/v1"

type ListType int

const (
	ListSelf 	ListType = iota
	ListRecursive
)

type ContainerHandler interface {

	GetStats() (*info.ContainerStats, error)

	ListContainers(listType ListType) ([]info.ContainerReference, error)
	
}