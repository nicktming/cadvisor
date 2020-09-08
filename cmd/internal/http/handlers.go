package http

import (
	httpmux "github.com/google/cadvisor/cmd/internal/http/mux"
	"github.com/google/cadvisor/manager"
	"github.com/google/cadvisor/cmd/internal/api"
	"fmt"
)

func RegisterHandlers(mux httpmux.Mux, containerManager manager.Manager, urlBasePrefix string) error {
	if err := api.RegisterHandlers(mux, containerManager); err != nil {
		return fmt.Errorf("failed to register API handlers: %s", err)
	}


	return nil
}
