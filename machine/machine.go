package machine

import (
	"github.com/google/cadvisor/utils/sysfs"
	"github.com/google/cadvisor/fs"

	info "github.com/google/cadvisor/info/v1"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"fmt"
	"strconv"
	"github.com/google/cadvisor/utils/sysinfo"
	"k8s.io/klog"
	"time"
)

var (
	coreRegExp = regexp.MustCompile(`(?m)^core id\s*:\s*([0-9]+)$`)
	nodeRegExp = regexp.MustCompile(`(?m)^physical id\s*:\s*([0-9]+)$`)
	// Power systems have a different format so cater for both
	cpuClockSpeedMHz     = regexp.MustCompile(`(?:cpu MHz|CPU MHz|clock)\s*:\s*([0-9]+\.[0-9]+)(?:MHz)?`)
	memoryCapacityRegexp = regexp.MustCompile(`MemTotal:\s*([0-9]+) kB`)
	swapCapacityRegexp   = regexp.MustCompile(`SwapTotal:\s*([0-9]+) kB`)

	cpuBusPath         = "/sys/bus/cpu/devices/"
	isMemoryController = regexp.MustCompile("mc[0-9]+")
	isDimm             = regexp.MustCompile("dimm[0-9]+")
	//machineArch        = getMachineArch()
	maxFreqFile        = "/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq"
)

const sysFsCPUCoreID = "core_id"
const sysFsCPUPhysicalPackageID = "physical_package_id"
const sysFsCPUTopology = "topology"
const memTypeFileName = "dimm_mem_type"
const sizeFileName = "size"

// GetPhysicalCores returns number of CPU cores reading /proc/cpuinfo file or if needed information from sysfs cpu path
func GetPhysicalCores(procInfo []byte) int {
	numCores := getUniqueMatchesCount(string(procInfo), coreRegExp)
	if numCores == 0 {
		// read number of cores from /sys/bus/cpu/devices/cpu*/topology/core_id to deal with processors
		// for which 'core id' is not available in /proc/cpuinfo
		//numCores = getUniqueCPUPropertyCount(cpuBusPath, sysFsCPUCoreID)
	}
	if numCores == 0 {
		klog.Errorf("Cannot read number of physical cores correctly, number of cores set to %d", numCores)
	}
	return numCores
}

// GetSockets returns number of CPU sockets reading /proc/cpuinfo file or if needed information from sysfs cpu path
func GetSockets(procInfo []byte) int {
	numSocket := getUniqueMatchesCount(string(procInfo), nodeRegExp)
	if numSocket == 0 {
		// read number of sockets from /sys/bus/cpu/devices/cpu*/topology/physical_package_id to deal with processors
		// for which 'physical id' is not available in /proc/cpuinfo
		//numSocket = getUniqueCPUPropertyCount(cpuBusPath, sysFsCPUPhysicalPackageID)
	}
	if numSocket == 0 {
		klog.Errorf("Cannot read number of sockets correctly, number of sockets set to %d", numSocket)
	}
	return numSocket
}

// getUniqueMatchesCount returns number of unique matches in given argument using provided regular expression
func getUniqueMatchesCount(s string, r *regexp.Regexp) int {
	matches := r.FindAllString(s, -1)
	uniques := make(map[string]bool)
	for _, match := range matches {
		uniques[match] = true
	}
	return len(uniques)
}


func Info(sysFs sysfs.SysFs, fsInfo fs.FsInfo, inHostNamespace bool) (*info.MachineInfo, error) {
	rootFs := "/"
	if !inHostNamespace {
		rootFs = "/rootfs"
	}

	cpuinfo, err := ioutil.ReadFile(filepath.Join(rootFs, "/proc/cpuinfo"))
	if err != nil {
		return nil, err
	}
	// TODO clock speed

	memoryCapacity, err := GetMachineMemoryCapacity()
	if err != nil {
		return nil, err
	}

	topology, numCores, err := GetTopology(sysFs)
	if err != nil {
		klog.Errorf("Failed to get topology information: %v", err)
	}

	machineInfo := &info.MachineInfo{
		Timestamp: 		time.Now(),
		NumCores: 		numCores,
		NumPhysicalCores:	GetPhysicalCores(cpuinfo),
		NumSockets: 		GetSockets(cpuinfo),
		MemoryCapacity: 	memoryCapacity,
		Topology: 		topology,
	}
	return machineInfo, nil
}

// GetTopology returns CPU topology reading information from sysfs
func GetTopology(sysFs sysfs.SysFs) ([]info.Node, int, error) {
	// s390/s390x changes
	//if isSystemZ() {
	//	return nil, getNumCores(), nil
	//}
	return sysinfo.GetNodesInfo(sysFs)
}

// GetMachineMemoryCapacity returns the machine's total memory from /proc/meminfo.
// Returns the total memory capacity as an uint64 (number of bytes).
func GetMachineMemoryCapacity() (uint64, error) {
	out, err := ioutil.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, err
	}

	memoryCapacity, err := parseCapacity(out, memoryCapacityRegexp)
	if err != nil {
		return 0, err
	}
	return memoryCapacity, err
}

// parseCapacity matches a Regexp in a []byte, returning the resulting value in bytes.
// Assumes that the value matched by the Regexp is in KB.
func parseCapacity(b []byte, r *regexp.Regexp) (uint64, error) {
	matches := r.FindSubmatch(b)
	if len(matches) != 2 {
		return 0, fmt.Errorf("failed to match regexp in output: %q", string(b))
	}
	m, err := strconv.ParseUint(string(matches[1]), 10, 64)
	if err != nil {
		return 0, err
	}

	// Convert to bytes.
	return m * 1024, err
}
