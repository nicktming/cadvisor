package api

import (
	"github.com/google/cadvisor/manager"
	"net/http"
	"k8s.io/klog"
)

const (
	containersApi 		= "containers"
	subcontainersApi	= "subcontainers"
	machineApi 		= "machine"
)

type ApiVersion interface {
	Version() 			string
	SupportedRequestTypes()		[]string
	HandleRequest(requestType string, request []string, m manager.Manager, w http.ResponseWriter, r *http.Request) error
}

// Gets all supported API versions.
func getApiVersions() []ApiVersion {
	v1_0 := &version1_0{}

	return []ApiVersion{v1_0}

}

// API v1.0
type version1_0 struct {

}

func (api *version1_0) Version() string {
	return "v1.0"
}

func (api *version1_0) SupportedRequestTypes() []string {
	return []string{containersApi, machineApi}
}

func (api *version1_0) HandleRequest(requestType string, request []string, m manager.Manager, w http.ResponseWriter, r *http.Request) error {
	switch requestType {
	case machineApi:
		klog.Infof("Api - Machine")

		machineInfo, err := m.GetMachineInfo()
		if err != nil {
			return err
		}
		err = writeResult(machineInfo, w)
		if err != nil {
			return err
		}
	}
}
