package manager

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/docker/docker/client"
	"github.com/julienschmidt/httprouter"
)

func Stop(w http.ResponseWriter, r *http.Request, p httprouter.Params, response PostRequest, ctx context.Context, cli *client.Client) {
	if response.Id == "" {
		payload := PostErrorResponse{Success: false, Error: "container id missing"}
		_ = json.NewEncoder(w).Encode(payload)
		return
	}

	err := cli.ServiceRemove(ctx, response.Id)
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
