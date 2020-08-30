package raw

import (
	"github.com/google/cadvisor/container/libcontainer"
	"github.com/google/cadvisor/container/common"
	"github.com/google/cadvisor/watcher"
	"fmt"
	"k8s.io/klog"
	"strings"
	"io/ioutil"
	"path"
	"os"
	inotify "k8s.io/utils/inotify"
)

type rawContainerWatcher struct {

	cgroupPaths 		map[string]string

	cgroupSubsystems 	*libcontainer.CgroupSubsystems

	watcher 		*common.InotifyWatcher

	stopWatcher 		chan error

}

func NewRawContainerWatcher() (watcher.ContainerWatcher, error) {
	cgroupSubsystems, err := libcontainer.GetAllCgroupSubsystems()

	klog.Infof("======>NewRawContainerWatcher got cgroupSubsystems")
	cgroupSubsystems.String()

	if err != nil {
		return nil, fmt.Errorf("failed to get cgroup subsystems: %v", err)
	}
	if len(cgroupSubsystems.Mounts) == 0 {
		return nil, fmt.Errorf("failed to find supported cgroup mounts for the raw factory")
	}

	watcher, err := common.NewInotifyWatcher()
	if err != nil {
		return nil, err
	}

	rawWatcher := &rawContainerWatcher{
		cgroupPaths:      common.MakeCgroupPaths(cgroupSubsystems.MountPoints, "/"),
		cgroupSubsystems: &cgroupSubsystems,
		watcher:          watcher,
		stopWatcher:      make(chan error),
	}

	klog.Infof("=============NewRawContainerWatcher print rawWatcher")
	for k, v := range rawWatcher.cgroupPaths {
		klog.Infof("rawWatcher.cgroupPaths %v=%v", k, v)
	}

	return rawWatcher, nil
}

func (w *rawContainerWatcher) Start(events chan watcher.ContainerEvent) error {
	// TODO .mount

	watched := make([]string, 0)

	for _, cgroupPath := range w.cgroupPaths {
		_, err := w.watchDirectory(events, cgroupPath, "/")
		if err != nil {
			for _, watchedCgroupPath := range watched {
				_, err := w.watcher.RemoveWatch("/", watchedCgroupPath)
				if err != nil {
					klog.Warningf("Failed to remove inotify watch for %q: %v", watchedCgroupPath, err)
				}
			}
			return err
		}
		watched = append(watched, cgroupPath)
	}

	// TODO removeDirectory watch
	go func(){
		for {
			select {
			case event := <-w.watcher.Event():
				err := w.processEvent(event, events)
				if err != nil {
					klog.Warningf("")
				}

			case err := <-w.watcher.Error():
				klog.Warningf("Error while watching %q: %v", "/", err)

			case <-w.stopWatcher:
				err := w.watcher.Close()
				if err == nil {
					w.stopWatcher <- err
					return
				}
			}
		}
	}()
	return nil
}

func (w *rawContainerWatcher) Stop() error {
	w.stopWatcher <- nil
	// TODO using another channel
	return <-w.stopWatcher
}


func (w *rawContainerWatcher) watchDirectory(events chan watcher.ContainerEvent, dir string, containerName string) (bool, error) {
	// TODO ignore .mount
	alreadyWatching, err := w.watcher.AddWatch(containerName, dir)
	if err != nil {
		return alreadyWatching, err
	}

	cleanup := true
	defer func() {
		if cleanup {
			_, err := w.watcher.RemoveWatch(containerName, dir)
			if err != nil {
				klog.Warningf("Failed to remove inotify watch for %q: %v", dir, err)
			}
		}
	}()

	entries, err := ioutil.ReadDir(dir)
	if err != nil {
		return alreadyWatching, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			entryPath := path.Join(dir, entry.Name())
			subcontainerName := path.Join(containerName, entry.Name())

			//klog.Infof("watch directory: entryPath: %v, subcontainerName: %v", entryPath, subcontainerName)
			alreadyWatchingSubDir, err := w.watchDirectory(events, entryPath, subcontainerName)
			if err != nil {
				klog.Infof("Failed to watch directory %q: %v", entryPath, err)
				if os.IsNotExist(err) {
					continue
				}
				return alreadyWatching, err
			}
			if !alreadyWatchingSubDir {
				go func() {
					events <- watcher.ContainerEvent{
						EventType: 	watcher.ContainerAdd,
						Name: 		subcontainerName,
						WatchSource: 	watcher.Raw,
					}
				}()
			}
		}
	}

	cleanup = false
	return alreadyWatching, nil
}

func (w *rawContainerWatcher) processEvent(event *inotify.Event, events chan watcher.ContainerEvent) error {
	var eventType watcher.ContainerEventType
	switch {
	case (event.Mask & inotify.InCreate) > 0:
		eventType = watcher.ContainerAdd
	case (event.Mask & inotify.InDelete) > 0:
		eventType = watcher.ContainerDelete
	case (event.Mask & inotify.InMovedFrom) > 0:
		eventType = watcher.ContainerDelete
	case (event.Mask & inotify.InMovedTo) > 0:
		eventType = watcher.ContainerAdd
	default:
		// Ignore other events.
		return nil
	}

	var containerName string
	for _, mount := range w.cgroupSubsystems.Mounts {
		mountLocation := path.Clean(mount.Mountpoint) + "/"
		klog.Infof("=====>processEvent ")
		if strings.HasPrefix(event.Name, mountLocation) {
			containerName = event.Name[len(mountLocation)-1:]
			break
		}
	}
	if containerName == "" {
		return fmt.Errorf("unable to detect container from watch event on directory %q", event.Name)
	}
	// Maintain the watch for the new or deleted container.
	switch eventType {
	case watcher.ContainerAdd:
		// New container was created, watch it.
		alreadyWatched, err := w.watchDirectory(events, event.Name, containerName)
		if err != nil {
			return err
		}

		// Only report container creation once.
		if alreadyWatched {
			return nil
		}
	case watcher.ContainerDelete:
		// Container was deleted, stop watching for it.
		lastWatched, err := w.watcher.RemoveWatch(containerName, event.Name)
		if err != nil {
			return err
		}

		// Only report container deletion once.
		if !lastWatched {
			return nil
		}
	default:
		return fmt.Errorf("unknown event type %v", eventType)
	}

	// Deliver the event.
	events <- watcher.ContainerEvent{
		EventType:   eventType,
		Name:        containerName,
		WatchSource: watcher.Raw,
	}

	return nil
}

























































