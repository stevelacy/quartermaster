package manager

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/docker/docker/client"
	"github.com/julienschmidt/httprouter"
	"golang.org/x/net/context"
)

func Stop(w http.ResponseWriter, r *http.Request, p httprouter.Params, response PostRequest, ctx context.Context, cli *client.Client) {
	timeout := 0 * time.Second
	if response.Id == "" {
		payload := PostErrorResponse{Success: false, Error: "container id missing"}
		_ = json.NewEncoder(w).Encode(payload)
		return
	}

	err := cli.ContainerStop(ctx, response.Id, &timeout)
	if err != nil && strings.Contains(err.Error(), "Error response from daemon: No such container:") {
		// Try to remove the id as a service
		err := cli.ServiceRemove(ctx, response.Id)
		// Not a container or a service
		if err != nil {
			w.WriteHeader(400)
			payload := PostErrorResponse{
				Success: false,
				Error:   err.Error(),
				Code:    400,
				Id:      response.Id,
			}
			_ = json.NewEncoder(w).Encode(payload)
			return
		}
	}
	service, ok := services[response.Id]
	if ok {
		// Update recorded memory

		updatedServices := make(map[string]ServiceSpec)
		for _, s := range services {
			if s.Id != response.Id {
				updatedServices[s.Id] = s
			}
		}
		services = updatedServices

		updatedNode, ok := nodes[service.NodeId]
		if ok {
			updatedNode.AvailableMemory = updatedNode.AvailableMemory + service.Memory
			nodes[updatedNode.Id] = updatedNode
		}
	}

	payload := &PostSuccessResponse{Success: true, Id: response.Id}
	_ = json.NewEncoder(w).Encode(payload)
}
