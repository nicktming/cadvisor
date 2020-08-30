package common

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"os"
)

var ArgContainerHints = flag.String("container_hints", "/etc/cadvisor/container_hints.json", "location of the container hints file")

type ContainerHints struct {
	AllHosts []containerHint `json:"all_hosts,omitempty"`
}

type containerHint struct {
	FullName         string            `json:"full_path,omitempty"`
	NetworkInterface *networkInterface `json:"network_interface,omitempty"`
	Mounts           []Mount           `json:"mounts,omitempty"`
}

type Mount struct {
	HostDir      string `json:"host_dir,omitempty"`
	ContainerDir string `json:"container_dir,omitempty"`
}

type networkInterface struct {
	VethHost  string `json:"veth_host,omitempty"`
	VethChild string `json:"veth_child,omitempty"`
}

func GetContainerHintsFromFile(containerHintsFile string) (ContainerHints, error) {
	dat, err := ioutil.ReadFile(containerHintsFile)
	if os.IsNotExist(err) {
		return ContainerHints{}, nil
	}
	var cHints ContainerHints
	if err == nil {
		err = json.Unmarshal(dat, &cHints)
	}

	return cHints, err
}
