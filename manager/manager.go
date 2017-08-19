package manager

import (
  "fmt"
  "log"
  // "flag"
  "time"
  "errors"
  "strings"
  "net/http"
  "encoding/json"

  "github.com/julienschmidt/httprouter"
  "github.com/docker/docker/client"
  "github.com/docker/docker/api/types"
  "github.com/docker/docker/api/types/swarm"
  "github.com/docker/docker/api/types/container"
  "golang.org/x/net/context"
)

var root_token = ""

type PostRequest struct {
  Token string `json:"token"`
  Command string `json:"command"`
  Image string `json:"image"`
  Auth string `json:"auth"`
  Type string `json:"type"`
  Label string `json:"label"`
  Name string `json:"name"`
  Id string `json:"id"`
}
type PostSuccessResponse struct {
  Success bool `json:"success"`
  Id string `json:"id"`
}
type PostErrorResponse struct {
  Success bool `json:"success"`
  Code int `json:"code"`
  Error string `json:"error"`
  Auth bool `json:"auth"`
  Id string `json:"id"`
}

func Init(token string, port string) {
  router := httprouter.New()
  root_token = token

  ctx := context.Background()
  cli, err := client.NewEnvClient()
  if err != nil {
    panic(err)
  }

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

  fmt.Println("Listening on port", port)
  log.Fatal(http.ListenAndServe(port, router))
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

  if response.Type == "service" {
    // Testing only has one node, the master node
    // placement := &swarm.Placement{}

    // if flag.Lookup("test.v") == nil {
    //   placement = &swarm.Placement{
    //     Constraints: []string{"node.role == worker"},
    //   }
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

    serviceSpec := swarm.ServiceSpec{
      Annotations: swarm.Annotations{
        Name: response.Name,
      },
      TaskTemplate: swarm.TaskSpec{
        ContainerSpec: &swarm.ContainerSpec{
          Image: response.Image,
          Labels: map[string]string{"name": response.Label},
          Command: command,
          StopSignal: "SIGINT",
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
    if err != nil {
      payload := PostErrorResponse{Success: false, Error: err.Error()}
      _ = json.NewEncoder(w).Encode(payload)
      return
    }
    payload := &PostSuccessResponse{Success: true, Id: resp.ID}
    _ = json.NewEncoder(w).Encode(payload)
    return
  }

  resp, err := cli.ContainerCreate(ctx, &container.Config{
    Image: response.Image,
    Cmd: command,
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
        Error: err.Error(),
        Code: 400,
        Id: response.Id,
      }
      _ = json.NewEncoder(w).Encode(payload)
      return
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
