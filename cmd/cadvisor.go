package main

import (
	"github.com/google/cadvisor/manager"
	"github.com/google/cadvisor/utils/sysfs"
	"k8s.io/klog"
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
}

// TODO
// 1. watch raw removeWatchDirectory