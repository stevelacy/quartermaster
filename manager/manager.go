package manager

import (
	"fmt"
	// "flag"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/julienschmidt/httprouter"
	"golang.org/x/net/context"
)

var root_token string = ""
var CONVERT_MB int64 = 1048576
var CONVERT_CPU int64 = 1000000000
var DEFAULT_MEMORY int64 = 1024 * CONVERT_MB // 1GB
var NODE_INTERVAL time.Duration = time.Minute * 5
var SERVICE_INTERVAL time.Duration = time.Second * 10

var Queue = make(chan QueueSpec, 100)

type PostRequest struct {
	Token   string `json:"token"`
	Command string `json:"command"`
	Image   string `json:"image"`
	Auth    string `json:"auth"`
	Type    string `json:"type"`
	Label   string `json:"label"`
	Name    string `json:"name"`
	Id      string `json:"id"`
	// Cpu     int64  `json:"cpu"`
	Memory int64 `json:"memory"`
}
type PostSuccessResponse struct {
	Success bool   `json:"success"`
	Id      string `json:"id"`
}
type PostErrorResponse struct {
	Success bool   `json:"success"`
	Code    int    `json:"code"`
	Error   string `json:"error"`
	Auth    bool   `json:"auth"`
	Id      string `json:"id"`
}
type NodeSpec struct {
	Id              string
	Hostname        string
	Name            string
	OS              string
	Arch            string
	Engine          string
	Status          string
	Addr            string
	Role            string
	State           string
	Memory          int64
	AvailableMemory int64
	Cpu             int64
	Version         swarm.Version
}

type ServiceSpec struct {
	Id        string
	Name      string
	Role      string
	Mode      string
	Memory    int64
	Replicas  uint64
	Cpu       int64
	Placement swarm.Placement
	Image     string
	Command   []string
	NodeId    string
}

type QueueSpec struct {
	ServiceSpec swarm.ServiceSpec
	Id          string
	Cli         client.Client
	Ctx         context.Context
}

var nodes = make(map[string]NodeSpec)
var services = make(map[string]ServiceSpec)

func Init(token string) http.Handler {
	router := httprouter.New()
	root_token = token

	ctx := context.Background()
	cli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	CollectNodes(ctx, cli)    // Fire node interval, then start timer
	CollectServices(ctx, cli) // Fire service interval, then start timer

	nodeTicker := time.NewTicker(NODE_INTERVAL)
	go func() {
		for _ = range nodeTicker.C {
			CollectNodes(ctx, cli)
		}
	}()

	serviceTicker := time.NewTicker(SERVICE_INTERVAL)
	go func() {
		for _ = range serviceTicker.C {
			CollectServices(ctx, cli)
		}
	}()

	go func() {
		for {
			select {
			case task := <-Queue:
				go func() {
					availableNode := NodeSpec{Id: ""}

					for availableNode.Id == "" {
						for _, node := range nodes {
							// TODO: global style mode for requesting nodes
							// if node.AvailableMemory > task.ServiceSpec.TaskTemplate.Resources.Limits.MemoryBytes {
							fmt.Println(node.AvailableMemory, task.ServiceSpec.TaskTemplate.Resources.Limits.MemoryBytes)
							if node.AvailableMemory > task.ServiceSpec.TaskTemplate.Resources.Limits.MemoryBytes {
								availableNode = node
								break
							}
							// Wait 5 seconds to retry
							time.Sleep(time.Second * 5)
						}
					}

					serviceInspect, _, err := cli.ServiceInspectWithRaw(task.Ctx, task.Id, types.ServiceInspectOptions{})
					if err != nil {
						fmt.Println(err)
						return
					}

					placement := &swarm.Placement{
						Constraints: []string{
							"node.role == worker", // Disable for local testing
							fmt.Sprintf("node.id == %v", availableNode.Id),
						},
					}

					replicas := uint64(1)
					requiredMemory := task.ServiceSpec.TaskTemplate.Resources.Limits.MemoryBytes

					task.ServiceSpec.TaskTemplate.Placement = placement
					task.ServiceSpec.Mode.Replicated.Replicas = &replicas
					task.ServiceSpec.Annotations.Name = serviceInspect.Spec.Name

					// Update service on the swarm
					_, err = cli.ServiceUpdate(task.Ctx, task.Id, serviceInspect.Version, task.ServiceSpec, types.ServiceUpdateOptions{})
					if err != nil {
						fmt.Println(err)
						return
					}

					addService := ServiceSpec{
						Id:        task.Id,
						Name:      task.ServiceSpec.Annotations.Name,
						Placement: *placement,
						Memory:    requiredMemory,
						Image:     task.ServiceSpec.TaskTemplate.ContainerSpec.Image,
						Command:   task.ServiceSpec.TaskTemplate.ContainerSpec.Command,
						NodeId:    availableNode.Id,
						Replicas:  replicas,
					}

					services[task.Id] = addService

					// Update recorded memory
					updatedNode := nodes[availableNode.Id]
					updatedNode.AvailableMemory = updatedNode.AvailableMemory - requiredMemory
					nodes[availableNode.Id] = updatedNode
				}()
			}
		}
	}()

	router.POST("/run", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		response, err := HandleAuth(w, r)
		if err != nil {
			fmt.Fprint(w, err)
			return
		}
		Run(w, r, p, response, ctx, cli)
	})

	router.POST("/stop", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		response, err := HandleAuth(w, r)
		if err != nil {
			return
		}
		Stop(w, r, p, response, ctx, cli)
	})

	return router
}

func CollectNodes(ctx context.Context, cli *client.Client) {
	// Find all nodes in the swarm
	nodeOptions := types.NodeListOptions{}
	nodeList, err := cli.NodeList(ctx, nodeOptions)
	if err != nil {
		fmt.Println(err)
	}
	if len(nodeList) == 0 {
		fmt.Println(errors.New("Error - no nodes found. Is this a swarm?"))
	}
	fmt.Println("CollectNodes", len(nodeList))
	for _, node := range nodeList {
		details, _, err := cli.NodeInspectWithRaw(ctx, node.ID)
		if err != nil {
			fmt.Println(err)
		}
		nodeSpec := NodeSpec{
			Hostname: details.Description.Hostname,
			Name:     details.Spec.Name,
			Id:       details.ID,
			Arch:     details.Description.Platform.Architecture,
			OS:       details.Description.Platform.OS,
			Addr:     details.Status.Addr,
			Role:     string(details.Spec.Role),
			Status:   string(details.Status.Message),
			State:    string(details.Status.State),
			Cpu:      details.Description.Resources.NanoCPUs / CONVERT_CPU,   // Convert from NanoCPUs to cpus
			Memory:   details.Description.Resources.MemoryBytes / CONVERT_MB, // Convert bytes to MB
			Version:  details.Meta.Version,
		}
		_, exists := nodes[nodeSpec.Id]
		if exists {
			nodeSpec.AvailableMemory = nodes[nodeSpec.Id].AvailableMemory
		}
		if nodeSpec.AvailableMemory == 0 {
			nodeSpec.AvailableMemory = nodeSpec.Memory * CONVERT_MB
		}
		nodes[nodeSpec.Id] = nodeSpec
		// TODO: remove if not existing
	}
}

func CollectServices(ctx context.Context, cli *client.Client) {
	taskList, err := cli.TaskList(ctx, types.TaskListOptions{})
	if err != nil {
		fmt.Println(err)
	}

	running := 0
	for _, task := range taskList {
		if task.Status.State == "running" {
			running = running + 1
		}
	}
	fmt.Printf("CollectServices: total: %v running: %v \n", len(taskList), running)

	// Check if they are deleted
	filtered := make(map[string]ServiceSpec)
	for _, existing := range services {
		found := false
		for _, listed := range taskList {
			if listed.ServiceID == existing.Id && listed.Status.State == "running" {
				filtered[existing.Id] = existing
				found = true
				break
			}
		}
		if found == false {
			// Delete it
			updatedNode := nodes[existing.NodeId]
			updatedNode.AvailableMemory = updatedNode.AvailableMemory + existing.Memory
			nodes[existing.NodeId] = updatedNode
		}
	}
	services = filtered
}

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

	if response.Type == "service" {
		// Testing only has one node, the master node
		// placement := &swarm.Placement{}

		// if flag.Lookup("test.v") == nil {
		//   placement = &swarm.Placement{
		//     Constraints: []string{"node.role == worker"},
		//   }
		// }

		placement := &swarm.Placement{
			Constraints: []string{
				"node.role == worker",
			},
		}

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
				ContainerSpec: &swarm.ContainerSpec{
					Image:      response.Image,
					Labels:     map[string]string{"name": response.Label},
					Command:    command,
					StopSignal: "SIGINT",
				},
				Resources: &swarm.ResourceRequirements{
					Limits: &swarm.Resources{
						MemoryBytes: int64(requiredMemory),
					},
				},
				Placement: placement,
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

	// Not a service, container

	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: response.Image,
		Cmd:   command,
	}, nil, nil, response.Name)
	if err != nil {
		payload := PostErrorResponse{Success: false, Error: err.Error()}
		_ = json.NewEncoder(w).Encode(payload)
		return
	}
	err = cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		payload := PostErrorResponse{Success: false, Error: err.Error()}
		_ = json.NewEncoder(w).Encode(payload)
		return
	}

	payload := &PostSuccessResponse{Success: true, Id: resp.ID}
	_ = json.NewEncoder(w).Encode(payload)
}

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

func HandleAuth(w http.ResponseWriter, r *http.Request) (PostRequest, error) {
	w.Header().Set("Content-Type", "application/json")
	var response PostRequest
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&response)
	if err != nil {
		w.WriteHeader(400)
		payload := PostErrorResponse{Success: false, Error: err.Error(), Code: 400}
		_ = json.NewEncoder(w).Encode(payload)
		return PostRequest{}, err
	}
	if response.Token != root_token {
		err := errors.New("Unauthorized")
		w.WriteHeader(401)
		payload := PostErrorResponse{Success: false, Error: err.Error(), Code: 400}
		_ = json.NewEncoder(w).Encode(payload)
		return PostRequest{}, err
	}
	return response, nil
}
