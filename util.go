package main

import (
	"k8s.io/klog"
	"fmt"
	"github.com/google/cadvisor/manager"
	cadvisorapiv2 "github.com/google/cadvisor/info/v2"
)

func getCadvisorContainerInfo(ca manager.Manager) (map[string]cadvisorapiv2.ContainerInfo, error) {
	infos, err := ca.GetContainerInfoV2("/", cadvisorapiv2.RequestOptions{
		IdType:    cadvisorapiv2.TypeName,
		Count:     2, // 2 samples are needed to compute "instantaneous" CPU
		Recursive: true,
	})
	if err != nil {
		if _, ok := infos["/"]; ok {
			// If the failure is partial, log it and return a best-effort
			// response.
			klog.Errorf("Partial failure issuing cadvisor.ContainerInfoV2: %v", err)
		} else {
			return nil, fmt.Errorf("failed to get root cgroup stats: %v", err)
		}
	}
	return infos, nil
}