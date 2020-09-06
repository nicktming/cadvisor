package sysinfo

import (
	"github.com/google/cadvisor/utils/sysfs"
	info "github.com/google/cadvisor/info/v1"
	"k8s.io/klog"
	"regexp"
	"fmt"
	"strconv"
	"os"
	"strings"
)

var (
	nodeDirRegExp        = regexp.MustCompile(`node/node(\d*)`)
	cpuDirRegExp         = regexp.MustCompile(`/cpu(\d+)`)
	memoryCapacityRegexp = regexp.MustCompile(`MemTotal:\s*([0-9]+) kB`)

	cpusPath = "/sys/devices/system/cpu"
)

const (
	cacheLevel2  = 2
	hugepagesDir = "hugepages/"
)

func GetNodesInfo(sysFs sysfs.SysFs) ([]info.Node, int, error) {
	nodes := []info.Node{}

	allLogicalCoresCount := 0

	nodesDirs, err := sysFs.GetNodesPaths()
	if err != nil {
		return nil, 0, err
	}

	if len(nodesDirs) == 0 {
		klog.Warningf("Nodes topology is not available, providing CPU topology")
		return getCPUTopology(sysFs)
	}

	for _, nodeDir := range nodesDirs {
		id, err := getMatchedInt(nodeDirRegExp, nodeDir)
		if err != nil {
			return nil, 0, err
		}
		node := info.Node{Id: id}

		cpuDirs, err := sysFs.GetCPUsPaths(nodeDir)
		klog.Infof("cpuDirs: %v", cpuDirs)

		if len(cpuDirs) == 0 {
			klog.Warningf("Found node without any CPU, nodeDir: %s, number of cpuDirs %d, err: %v", nodeDir, len(cpuDirs), err)
		} else {
			cores, err := getCoresInfo(sysFs, cpuDirs)
			if err != nil {
				return nil, 0, err
			}
			node.Cores = cores
			for _, core := range cores {
				allLogicalCoresCount += len(core.Threads)
			}
		}

		err = addCacheInfo(sysFs, &node)
		if err != nil {
			klog.V(1).Infof("Found node without cache information, nodeDir: %s", nodeDir)
		}

		node.Memory, err = getNodeMemInfo(sysFs, nodeDir)
		if err != nil {
			return nil, 0, err
		}

		// TODO huge page info

		//hugepagesDirectory := fmt.Sprintf("%s/%s", nodeDir, hugepagesDir)
		//node.HugePages, err = GetHugePagesInfo(sysFs, hugepagesDirectory)
		//if err != nil {
		//	return nil, 0, err
		//}

		nodes = append(nodes, node)
	}

	klog.Infof("nodesDir: %v", nodesDirs)

	return nodes, allLogicalCoresCount, nil
}

func getCPUTopology(sysFs sysfs.SysFs) ([]info.Node, int, error) {
	nodes := []info.Node{}

	cpusPaths, err := sysFs.GetCPUsPaths(cpusPath)
	if err != nil {
		return nil, 0, err
	}
	cpusCount := len(cpusPaths)

	if cpusCount == 0 {
		err = fmt.Errorf("Any CPU is not available, cpusPath: %s", cpusPath)
		return nil, 0, err
	}

	cpusByPhysicalPackageID, err := getCpusByPhysicalPackageID(sysFs, cpusPaths)
	if err != nil {
		return nil, 0, err
	}

	if len(cpusByPhysicalPackageID) == 0 {
		klog.Warningf("Cannot read any physical package id for any CPU")
		return nil, cpusCount, nil
	}

	for physicalPackageID, cpus := range cpusByPhysicalPackageID {
		node := info.Node{Id: physicalPackageID}

		cores, err := getCoresInfo(sysFs, cpus)
		if err != nil {
			return nil, 0, err
		}
		node.Cores = cores

		// On some Linux platforms(such as Arm64 guest kernel), cache info may not exist.
		// So, we should ignore error here.
		err = addCacheInfo(sysFs, &node)
		if err != nil {
			klog.V(1).Infof("Found cpu without cache information, cpuPath: %s", cpus)
		}
		nodes = append(nodes, node)
	}
	return nodes, cpusCount, nil
}

func getCpusByPhysicalPackageID(sysFs sysfs.SysFs, cpusPaths []string) (map[int][]string, error) {
	cpuPathsByPhysicalPackageID := make(map[int][]string)
	for _, cpuPath := range cpusPaths {

		rawPhysicalPackageID, err := sysFs.GetCPUPhysicalPackageID(cpuPath)
		if os.IsNotExist(err) {
			klog.Warningf("Cannot read physical package id for %s, physical_package_id file does not exist, err: %s", cpuPath, err)
			continue
		} else if err != nil {
			return nil, err
		}

		physicalPackageID, err := strconv.Atoi(rawPhysicalPackageID)
		if err != nil {
			return nil, err
		}

		if _, ok := cpuPathsByPhysicalPackageID[physicalPackageID]; !ok {
			cpuPathsByPhysicalPackageID[physicalPackageID] = make([]string, 0)
		}

		cpuPathsByPhysicalPackageID[physicalPackageID] = append(cpuPathsByPhysicalPackageID[physicalPackageID], cpuPath)
	}
	return cpuPathsByPhysicalPackageID, nil
}

// getNodeMemInfo returns information about total memory for NUMA node
func getNodeMemInfo(sysFs sysfs.SysFs, nodeDir string) (uint64, error) {
	rawMem, err := sysFs.GetMemInfo(nodeDir)
	if err != nil {
		//Ignore if per-node info is not available.
		klog.Warningf("Found node without memory information, nodeDir: %s", nodeDir)
		return 0, nil
	}
	matches := memoryCapacityRegexp.FindStringSubmatch(rawMem)
	if len(matches) != 2 {
		return 0, fmt.Errorf("failed to match regexp in output: %q", string(rawMem))
	}
	memory, err := strconv.ParseUint(matches[1], 10, 64)
	if err != nil {
		return 0, err
	}
	memory = memory * 1024 // Convert to bytes
	return uint64(memory), nil
}

func getCoresInfo(sysFs sysfs.SysFs, cpuDirs []string) ([]info.Core, error) {
	cores := make([]info.Core, 0, len(cpuDirs))
	for _, cpuDir := range cpuDirs {
		cpuID, err := getMatchedInt(cpuDirRegExp, cpuDir)
		if err != nil {
			return nil, fmt.Errorf("Unexpected format of CPU directory, cpuDirRegExp %s, cpuDir: %s", cpuDirRegExp, cpuDir)
		}
		if !sysFs.IsCPUOnline(cpuDir) {
			continue
		}

		rawPhysicalID, err := sysFs.GetCoreID(cpuDir)
		if os.IsNotExist(err) {
			klog.Warningf("Cannot read core id for %s, core_id file does not exist, err: %s", cpuDir, err)
			continue
		} else if err != nil {
			return nil, err
		}
		physicalID, err := strconv.Atoi(rawPhysicalID)
		if err != nil {
			return nil, err
		}
		coreIDx := -1
		for id, core := range cores {
			if core.Id == physicalID {
				coreIDx = id
				// break  got a pr
			}
		}
		if coreIDx == -1 {
			cores = append(cores, info.Core{})
			coreIDx = len(cores) - 1
		}

		desiredCore := &cores[coreIDx]
		desiredCore.Id = physicalID

		if len(desiredCore.Threads) == 0 {
			desiredCore.Threads = []int{cpuID}
		} else {
			desiredCore.Threads = append(desiredCore.Threads, cpuID)
		}

		rawPhysicalPackageID, err := sysFs.GetCPUPhysicalPackageID(cpuDir)
		if os.IsNotExist(err) {
			klog.Warningf("Cannot read physical package id for %s, physical_package_id file does not exist, err: %s", cpuDir, err)
			continue
		} else if err != nil {
			return nil, err
		}

		physicalPackageID, err := strconv.Atoi(rawPhysicalPackageID)
		if err != nil {
			return nil, err
		}
		desiredCore.SocketID = physicalPackageID

	}
	return cores, nil
}

func addCacheInfo(sysFs sysfs.SysFs, node *info.Node) error {
	for coreID, core := range node.Cores {
		threadID := core.Threads[0]
		caches, err := GetCacheInfo(sysFs, threadID)
		if err != nil {
			return err
		}

		numThreadsPerCore := len(core.Threads)
		numThreadsPerNode := len(node.Cores) * numThreadsPerCore

		for _, cache := range caches {
			c := info.Cache{
				Size: 		cache.Size,
				Level: 		cache.Level,
				Type: 		cache.Type,
			}
			// TODO don't understand
			if cache.Cpus == numThreadsPerNode && cache.Level > cacheLevel2 {
				// Add a node-level cache.
				cacheFound := false
				for _, nodeCache := range node.Caches {
					if nodeCache == c {
						cacheFound = true
					}
				}
				if !cacheFound {
					node.Caches = append(node.Caches, c)
				}
			} else if cache.Cpus == numThreadsPerCore {
				// Add core level cache
				node.Cores[coreID].Caches = append(node.Cores[coreID].Caches, c)
			}
		}
	}
	return nil
}

func getMatchedInt(reg *regexp.Regexp, str string) (int, error) {
	matches := reg.FindStringSubmatch(str)
	klog.Info("getMatchedInt matches: %v", matches)
	if len(matches) != 2 {
		return 0, fmt.Errorf("failed to match regexp, str: %s", str)
	}
	valInt, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, err
	}
	return valInt, nil
}

func GetCacheInfo(sysFs sysfs.SysFs, id int) ([]sysfs.CacheInfo, error) {
	caches, err := sysFs.GetCaches(id)
	if err != nil {
		return nil, err
	}

	info := []sysfs.CacheInfo{}
	for _, cache := range caches {
		if !strings.HasPrefix(cache.Name(), "index") {
			continue
		}
		cacheInfo, err := sysFs.GetCacheInfo(id, cache.Name())
		if err != nil {
			return nil, err
		}
		info = append(info, cacheInfo)
	}
	return info, nil
}