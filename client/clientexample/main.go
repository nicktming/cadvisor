// Copyright 2014 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"

	"github.com/google/cadvisor/client"
	info "github.com/google/cadvisor/info/v1"

	"k8s.io/klog/v2"
)

func staticClientTmingExample() {
	staticClient, err := client.NewClient("http://localhost:8080/")
	if err != nil {
		klog.Errorf("tried to make client and got error %v", err)
		return
	}
	machine, err := staticClient.MachineInfo()
	if err != nil {
		klog.Errorf("got machine info: %v", err)
		return
	}
	klog.Infof("machine: %v", machine)


	ci, err := staticClient.ContainerInfo("busybox", nil)
	if err != nil {
		klog.Errorf("got machine info: %v", err)
		return
	}
	klog.Infof("ci: %v", ci)
}

func staticClientExample() {
	klog.Infof("------staticClientExample start 111--------")
	staticClient, err := client.NewClient("http://localhost:8080/")
	if err != nil {
		klog.Errorf("tried to make client and got error %v", err)
		return
	}
	klog.Infof("------staticClientExample start 222--------")
	einfo, err := staticClient.EventStaticInfo("?oom_events=true")
	if err != nil {
		klog.Errorf("got error retrieving event info: %v", err)
		return
	}
	klog.Infof("------staticClientExample start 333--------")
	for idx, ev := range einfo {
		klog.Infof("static einfo %v: %v", idx, ev)
	}
	klog.Infof("------staticClientExample end--------")
}

func streamingClientExample(url string) {
	streamingClient, err := client.NewClient("http://localhost:8080/")
	if err != nil {
		klog.Errorf("tried to make client and got error %v", err)
		return
	}
	einfo := make(chan *info.Event)
	go func() {
		err = streamingClient.EventStreamingInfo(url, einfo)
		if err != nil {
			klog.Errorf("got error retrieving event info: %v", err)
			return
		}
	}()
	for ev := range einfo {
		klog.Infof("streaming einfo: %v\n", ev)
	}
}

// demonstrates how to use event clients
func main() {
	klog.InitFlags(nil)
	flag.Parse()
	staticClientExample()
	staticClientTmingExample()
	streamingClientExample("?creation_events=true&stream=true&oom_events=true&deletion_events=true")
}
