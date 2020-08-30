package common

import (
	"path"
	"github.com/google/cadvisor/container"
	info "github.com/google/cadvisor/info/v1"
	"os"
	"github.com/karrick/godirwalk"
	"github.com/pkg/errors"
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
