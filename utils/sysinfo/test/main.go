package main

import (
	"github.com/google/cadvisor/utils/sysfs/fakesysfs"
	"github.com/google/cadvisor/utils/sysfs"
	"os"
	"github.com/google/cadvisor/utils/sysinfo"
	"k8s.io/klog"
	"encoding/json"
)

func main() {
	testGetNodesInfo()
}

func testGetNodesInfo() {
	fakeSys := &fakesysfs.FakeSysFs{}
	c := sysfs.CacheInfo{
		Size: 		32 * 1024,
		Type:		"unified",
		Level:		3,
		Cpus: 		2,
	}
	fakeSys.SetCacheInfo(c)

	nodesPaths := []string{
		"/fakeSysfs/devices/system/node/node0",
		"/fakeSysfs/devices/system/node/node1",
	}
	fakeSys.SetNodesPaths(nodesPaths, nil)

	cpusPaths := map[string][]string{
		"/fakeSysfs/devices/system/node/node0": {
			"/fakeSysfs/devices/system/node/node0/cpu0",
			"/fakeSysfs/devices/system/node/node0/cpu1",
		},
		"/fakeSysfs/devices/system/node/node1": {
			"/fakeSysfs/devices/system/node/node0/cpu2",
			"/fakeSysfs/devices/system/node/node0/cpu3",
		},
	}
	fakeSys.SetCPUsPaths(cpusPaths, nil)

	coreThread := map[string]string{
		"/fakeSysfs/devices/system/node/node0/cpu0": "0",
		"/fakeSysfs/devices/system/node/node0/cpu1": "0",
		"/fakeSysfs/devices/system/node/node0/cpu2": "1",
		"/fakeSysfs/devices/system/node/node0/cpu3": "1",
	}
	fakeSys.SetCoreThreads(coreThread, nil)

	memTotal := "MemTotal:       32817192 kB"
	fakeSys.SetMemory(memTotal, nil)

	hugePages := []os.FileInfo{
		&fakesysfs.FileInfo{EntryName: "hugepages-2048kB"},
	}
	fakeSys.SetHugePages(hugePages, nil)

	hugePageNr := map[string]string{
		"/fakeSysfs/devices/system/node/node0/hugepages/hugepages-2048kB/nr_hugepages": "1",
		"/fakeSysfs/devices/system/node/node1/hugepages/hugepages-2048kB/nr_hugepages": "1",
	}
	fakeSys.SetHugePagesNr(hugePageNr, nil)

	physicalPackageIDs := map[string]string{
		"/fakeSysfs/devices/system/node/node0/cpu0": "0",
		"/fakeSysfs/devices/system/node/node0/cpu1": "0",
		"/fakeSysfs/devices/system/node/node0/cpu2": "1",
		"/fakeSysfs/devices/system/node/node0/cpu3": "1",
	}
	fakeSys.SetPhysicalPackageIDs(physicalPackageIDs, nil)


	nodes, cores, err := sysinfo.GetNodesInfo(fakeSys)
	if err != nil {
		panic(err)
	}
	klog.Infof("nodes: %v, cores: %v", nodes, cores)


	nodesJSON, err := json.Marshal(nodes)
	expectedNodes := `
	[
      {
        "node_id": 0,
        "memory": 33604804608,
        "hugepages": [
          {
            "page_size": 2048,
            "num_pages": 1
          }
        ],
        "cores": [
          {
            "core_id": 0,
            "thread_ids": [
              0,
              1
            ],
            "caches": null,
	    "socket_id": 0
          }
        ],
        "caches": [
          {
            "size": 32768,
            "type": "unified",
            "level": 3
          }
        ]
      },
      {
        "node_id": 1,
        "memory": 33604804608,
        "hugepages": [
          {
            "page_size": 2048,
            "num_pages": 1
          }
        ],
        "cores": [
          {
            "core_id": 1,
            "thread_ids": [
              2,
              3
            ],
            "caches": null,
	    "socket_id": 1
          }
        ],
        "caches": [
          {
            "size": 32768,
            "type": "unified",
            "level": 3
          }
        ]
      }
    ]
    `

	klog.Infof("nodesJSON==expectedNodes? %v", (string(nodesJSON)==expectedNodes))
}