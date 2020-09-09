package main

import (
	"github.com/google/cadvisor/manager"
	"github.com/google/cadvisor/utils/sysfs"
	"k8s.io/klog"
	"github.com/google/cadvisor/events"
	info "github.com/google/cadvisor/info/v1"
	"encoding/json"
	"time"
	cadvisormetrics "github.com/google/cadvisor/container"
	"net/http"

	cadvisorhttp "github.com/google/cadvisor/cmd/internal/http"
	"flag"
	"fmt"

	// Register container providers
	//_ "github.com/google/cadvisor/cmd/internal/container/install"
	//"os/user"
	//"github.com/google/cadvisor/container/docker"
)


var argIp = flag.String("listen_ip", "", "IP to listen on, defaults to all IPs")
var argPort = flag.Int("port", 9090, "port to listen")

var urlBasePrefix = flag.String("url_base_prefix", "", "prefix path that will be prepended to all paths to support some reverse proxies")

func main() {

	sysFs := sysfs.NewRealSysFs()

	memoryStorage, err := NewMemoryStorage()
	if err != nil {
		panic(err)
	}

	includedMetrics := cadvisormetrics.MetricSet {
		cadvisormetrics.CpuUsageMetrics:         struct{}{},
		cadvisormetrics.MemoryUsageMetrics:      struct{}{},
		cadvisormetrics.CpuLoadMetrics:          struct{}{},
		cadvisormetrics.DiskIOMetrics:           struct{}{},
		cadvisormetrics.NetworkUsageMetrics:     struct{}{},
		cadvisormetrics.AcceleratorUsageMetrics: struct{}{},
		cadvisormetrics.AppMetrics:              struct{}{},
	}

	ignored := []string{"/kubepods", "/user.slice/user-0.slice/session-462880.scope", "/system.slice/docker.service"}

	resourceManager, err := manager.New(memoryStorage, sysFs, includedMetrics, ignored)

	if err != nil {
		klog.Infof("Failed to create a manager: %s", err)
	}
	req := events.NewRequest()
	req.EventType[info.EventContainerCreation] = true
	req.EventType[info.EventContainerDeletion] = true
	req.EventType[info.EventOom] = true
	req.EventType[info.EventOomKill] = true

	//ec, err := resourceManager.WatchForEvents(req)

	if err := resourceManager.Start(); err != nil {
		klog.Fatal("Failed to start manager: %v", err)
	}

	//klog.Infof("====>got watcher Id: %v", ec.GetWatchId())
	//go func() {
	//	for  {
	//		select {
	//		case event := <- ec.GetChannel():
	//			klog.Infof("+++++++++++++++event containerName: %v, type: %v", event.ContainerName, event.EventType)
	//		}
	//	}
	//}()

	time.Sleep(5 * time.Second)

	request := info.DefaultContainerInfoRequest()
	request.NumStats = 1

	cinfo, err := resourceManager.GetContainerInfo("/", &request)
	if err != nil {
		panic(err)
	}

	klog.Infof("cinfo: %v/%v", cinfo.Namespace, cinfo.Name)

	pretty_cinfo, _ := json.MarshalIndent(cinfo, "", "\t")

	klog.Infof("pretty cinfo: %v", string(pretty_cinfo))



	fs, _ := resourceManager.GetDirFsInfo("/var/lib/kubelet")
	pretty_fs, _ := json.MarshalIndent(fs, "", "\t")

	klog.Infof("pretty fs: %v", string(pretty_fs))

	mux := http.NewServeMux()
	err = cadvisorhttp.RegisterHandlers(mux, resourceManager, *urlBasePrefix)
	if err != nil {
		klog.Fatalf("Failed to register HTTP handlers: %v", err)
	}

	rootMux := http.NewServeMux()
	rootMux.Handle(*urlBasePrefix + "/", http.StripPrefix(*urlBasePrefix, mux))

	addr := fmt.Sprintf("%s:%d", *argIp, *argPort)
	klog.Infof("start to listen at addr: %s", addr)
	klog.Fatal(http.ListenAndServe(addr, rootMux))


}

// TODO
// 1. watch raw removeWatchDirectory
// 2. MakeCgroupPaths no need to use /
// 3. watch oom
// 4. memory workingset (https://qingwave.github.io/container-memory/)
// 5. cpu info (https://www.cnblogs.com/ustcrliu/p/9049117.html)

