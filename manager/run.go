package manager

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/julienschmidt/httprouter"
)

func Run(w http.ResponseWriter, r *http.Request, p httprouter.Params, response PostRequest, ctx context.Context, cli *client.Client) {
	if response.Command == "" {
		payload := PostErrorResponse{Success: false, Error: "command missing or invalid"}
		_ = json.NewEncoder(w).Encode(payload)
		return
	}
	if response.Image == "" {
		payload := PostErrorResponse{Success: false, Error: "image missing or invalid"}
		_ = json.NewEncoder(w).Encode(payload)
		return
	}

	command := strings.Split(response.Command, " ")

	requiredMemory := DEFAULT_MEMORY
	if response.Memory != 0 {
		requiredMemory = response.Memory * CONVERT_MB
	}

	// Testing only has one node, the master node
	// placement := &swarm.Placement{}

	// if flag.Lookup("test.v") == nil {
	//   placement = &swarm.Placement{
	//     Constraints: []string{"node.role == worker"},
	//   }
	// }

	// placement := &swarm.Placement{
	// 	Constraints: []string{
	// 		"node.role == worker",
	// 	},
	// }

	pullOptions := types.ImagePullOptions{}
	if response.Auth != "" {
		pullOptions = types.ImagePullOptions{
			RegistryAuth: response.Auth,
		}
	}
	_, err := cli.ImagePull(ctx, response.Image, pullOptions)

	if err != nil {
		payload := PostErrorResponse{Success: false, Error: err.Error(), Auth: response.Auth != ""}
		_ = json.NewEncoder(w).Encode(payload)
		return
	}

	replicas := uint64(0)

	serviceSpec := swarm.ServiceSpec{
		Annotations: swarm.Annotations{
			Name: response.Name,
		},
		Mode: swarm.ServiceMode{
			Replicated: &swarm.ReplicatedService{
				Replicas: &replicas,
			},
		},
		TaskTemplate: swarm.TaskSpec{
			ContainerSpec: swarm.ContainerSpec{
				Image:      response.Image,
				Labels:     response.Labels,
				Command:    command,
				StopSignal: "SIGINT",
			},
			Resources: &swarm.ResourceRequirements{
				Limits: &swarm.Resources{
					MemoryBytes: int64(requiredMemory),
				},
			},
			// Placement: placement,
			RestartPolicy: &swarm.RestartPolicy{
				Condition: "none",
			},
		},
	}

	serviceOptions := types.ServiceCreateOptions{}
	if response.Auth != "" {
		serviceOptions = types.ServiceCreateOptions{
			EncodedRegistryAuth: response.Auth,
		}
	}
	resp, err := cli.ServiceCreate(ctx, serviceSpec, serviceOptions)

	task := QueueSpec{
		ServiceSpec: serviceSpec,
		Id:          resp.ID,
		Cli:         *cli,
		Ctx:         ctx,
	}

	Queue <- task

	if err != nil {
		payload := PostErrorResponse{Success: false, Error: err.Error()}
		_ = json.NewEncoder(w).Encode(payload)
		return
	}

	payload := &PostSuccessResponse{Success: true, Id: resp.ID}
	_ = json.NewEncoder(w).Encode(payload)
	return
}
