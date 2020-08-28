package libcontainer

import (
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/google/cadvisor/container"
	"fmt"
)

type CgroupSubsystems struct {

	Mounts 	[]cgroups.Mount

	MountPoints 	map[string]string
}


func GetCgroupSubsystems(includedMetrics container.MetricSet) (CgroupSubsystems, error) {
	allCgroups, err := cgroups.GetCgroupMount(true)
	if err != nil {
		return CgroupSubsystems{}, err
	}

	disableCgroups := map[string]struct{}{}

	if !includedMetrics.Has(container.DiskIOMetrics) {
		disableCgroups["blkio"] = struct {}{}
		disableCgroups["io"] = struct {}{}
	}
	return getCgroupSubsystemsHelper(allCgroups, disableCgroups)
}

func GetAllCgroupSubsystems() (CgroupSubsystems, error) {
	// Get all cgroup mounts.
	allCgroups, err := cgroups.GetCgroupMounts(true)
	if err != nil {
		return CgroupSubsystems{}, err
	}

	emptyDisableCgroups := map[string]struct{}{}
	return getCgroupSubsystemsHelper(allCgroups, emptyDisableCgroups)
}


func getCgroupSubsystemsHelper(allCgroups []cgroups.Mount, disableCgroups map[string]struct{}) (CgroupSubsystems, error) {
	if len(allCgroups) == 0 {
		return CgroupSubsystems{}, fmt.Errorf("failed to find cgroup mounts")
	}

	// Trim the mounts to only the subsystems we care about.
	supportedCgroups := make([]cgroups.Mount, 0, len(allCgroups))
	recordedMountpoints := make(map[string]struct{}, len(allCgroups))
	mountPoints := make(map[string]string, len(allCgroups))
	for _, mount := range allCgroups {
		for _, subsystem := range mount.Subsystems {
			if _, exists := disableCgroups[subsystem]; exists {
				continue
			}
			if _, ok := supportedSubsystems[subsystem]; !ok {
				// Unsupported subsystem
				continue
			}
			if _, ok := mountPoints[subsystem]; ok {
				// duplicate mount for this subsystem; use the first one we saw
				klog.V(5).Infof("skipping %s, already using mount at %s", mount.Mountpoint, mountPoints[subsystem])
				continue
			}
			if _, ok := recordedMountpoints[mount.Mountpoint]; !ok {
				// avoid appending the same mount twice in e.g. `cpu,cpuacct` case
				supportedCgroups = append(supportedCgroups, mount)
				recordedMountpoints[mount.Mountpoint] = struct{}{}
			}
			mountPoints[subsystem] = mount.Mountpoint
		}
	}

	return CgroupSubsystems{
		Mounts:      supportedCgroups,
		MountPoints: mountPoints,
	}, nil
}

// Cgroup subsystems we support listing (should be the minimal set we need stats from).
var supportedSubsystems map[string]struct{} = map[string]struct{}{
	"cpu":        {},
	"cpuacct":    {},
	"memory":     {},
	"hugetlb":    {},
	"pids":       {},
	"cpuset":     {},
	"blkio":      {},
	"io":         {},
	"devices":    {},
	"perf_event": {},
}

