package manager

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/julienschmidt/httprouter"
)

var root_token string = ""
var CONVERT_MB int64 = 1048576
var CONVERT_CPU int64 = 1000000000
var DEFAULT_MEMORY int64
var NODE_INTERVAL time.Duration = time.Minute * 5
var SERVICE_INTERVAL time.Duration = time.Second * 30
var QUEUE_CAP = 1000

var Queue = make(chan QueueSpec, QUEUE_CAP)

type PostRequest struct {
	Token   string            `json:"token"`
	Command string            `json:"command"`
	Image   string            `json:"image"`
	Auth    string            `json:"auth"`
	Labels  map[string]string `json:"labels`
	Name    string            `json:"name"`
	Id      string            `json:"id"`
	Memory  int64             `json:"memory"`
}

type PostSuccessResponse struct {
	Success bool   `json:"success"`
	Id      string `json:"id"`
	Status  string `json:"status"`
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

func Init(token string, memory int64) http.Handler {
	DEFAULT_MEMORY = memory * CONVERT_MB
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
							if node.State != "ready" {
								continue
							}
							if node.AvailableMemory > task.ServiceSpec.TaskTemplate.Resources.Limits.MemoryBytes {
								availableNode = node
								fmt.Println("scheduled task on node: ", node.Id, "node mem: ", node.AvailableMemory/CONVERT_MB, "task mem:", task.ServiceSpec.TaskTemplate.Resources.Limits.MemoryBytes/CONVERT_MB)
								break
							}
						}
						// Wait 5 seconds to skip or retry
						time.Sleep(time.Second * 5)
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

					fmt.Println("Starting task: ", task.Id, task.ServiceSpec.Name)

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
		response, err := HandlePostAuth(w, r)
		if err != nil {
			fmt.Fprint(w, err)
			return
		}
		Run(w, r, p, response, ctx, cli)
	})

	router.POST("/stop", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		response, err := HandlePostAuth(w, r)
		if err != nil {
			fmt.Fprint(w, err)
			return
		}
		Stop(w, r, p, response, ctx, cli)
	})

	router.GET("/status/:id", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		response, err := HandleGetAuth(w, r)
		if err != nil {
			fmt.Fprint(w, err)
			return
		}
		Status(w, r, p, response, ctx, cli)
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
		if nodeSpec.Role == "manager" {
			// Skip the managers
			continue
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
	fmt.Println("Collect Worker Nodes:", len(nodes))
}

func CollectServices(ctx context.Context, cli *client.Client) {
	taskList, err := cli.TaskList(ctx, types.TaskListOptions{})
	if err != nil {
		fmt.Println(err)
	}

	counts := make(map[string]int)

	for _, task := range taskList {
		updatedCount := counts[string(task.Status.State)] + 1
		counts[string(task.Status.State)] = updatedCount
	}
	fmt.Printf("Services: total: %v, running: %v, stopped: %v, complete: %v, shutdown: %v, failed: %v \n", len(taskList), counts["running"], counts["stopped"], counts["complete"], counts["shutdown"], counts["failed"])

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
			fmt.Println("Deleting service", existing.NodeId, existing.Name)
			updatedNode := nodes[existing.NodeId]
			updatedNode.AvailableMemory = updatedNode.AvailableMemory + existing.Memory
			nodes[existing.NodeId] = updatedNode
		}
	}
	services = filtered
}
