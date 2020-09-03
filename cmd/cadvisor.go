package main

import (
	"github.com/google/cadvisor/manager"
	"github.com/google/cadvisor/utils/sysfs"
	"k8s.io/klog"
	"github.com/google/cadvisor/events"
	info "github.com/google/cadvisor/info/v1"
)

func main() {

	sysFs := sysfs.NewRealSysFs()

	resourceManager, err := manager.New(nil, sysFs)

	if err != nil {
		klog.Infof("Failed to create a manager: %s", err)
	}

	if err := resourceManager.Start(); err != nil {
		klog.Fatal("Failed to start manager: %v", err)
	}

	req := events.NewRequest()
	req.EventType[info.EventContainerCreation] = true

	ec, err := resourceManager.WatchForEvents(req)
	for  {
		select {
		case event := <- ec.GetChannel():
			klog.Infof("event containerName: %v, type: %v", event.ContainerName, event.EventType)
		}
	}

}

// TODO
// 1. watch raw removeWatchDirectory
// 2. MakeCgroupPaths no need to use /
