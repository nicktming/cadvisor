package main

import (
	"github.com/google/cadvisor/manager"
	"github.com/google/cadvisor/utils/sysfs"
	"k8s.io/klog"
	"github.com/google/cadvisor/events"
	info "github.com/google/cadvisor/info/v1"
	"encoding/json"
	"time"
)

func main() {

	sysFs := sysfs.NewRealSysFs()

	memoryStorage, err := NewMemoryStorage()
	if err != nil {
		panic(err)
	}

	resourceManager, err := manager.New(memoryStorage, sysFs)

	if err != nil {
		klog.Infof("Failed to create a manager: %s", err)
	}
	req := events.NewRequest()
	req.EventType[info.EventContainerCreation] = true

	ec, err := resourceManager.WatchForEvents(req)

	if err := resourceManager.Start(); err != nil {
		klog.Fatal("Failed to start manager: %v", err)
	}

	stop := make(chan struct{})
	klog.Infof("====>got watcher Id: %v", ec.GetWatchId())
	go func() {
		for  {
			select {
			case event := <- ec.GetChannel():
				klog.Infof("+++++++++++++++event containerName: %v, type: %v", event.ContainerName, event.EventType)
			}
		}
	}()

	time.Sleep(5 * time.Second)

	request := info.DefaultContainerInfoRequest()

	cinfo, err := resourceManager.GetContainerInfo("/", &request)
	if err != nil {
		panic(err)
	}

	klog.Infof("cinfo: %v/%v", cinfo.Namespace, cinfo.Name)

	pretty_cinfo, _ := json.MarshalIndent(cinfo, "", "\t")

	klog.Infof("pretty cinfo: %v", string(pretty_cinfo))

	<- stop

}

// TODO
// 1. watch raw removeWatchDirectory
// 2. MakeCgroupPaths no need to use /
