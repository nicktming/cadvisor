package v2

import "time"

type FsInfo struct {
	// Time of generation of these stats.
	Timestamp time.Time `json:"timestamp"`

	// The block device name associated with the filesystem.
	Device string `json:"device"`

	// Path where the filesystem is mounted.
	Mountpoint string `json:"mountpoint"`

	// Filesystem usage in bytes.
	Capacity uint64 `json:"capacity"`

	// Bytes available for non-root use.
	Available uint64 `json:"available"`

	// Number of bytes used on this filesystem.
	Usage uint64 `json:"usage"`

	// Labels associated with this filesystem.
	Labels []string `json:"labels"`

	// Number of Inodes.
	Inodes *uint64 `json:"inodes,omitempty"`

	// Number of available Inodes (if known)
	InodesFree *uint64 `json:"inodes_free,omitempty"`
}

