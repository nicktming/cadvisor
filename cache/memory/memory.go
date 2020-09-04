package memory

import (
	"errors"
	info "github.com/google/cadvisor/info/v1"
	"github.com/google/cadvisor/utils"
	"time"
	"sync"
	"github.com/google/cadvisor/storage"
	"k8s.io/klog"
)

var ErrDataNotFound = errors.New("unable to find data in memory cache")

type containerCache struct {
	ref 		info.ContainerReference

	recentStats 	*utils.TimedStore

	maxAge 		time.Duration

	lock 		sync.RWMutex
}

func (c *containerCache) AddStats(stats *info.ContainerStats) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.recentStats.Add(stats.Timestamp, stats)
	return nil
}

func (c *containerCache) RecentStats(start, end time.Time, maxStats int) ([]*info.ContainerStats, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	result := c.recentStats.InTimeRange(start, end, maxStats)
	converted := make([]*info.ContainerStats, len(result))
	for i, el := range result {
		converted[i] = el.(*info.ContainerStats)
	}
	return converted, nil
}

func newContainerStore(ref info.ContainerReference, maxAge time.Duration) *containerCache {
	return &containerCache{
		ref:         ref,
		recentStats: utils.NewTimedStore(maxAge, -1),
		maxAge:      maxAge,
	}
}

type InMemoryCache struct {
	lock 				sync.RWMutex
	containerCacheMap 		map[string]*containerCache
	maxAge 				time.Duration
	backend 			[]storage.StorageDriver
}

func (c *InMemoryCache) AddStats(cInfo *info.ContainerInfo, stats *info.ContainerStats) error {
	var cstore *containerCache
	var ok bool
	func() {
		c.lock.Lock()
		defer c.lock.Unlock()
		if cstore, ok = c.containerCacheMap[cInfo.ContainerReference.Name]; !ok {
			cstore = newContainerStore(cInfo.ContainerReference, c.maxAge)
			c.containerCacheMap[cInfo.ContainerReference.Name] = cstore
		}
	}()

	for _, backend := range c.backend {
		if err := backend.AddStats(cInfo, stats); err != nil {
			klog.Error(err)
		}
	}
	return cstore.AddStats(stats)
}

func (c *InMemoryCache) RecentStats(name string, start, end time.Time, maxStats int) ([]*info.ContainerStats, error) {
	var cstore *containerCache
	var ok bool
	err := func() error {
		c.lock.RLock()
		defer c.lock.RUnlock()
		if cstore, ok = c.containerCacheMap[name]; !ok {
			return ErrDataNotFound
		}
		return nil
	}()
	if err != nil {
		return nil, err
	}

	return cstore.RecentStats(start, end, maxStats)
}

func (c *InMemoryCache) Close() error {
	c.lock.Lock()
	c.containerCacheMap = make(map[string]*containerCache, 32)
	c.lock.Unlock()
	return nil
}

func (c *InMemoryCache) RemoveContainer(containerName string) error {
	c.lock.Lock()
	delete(c.containerCacheMap, containerName)
	c.lock.Unlock()
	return nil
}

func New(
maxAge time.Duration,
backend []storage.StorageDriver,
) *InMemoryCache {
	ret := &InMemoryCache{
		containerCacheMap: make(map[string]*containerCache, 32),
		maxAge:            maxAge,
		backend:           backend,
	}
	return ret
}




















































