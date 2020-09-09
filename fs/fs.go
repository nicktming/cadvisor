package fs

import (
	"os"
	"fmt"
	"syscall"
	"path/filepath"
	"k8s.io/utils/mount"
	"k8s.io/klog"
	"strings"
	"io/ioutil"
	"path"
)

const (
	LabelSystemRoot          = "root"
	LabelDockerImages        = "docker-images"
	LabelCrioImages          = "crio-images"
	DriverStatusPoolName     = "Pool Name"
	DriverStatusDataLoopFile = "Data loop file"
)

const (
	// The block size in bytes.
	statBlockSize uint64 = 512
	// The maximum number of `disk usage` tasks that can be running at once.
	maxConcurrentOps = 20
)

// A pool for restricting the number of consecutive `du` and `find` tasks running.
var pool = make(chan struct{}, maxConcurrentOps)

func init() {
	for i := 0; i < maxConcurrentOps; i++ {
		releaseToken()
	}
}

func claimToken() {
	<-pool
}

func releaseToken() {
	pool <- struct{}{}
}

type partition struct {
	mountpoint string
	major      uint
	minor      uint
	fsType     string
	blockSize  uint
}

type RealFsInfo struct {
	// Map from block device path to partition information.
	partitions map[string]partition
	// Map from label to block device path.
	// Labels are intent-specific tags that are auto-detected.
	labels map[string]string
	// Map from mountpoint to mount information.
	mounts map[string]mount.MountInfo
	// devicemapper client
	// TODO devicemapper
	//dmsetup devicemapper.DmsetupClient
	// fsUUIDToDeviceName is a map from the filesystem UUID to its device name.
	fsUUIDToDeviceName map[string]string
}

func NewFsInfo(context Context) (FsInfo, error) {
	mounts, err := mount.ParseMountInfo("/proc/self/mountinfo")
	if err != nil {
		return nil, err
	}

	fsUUIDToDeviceName, err := getFsUUIDToDeviceNameMap()
	if err != nil {
		// UUID is not always available across different OS distributions.
		// Do not fail if there is an error.
		klog.Warningf("Failed to get disk UUID mapping, getting disk info by uuid will not work: %v", err)
	}

	// Avoid devicemapper container mounts - these are tracked by the ThinPoolWatcher
	excluded := []string{fmt.Sprintf("%s/devicemapper/mnt", context.Docker.Root)}
	fsInfo := &RealFsInfo{
		partitions:         processMounts(mounts, excluded),
		labels:             make(map[string]string),
		mounts:             make(map[string]mount.MountInfo),
		//dmsetup:            devicemapper.NewDmsetupClient(),
		fsUUIDToDeviceName: fsUUIDToDeviceName,
	}

	for _, mount := range mounts {
		fsInfo.mounts[mount.MountPoint] = mount
	}

	// need to call this before the log line below printing out the partitions, as this function may
	// add a "partition" for devicemapper to fsInfo.partitions
	fsInfo.addDockerImagesLabel(context, mounts)
	//TODO crio
	//fsInfo.addCrioImagesLabel(context, mounts)

	klog.Infof("Filesystem UUIDs: %+v", fsInfo.fsUUIDToDeviceName)
	klog.Infof("Filesystem partitions: %+v", fsInfo.partitions)
	// TODO system
	//fsInfo.addSystemRootLabel(mounts)
	return fsInfo, nil
}

func (i *RealFsInfo) addDockerImagesLabel(context Context, mounts []mount.MountInfo) {
	dockerDev, dockerPartition, err := i.getDockerDeviceMapperInfo(context.Docker)
	if err != nil {
		klog.Warningf("Could not get Docker devicemapper device: %v", err)
	}
	if len(dockerDev) > 0 && dockerPartition != nil {
		i.partitions[dockerDev] = *dockerPartition
		i.labels[LabelDockerImages] = dockerDev
	} else {
		i.updateContainerImagesPath(LabelDockerImages, mounts, getDockerImagePaths(context))
	}
}

// Generate a list of possible mount points for docker image management from the docker root directory.
// Right now, we look for each type of supported graph driver directories, but we can do better by parsing
// some of the context from `docker info`.
func getDockerImagePaths(context Context) map[string]struct{} {
	dockerImagePaths := map[string]struct{}{
		"/": {},
	}

	// TODO(rjnagal): Detect docker root and graphdriver directories from docker info.
	dockerRoot := context.Docker.Root
	for _, dir := range []string{"devicemapper", "btrfs", "aufs", "overlay", "overlay2", "zfs"} {
		dockerImagePaths[path.Join(dockerRoot, dir)] = struct{}{}
	}
	for dockerRoot != "/" && dockerRoot != "." {
		dockerImagePaths[dockerRoot] = struct{}{}
		dockerRoot = filepath.Dir(dockerRoot)
	}
	return dockerImagePaths
}

// This method compares the mountpoints with possible container image mount points. If a match is found,
// the label is added to the partition.
func (i *RealFsInfo) updateContainerImagesPath(label string, mounts []mount.MountInfo, containerImagePaths map[string]struct{}) {
	var useMount *mount.MountInfo
	for _, m := range mounts {
		if _, ok := containerImagePaths[m.MountPoint]; ok {
			if useMount == nil || (len(useMount.MountPoint) < len(m.MountPoint)) {
				useMount = new(mount.MountInfo)
				*useMount = m
			}
		}
	}
	if useMount != nil {
		i.partitions[useMount.Source] = partition{
			fsType:     useMount.FsType,
			mountpoint: useMount.MountPoint,
			major:      uint(useMount.Major),
			minor:      uint(useMount.Minor),
		}
		i.labels[label] = useMount.Source
	}
}


func (i *RealFsInfo) getDockerDeviceMapperInfo(context DockerContext) (string, *partition, error) {
	klog.Infof("====>context driver: %s", context.Driver)
	if context.Driver != DeviceMapper.String() {

		return "", nil, nil
	}

	//dataLoopFile := context.DriverStatus[DriverStatusDataLoopFile]
	//if len(dataLoopFile) > 0 {
	//	return "", nil, nil
	//}
	//
	//dev, major, minor, blockSize, err := dockerDMDevice(context.DriverStatus, i.dmsetup)
	//if err != nil {
	//	return "", nil, err
	//}
	//
	//return dev, &partition{
	//	fsType:    DeviceMapper.String(),
	//	major:     major,
	//	minor:     minor,
	//	blockSize: blockSize,
	//}, nil
	return "", nil, nil
}

func (i *RealFsInfo) GetDirFsDevice(dir string) (*DeviceInfo, error) {
	buf := new(syscall.Stat_t)
	err := syscall.Stat(dir, buf)
	if err != nil {
		return nil, fmt.Errorf("stat failed on %s with error: %s", dir, err)
	}

	// The type Dev in Stat_t is 32bit on mips.
	major := major(uint64(buf.Dev)) // nolint: unconvert
	minor := minor(uint64(buf.Dev)) // nolint: unconvert
	klog.Infof("++++++GetDirFsDevice major: %v, minor: %v", major, minor)
	for device, partition := range i.partitions {
		klog.Infof("++++++GetDirFsDevice partition mountpoint: %v, major: %v, minor: %v", partition.mountpoint, partition.major, partition.minor)
		if partition.major == major && partition.minor == minor {
			return &DeviceInfo{device, major, minor}, nil
		}
	}

	mount, found := i.mounts[dir]
	// try the parent dir if not found until we reach the root dir
	// this is an issue on btrfs systems where the directory is not
	// the subvolume
	for !found {
		pathdir, _ := filepath.Split(dir)
		// break when we reach root
		if pathdir == "/" {
			break
		}
		// trim "/" from the new parent path otherwise the next possible
		// filepath.Split in the loop will not split the string any further
		dir = strings.TrimSuffix(pathdir, "/")
		mount, found = i.mounts[dir]
	}

	if found && mount.FsType == "btrfs" && mount.Major == 0 && strings.HasPrefix(mount.Source, "/dev/") {
		major, minor, err := getBtrfsMajorMinorIds(&mount)
		if err != nil {
			klog.Warningf("%s", err)
		} else {
			return &DeviceInfo{mount.Source, uint(major), uint(minor)}, nil
		}
	}
	return nil, fmt.Errorf("could not find device with major: %d, minor: %d in cached partitions map", major, minor)
}

// getFsUUIDToDeviceNameMap creates the filesystem uuid to device name map
// using the information in /dev/disk/by-uuid. If the directory does not exist,
// this function will return an empty map.
func getFsUUIDToDeviceNameMap() (map[string]string, error) {
	const dir = "/dev/disk/by-uuid"

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return make(map[string]string), nil
	}

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	fsUUIDToDeviceName := make(map[string]string)
	for _, file := range files {
		path := filepath.Join(dir, file.Name())
		target, err := os.Readlink(path)
		if err != nil {
			klog.Warningf("Failed to resolve symlink for %q", path)
			continue
		}
		device, err := filepath.Abs(filepath.Join(dir, target))
		if err != nil {
			return nil, fmt.Errorf("failed to resolve the absolute path of %q", filepath.Join(dir, target))
		}
		fsUUIDToDeviceName[file.Name()] = device
	}
	return fsUUIDToDeviceName, nil
}

func processMounts(mounts []mount.MountInfo, excludedMountpointPrefixes []string) map[string]partition {
	partitions := make(map[string]partition)

	supportedFsType := map[string]bool{
		// all ext systems are checked through prefix.
		"btrfs":   true,
		"overlay": true,
		"tmpfs":   true,
		"xfs":     true,
		"zfs":     true,
	}

	for _, mount := range mounts {
		if !strings.HasPrefix(mount.FsType, "ext") && !supportedFsType[mount.FsType] {
			continue
		}
		// Avoid bind mounts, exclude tmpfs.
		if _, ok := partitions[mount.Source]; ok {
			if mount.FsType != "tmpfs" {
				continue
			}
		}

		hasPrefix := false
		for _, prefix := range excludedMountpointPrefixes {
			if strings.HasPrefix(mount.MountPoint, prefix) {
				hasPrefix = true
				break
			}
		}
		if hasPrefix {
			continue
		}

		// using mountpoint to replace device once fstype it tmpfs
		if mount.FsType == "tmpfs" {
			mount.Source = mount.MountPoint
		}
		// btrfs fix: following workaround fixes wrong btrfs Major and Minor Ids reported in /proc/self/mountinfo.
		// instead of using values from /proc/self/mountinfo we use stat to get Ids from btrfs mount point
		if mount.FsType == "btrfs" && mount.Major == 0 && strings.HasPrefix(mount.Source, "/dev/") {
			major, minor, err := getBtrfsMajorMinorIds(&mount)
			if err != nil {
				klog.Warningf("%s", err)
			} else {
				mount.Major = major
				mount.Minor = minor
			}
		}

		// overlay fix: Making mount source unique for all overlay mounts, using the mount's major and minor ids.
		if mount.FsType == "overlay" {
			mount.Source = fmt.Sprintf("%s_%d-%d", mount.Source, mount.Major, mount.Minor)
		}

		partitions[mount.Source] = partition{
			fsType:     mount.FsType,
			mountpoint: mount.MountPoint,
			major:      uint(mount.Major),
			minor:      uint(mount.Minor),
		}
	}

	return partitions
}


func major(devNumber uint64) uint {
	return uint((devNumber >> 8) & 0xfff)
}

func minor(devNumber uint64) uint {
	return uint((devNumber & 0xff) | ((devNumber >> 12) & 0xfff00))
}

// Get major and minor Ids for a mount point using btrfs as filesystem.
func getBtrfsMajorMinorIds(mount *mount.MountInfo) (int, int, error) {
	// btrfs fix: following workaround fixes wrong btrfs Major and Minor Ids reported in /proc/self/mountinfo.
	// instead of using values from /proc/self/mountinfo we use stat to get Ids from btrfs mount point

	buf := new(syscall.Stat_t)
	err := syscall.Stat(mount.Source, buf)
	if err != nil {
		err = fmt.Errorf("stat failed on %s with error: %s", mount.Source, err)
		return 0, 0, err
	}

	klog.V(4).Infof("btrfs mount %#v", mount)
	if buf.Mode&syscall.S_IFMT == syscall.S_IFBLK {
		err := syscall.Stat(mount.MountPoint, buf)
		if err != nil {
			err = fmt.Errorf("stat failed on %s with error: %s", mount.MountPoint, err)
			return 0, 0, err
		}

		// The type Dev and Rdev in Stat_t are 32bit on mips.
		klog.V(4).Infof("btrfs dev major:minor %d:%d\n", int(major(uint64(buf.Dev))), int(minor(uint64(buf.Dev))))    // nolint: unconvert
		klog.V(4).Infof("btrfs rdev major:minor %d:%d\n", int(major(uint64(buf.Rdev))), int(minor(uint64(buf.Rdev)))) // nolint: unconvert

		return int(major(uint64(buf.Dev))), int(minor(uint64(buf.Dev))), nil // nolint: unconvert
	}
	return 0, 0, fmt.Errorf("%s is not a block device", mount.Source)
}


func GetDirUsage(dir string) (UsageInfo, error) {
	var usage UsageInfo

	if dir == "" {
		return usage, fmt.Errorf("invalid directory")
	}

	rootInfo, err := os.Stat(dir)
	if err != nil {
		return usage, fmt.Errorf("could not stat %q to get inode usage: %v", dir, err)
	}

	rootStat, ok := rootInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return usage, fmt.Errorf("unsuported fileinfo for getting inode usage of %q", dir)
	}

	rootDevID := rootStat.Dev

	// dedupedInode stores inodes that could be duplicates (nlink > 1)
	dedupedInodes := make(map[uint64]struct{})

	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if os.IsNotExist(err) {
			// expected if files appear/vanish
			return nil
		}
		if err != nil {
			return fmt.Errorf("unable to count inodes for part of dir %s: %s", dir, err)
		}

		// according to the docs, Sys can be nil
		if info.Sys() == nil {
			return fmt.Errorf("fileinfo Sys is nil")
		}

		s, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return fmt.Errorf("unsupported fileinfo; could not convert to stat_t")
		}

		if s.Dev != rootDevID {
			// don't descend into directories on other devices
			return filepath.SkipDir
		}
		if s.Nlink > 1 {
			if _, ok := dedupedInodes[s.Ino]; !ok {
				// Dedupe things that could be hardlinks
				dedupedInodes[s.Ino] = struct{}{}

				usage.Bytes += uint64(s.Blocks) * statBlockSize
				usage.Inodes++
			}
		} else {
			usage.Bytes += uint64(s.Blocks) * statBlockSize
			usage.Inodes++
		}
		return nil
	})

	return usage, err
}

func (i *RealFsInfo) GetDirUsage(dir string) (UsageInfo, error) {
	claimToken()
	defer releaseToken()
	return GetDirUsage(dir)
}

func (i *RealFsInfo) GetMountpointForDevice(dev string) (string, error) {
	p, ok := i.partitions[dev]
	if !ok {
		return "", fmt.Errorf("no partition info for device %q", dev)
	}
	return p.mountpoint, nil
}

func (i *RealFsInfo) GetDeviceForLabel(label string) (string, error) {
	dev, ok := i.labels[label]
	if !ok {
		return "", fmt.Errorf("non-existent label %q", label)
	}
	return dev, nil
}


func (i *RealFsInfo) GetLabelsForDevice(device string) ([]string, error) {
	labels := []string{}
	for label, dev := range i.labels {
		if dev == device {
			labels = append(labels, label)
		}
	}
	return labels, nil
}

















































