package manager

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/julienschmidt/httprouter"
)

// http://localhost:9090/status/4k6ej6kuc67s55bbb5ufg5uao?token=6sdsd-94sdkf-43dsf-4245

func Status(w http.ResponseWriter, r *http.Request, params httprouter.Params, response PostRequest, ctx context.Context, cli *client.Client) {
	id := params.ByName("id")
	if id == "" {
		payload := PostErrorResponse{Success: false, Error: "service id missing"}
		_ = json.NewEncoder(w).Encode(payload)
		return
	}

	services, err := cli.ServiceList(ctx, types.ServiceListOptions{})

	status := ""
	for _, service := range services {
		if id == service.ID {
			replicas := *service.Spec.Mode.Replicated.Replicas
			if replicas == 0 {
				status = "pending"
				break
			}

			tasks, err := cli.TaskList(ctx, types.TaskListOptions{})
			if err != nil {
				break
			}

			for _, task := range tasks {
				if task.ServiceID == service.ID {
					status = task.Status.Message
					break
				}
			}
		}
	}

	if err != nil {
		w.WriteHeader(400)
		payload := PostErrorResponse{
			Success: false,
			Error:   err.Error(),
			Code:    400,
			Auth:    true,
			Id:      id,
		}
		_ = json.NewEncoder(w).Encode(payload)
		return
	}

	if status == "" {
		payload := PostErrorResponse{
			Success: false,
			Error:   "Service not found",
			Code:    404,
			Auth:    true,
			Id:      id,
		}
		_ = json.NewEncoder(w).Encode(payload)
		return
	}

	payload := &PostSuccessResponse{
		Success: true,
		Id:      id,
		Status:  status,
	}
	_ = json.NewEncoder(w).Encode(payload)
}
