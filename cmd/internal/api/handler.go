package api

import (
	"github.com/google/cadvisor/manager"
	httpmux "github.com/google/cadvisor/cmd/internal/http/mux"
	"net/http"
	"time"
	"k8s.io/klog"
	"strings"
	"fmt"
	"sort"
	"regexp"
	"encoding/json"
)


const (
	apiResource	= 	"/api/"
)

// Captures the API version, requestType [optional], and remaining request [optional].
var apiRegexp = regexp.MustCompile(`/api/([^/]+)/?([^/]+)?(.*)`)

const (
	apiVersion = iota + 1
	apiRequestType
	apiRequestArgs
)

func RegisterHandlers(mux httpmux.Mux, m manager.Manager) error {
	apiVersions := getApiVersions()
	supportedApiVersions := make(map[string]ApiVersion, len(apiVersions))
	for _, v := range apiVersions {
		supportedApiVersions[v.Version()] = v
	}

	mux.HandleFunc(apiResource, func(w http.ResponseWriter, r *http.Request){
		klog.Infof("=======RegisterHandlers api handler=======")
		err := handleRequest(supportedApiVersions, m, w, r)
		if err != nil {
			http.Error(w, err.Error(), 500)
		}
	})
	return nil
}

func handleRequest(supportedApiVersions map[string]ApiVersion, m manager.Manager, w http.ResponseWriter, r *http.Request) error {
	start := time.Now()
	defer func() {
		klog.Infof("Request took %s", time.Since(start))
	}()

	request := r.URL.Path
	const apiPrefix = "/api"
	if !strings.HasPrefix(request, apiPrefix) {
		return fmt.Errorf("incomplete API request %q", request)
	}

	if request == apiPrefix || request == apiResource {
		versions := make([]string, 0, len(supportedApiVersions))
		for v := range supportedApiVersions {
			versions = append(versions, v)
		}
		sort.Strings(versions)
		http.Error(w, fmt.Sprintf("Supported API versions: %s", strings.Join(versions, ",")), http.StatusBadRequest)
		return nil
	}
	requestElements := apiRegexp.FindStringSubmatch(request)
	if len(requestElements) == 0 {
		return fmt.Errorf("malformed request %q", request)
	}
	version := requestElements[apiVersion]
	requestType := requestElements[apiRequestType]
	requestArgs := strings.Split(requestElements[apiRequestArgs], "/")

	klog.Infof("request: %v, version: %v, requestType: %v, requestArgs: %v",
		request, version, requestType, requestArgs)

	versionHandler, ok := supportedApiVersions[version]
	if !ok {
		return fmt.Errorf("unsupported API version %q", version)
	}

	if requestType == "" {
		requestTypes := versionHandler.SupportedRequestTypes()
		sort.Strings(requestTypes)
		http.Error(w, fmt.Sprintf("Supported request types: %q", strings.Join(requestTypes, ",")), http.StatusBadRequest)
		return nil
	}

	if len(requestArgs) > 0 && requestArgs[0] == "" {
		requestArgs = requestArgs[1:]
	}

	return versionHandler.HandleRequest(requestType, requestArgs, m, w, r)
}

func writeResult(res interface{}, w http.ResponseWriter) error {
	out, err := json.Marshal(res)
	if err != nil {
		return fmt.Errorf("failed to marshall response %+v with error: %s", res, err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(out)
	return nil
}