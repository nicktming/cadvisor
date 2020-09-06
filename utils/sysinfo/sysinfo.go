package sysinfo

import (
	"github.com/google/cadvisor/utils/sysfs"
	info "github.com/google/cadvisor/info/v1"
	"k8s.io/klog"
)

func GetNodesInfo(sysFs sysfs.SysFs) ([]info.Node, int, error) {
	nodes := []info.Node{}

	//allLogicalCoresCount := 0

	nodesDir, err := sysFs.GetNodesPaths()
	if err != nil {
		return nil, 0, err
	}

	klog.Infof("nodesDir: %v", nodesDir)

	return nodes, 0, nil
}
