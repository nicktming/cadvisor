package common

import (
	"path"
	"github.com/google/cadvisor/container"
	info "github.com/google/cadvisor/info/v1"
	"os"
	"github.com/karrick/godirwalk"
	"github.com/pkg/errors"
	"time"
	"strconv"
	"github.com/google/cadvisor/utils"
	"io/ioutil"
	"strings"
	"k8s.io/klog"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"path/filepath"
)

func MakeCgroupPaths(mountPoints map[string]string, name string) map[string]string {
	cgroupPaths := make(map[string]string, len(mountPoints))
	for key, val := range mountPoints {
		cgroupPaths[key] = path.Join(val, name)
	}

	return cgroupPaths
}

// Lists all directories under "path" and outputs the results as children of "parent".
func ListDirectories(dirpath string, parent string, recursive bool, output map[string]struct{}) error {
	buf := make([]byte, godirwalk.DefaultScratchBufferSize)
	return listDirectories(dirpath, parent, recursive, output, buf)
}

func listDirectories(dirpath string, parent string, recursive bool, output map[string]struct{}, buf []byte) error {
	dirents, err := godirwalk.ReadDirents(dirpath, buf)
	if err != nil {
		// Ignore if this hierarchy does not exist.
		if os.IsNotExist(errors.Cause(err)) {
			err = nil
		}
		return err
	}
	for _, dirent := range dirents {
		// We only grab directories.
		if !dirent.IsDir() {
			continue
		}
		dirname := dirent.Name()

		name := path.Join(parent, dirname)
		output[name] = struct{}{}

		// List subcontainers if asked to.
		if recursive {
			err := listDirectories(path.Join(dirpath, dirname), name, true, output, buf)
			if err != nil {
				return err
			}
		}
	}
	return nil
}


func ListContainers(name string, cgroupPaths map[string]string, listType container.ListType) ([]info.ContainerReference, error) {
	containers := make(map[string]struct{})
	for _, cgroupPath := range cgroupPaths {
		err := ListDirectories(cgroupPath, name, listType == container.ListRecursive, containers)
		if err != nil {
			return nil, err
		}
	}

	// Make into container references.
	ret := make([]info.ContainerReference, 0, len(containers))
	for cont := range containers {
		ret = append(ret, info.ContainerReference{
			Name: cont,
		})
	}

	return ret, nil
}

// TODO machinInfo
func GetSpec(cgroupPaths map[string]string, hasNetwork, hasFilesystem bool) (info.ContainerSpec, error) {
	var spec info.ContainerSpec

	// Assume unified hierarchy containers.
	// Get the lowest creation time from all hierarchies as the container creation time.
	now := time.Now()
	lowestTime := now
	for _, cgroupPath := range cgroupPaths {
		// The modified time of the cgroup directory changes whenever a subcontainer is created.
		// eg. /docker will have creation time matching the creation of latest docker container.
		// Use clone_children as a workaround as it isn't usually modified. It is only likely changed
		// immediately after creating a container.

		cgroupPath = path.Join(cgroupPath, "cgroup.clone_children")
		fi, err := os.Stat(cgroupPath)
		if err == nil && fi.ModTime().Before(lowestTime) {
			lowestTime = fi.ModTime()
		}
	}
	if lowestTime != now {
		spec.CreationTime = lowestTime
	}

	// Get machine info.
	//mi, err := machineInfoFactory.GetMachineInfo()
	//if err != nil {
	//	return spec, err
	//}

	// CPU.
	cpuRoot, ok := cgroupPaths["cpu"]
	if ok {
		if utils.FileExists(cpuRoot) {
			spec.HasCpu = true
			spec.Cpu.Limit = readUInt64(cpuRoot, "cpu.shares")
			spec.Cpu.Period = readUInt64(cpuRoot, "cpu.cfs_period_us")
			quota := readString(cpuRoot, "cpu.cfs_quota_us")

			if quota != "" && quota != "-1" {
				val, err := strconv.ParseUint(quota, 10, 64)
				if err != nil {
					klog.Errorf("GetSpec: Failed to parse CPUQuota from %q: %s", path.Join(cpuRoot, "cpu.cfs_quota_us"), err)
				} else {
					spec.Cpu.Quota = val
				}
			}
		}
	}

	// Cpu Mask.
	// This will fail for non-unified hierarchies. We'll return the whole machine mask in that case.
	cpusetRoot, ok := cgroupPaths["cpuset"]
	if ok {
		if utils.FileExists(cpusetRoot) {
			spec.HasCpu = true
			//mask := ""
			//if cgroups.IsCgroup2UnifiedMode() {
			//	mask = readString(cpusetRoot, "cpuset.cpus.effective")
			//} else {
			//	mask = readString(cpusetRoot, "cpuset.cpus")
			//}
			//spec.Cpu.Mask = utils.FixCpuMask(mask, mi.NumCores)
		}
	}

	// Memory
	memoryRoot, ok := cgroupPaths["memory"]
	if ok {
		if !cgroups.IsCgroup2UnifiedMode() {
			if utils.FileExists(memoryRoot) {
				spec.HasMemory = true
				spec.Memory.Limit = readUInt64(memoryRoot, "memory.limit_in_bytes")
				spec.Memory.SwapLimit = readUInt64(memoryRoot, "memory.memsw.limit_in_bytes")
				spec.Memory.Reservation = readUInt64(memoryRoot, "memory.soft_limit_in_bytes")
			}
		} else {
			memoryRoot, err := findFileInAncestorDir(memoryRoot, "memory.max", "/sys/fs/cgroup")
			if err != nil {
				return spec, err
			}
			if memoryRoot != "" {
				spec.HasMemory = true
				spec.Memory.Reservation = readUInt64(memoryRoot, "memory.high")
				spec.Memory.Limit = readUInt64(memoryRoot, "memory.max")
				spec.Memory.SwapLimit = readUInt64(memoryRoot, "memory.swap.max")
			}
		}
	}

	// Hugepage
	hugepageRoot, ok := cgroupPaths["hugetlb"]
	if ok {
		if utils.FileExists(hugepageRoot) {
			spec.HasHugetlb = true
		}
	}

	// Processes, read it's value from pids path directly
	pidsRoot, ok := cgroupPaths["pids"]
	if ok {
		if utils.FileExists(pidsRoot) {
			spec.HasProcesses = true
			spec.Processes.Limit = readUInt64(pidsRoot, "pids.max")
		}
	}

	spec.HasNetwork = hasNetwork
	spec.HasFilesystem = hasFilesystem

	ioControllerName := "blkio"
	if cgroups.IsCgroup2UnifiedMode() {
		ioControllerName = "io"
	}
	if blkioRoot, ok := cgroupPaths[ioControllerName]; ok && utils.FileExists(blkioRoot) {
		spec.HasDiskIo = true
	}

	return spec, nil
}

// findFileInAncestorDir returns the path to the parent directory that contains the specified file.
// "" is returned if the lookup reaches the limit.
func findFileInAncestorDir(current, file, limit string) (string, error) {
	for {
		fpath := path.Join(current, file)
		_, err := os.Stat(fpath)
		if err == nil {
			return current, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		if current == limit {
			return "", nil
		}
		current = filepath.Dir(current)
	}
}

func readString(dirpath string, file string) string {
	cgroupFile := path.Join(dirpath, file)

	// Read
	out, err := ioutil.ReadFile(cgroupFile)
	if err != nil {
		// Ignore non-existent files
		if !os.IsNotExist(err) {
			klog.Warningf("readString: Failed to read %q: %s", cgroupFile, err)
		}
		return ""
	}
	return strings.TrimSpace(string(out))
}

func readUInt64(dirpath string, file string) uint64 {
	out := readString(dirpath, file)
	if out == "" || out == "max" {
		return 0
	}

	val, err := strconv.ParseUint(out, 10, 64)
	if err != nil {
		klog.Errorf("readUInt64: Failed to parse int %q from file %q: %s", out, path.Join(dirpath, file), err)
		return 0
	}

	return val
}
