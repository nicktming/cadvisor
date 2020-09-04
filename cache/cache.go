package cache

import info "github.com/google/cadvisor/info/v1"

type Cache interface {
	AddStats(ref info.ContainerReference, stats *info.ContainerStats)

	RemoveContainer(containerName string) error

	RecentStats(containerName string, numStats int) ([]*info.ContainerStats, error)

	Close() error
}