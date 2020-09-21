package common

import (
	"sync"
	"time"
	"github.com/google/cadvisor/fs"
	"k8s.io/klog"
	"fmt"
)

type FsHandler interface {
	Start()
	Usage() FsUsage
	Stop()
}

type FsUsage struct {
	BaseUsageBytes 		uint64
	TotalUsageBytes 	uint64
	InodeUsage 		uint64
}

type realFsHandler struct {
	sync.RWMutex
	lastUpdate 		time.Time
	usage 			FsUsage
	period 			time.Duration
	minPeriod 		time.Duration
	rootfs 			string
	extraDir 		string
	fsInfo     		fs.FsInfo

	stopChan 		chan struct{}
}

const (
	maxBackoffFactor 	= 20
)

const DefaultPeriod = time.Minute

var _ FsHandler = &realFsHandler{}

func NewFsHandler(period time.Duration, rootfs, extraDir string, fsInfo fs.FsInfo) FsHandler {
	return &realFsHandler{
		lastUpdate: time.Time{},
		usage:      FsUsage{},
		period:     period,
		minPeriod:  period,
		rootfs:     rootfs,
		extraDir:   extraDir,
		fsInfo:     fsInfo,
		stopChan:   make(chan struct{}, 1),
	}
}

func (fh *realFsHandler) update() error {
	var (
		rootUsage, extraUsage 		fs.UsageInfo
		rootErr, extraErr 		error
	)
	if fh.rootfs != "" {
		rootUsage, rootErr = fh.fsInfo.GetDirUsage(fh.rootfs)
		klog.Infof("======>rootfs: %v, rootUsage: %v", fh.rootfs, rootUsage)
	}
	if fh.extraDir != "" {
		extraUsage, extraErr = fh.fsInfo.GetDirUsage(fh.extraDir)
		klog.Infof("======>extraDir: %v, extraUsage: %v", fh.extraDir, extraUsage)
	}

	fh.Lock()
	defer fh.Unlock()
	fh.lastUpdate = time.Now()
	if fh.rootfs != "" && rootErr == nil {
		fh.usage.InodeUsage = rootUsage.Inodes
		fh.usage.BaseUsageBytes = rootUsage.Bytes
		fh.usage.TotalUsageBytes = rootUsage.Bytes
	}
	if fh.extraDir != "" && extraErr == nil {
		fh.usage.TotalUsageBytes += extraUsage.Bytes
	}
	// Combine errors into a single error to return
	if rootErr != nil || extraErr != nil {
		return fmt.Errorf("rootDiskErr: %v, extraDiskErr: %v", rootErr, extraErr)
	}
	return nil
}

func (fh *realFsHandler) trackUsage() {
	longOp := time.Second
	for {
		start := time.Now()
		if err := fh.update(); err != nil {
			klog.Errorf("failed to collect filesystem stats - %v", err)
			fh.period = fh.period * 2
			if fh.period > maxBackoffFactor*fh.minPeriod {
				fh.period = maxBackoffFactor * fh.minPeriod
			}
		} else {
			fh.period = fh.minPeriod
		}
		duration := time.Since(start)
		if duration > longOp {
			// adapt longOp time so that message doesn't continue to print
			// if the long duration is persistent either because of slow
			// disk or lots of containers.
			longOp = longOp + time.Second
			klog.V(2).Infof("fs: disk usage and inodes count on following dirs took %v: %v; will not log again for this container unless duration exceeds %v", duration, []string{fh.rootfs, fh.extraDir}, longOp)
		}
		select {
		case <-fh.stopChan:
			return
		case <-time.After(fh.period):
		}
	}
}

func (fh *realFsHandler) Start() {
	go fh.trackUsage()
}

func (fh *realFsHandler) Stop() {
	close(fh.stopChan)
}

func (fh *realFsHandler) Usage() FsUsage {
	fh.RLock()
	defer fh.RUnlock()
	return fh.usage
}











































