# quartermaster

> Docker cluster and swarm manager for FASS (functions as a service) AWS/Remote clusters with token authentication

## Install

Uses [dep](https://github.com/golang/dep) to manage dependencies.

`$ dep ensure`


## Build

`$ ./build.sh`

## Run

The `token` option is a user defined random character sequence, for instance `$ uuidgen` on certain UNIX systems. Treat this as the universal key for controlling the swarm.

Using the `TOKEN` env:

`$ TOKEN=4jrs8-534js-345ds-3lrd0 ./quartermaster`

Using the `--token` flag:

`$ ./quartermaster --token=4jrs8-534js-345ds-3lrd0`


**Note:** all jobs are run on docker swarm nodes with `role=worker`


## Usage

#### /run

Run a service on any node in a swarm:

```curl
$ curl -X POST \
  http://localhost:9090/run \
  -H 'cache-control: no-cache' \
  -H 'content-type: application/json' \
  -d '{
  "token": "4jrs8-534js-345ds-3lrd0",
  "command": "node index.js",
  "image": "user/image",
  "name": "my_service",
  "labels": { "name": "value" },
  "memory": 1024
}'

$
{
  "success": true,
  "id": "a52f382c82a7d101b8ee3bbaa9ff59243b43c95915f1da579f35df0c4af0c822"
}
```

Options:
- token: Predefined token
- command: command to run in service (see docker service create)
- image: docker image to use
- name: name of service
- memory: memory limit for service
- labels: set of labels for the service

#### /stop

Stop and remove a service on the swarm:
```curl
$ curl -X POST \
  http://localhost:9090/stop \
  -H 'cache-control: no-cache' \
  -H 'content-type: application/json' \
  -d '{
  "token": "4jrs8-534js-345ds-3lrd0",
  "id": "a52f382c82a7d101b8ee3bbaa9ff59243b43c95915f1da579f35df0c4af0c822"
  }'

$
{
  "success": true,
  "id": "a52f382c82a7d101b8ee3bbaa9ff59243b43c95915f1da579f35df0c4af0c822"
}
```


#### /status/:id?token=<auth token>

Get the status of the service
```curl

curl -X GET \
  'http://localhost:9090/status/4k6ej6kuc67s55bbb5ufg5uao?token=6sdsd-94sdkf-43dsf-4245' \
  -H 'content-type: application/json' \
  -d '{
  "token": "6sdsd-94sdkf-43dsf-4245",
  "command": "sleep 50",
  "image": "alpine",
  "memory": 250,
  "labels": { "name": "value" },
  "type":"service"
  }'
$
{
  "success":true,
  "id":"szzn6kjmmxqc1wkhxhi16kah8",
  "status":"started"
}
```


### Authentication from private registries
Pass the `auth` key in the `run` request for it to pull from the private registry per swarm node.

The exact shape of the auth should conform to the docker auth spec:
```
{
  "username":"your_registry_username_or_email",
  "password":"*****",
  "auth":"",    // leave empty
  "email":"your@email.tld"
}
```
base64 encode the entire object as a string.

```curl
curl -X POST \
  http://localhost:9090/run \
  -H 'cache-control: no-cache' \
  -H 'content-type: application/json' \
  -d '{
  "token": "4jrs8-534js-345ds-3lrd0",
  "command": "sleep 50",
  "image": "user/private-image",
  "auth":"eyJ1c2VybmFtZSI6InVzZXIiLCAicGFzc3dvcmQiOiJwYXNzd29yZCIsICJhdXRoIjoiIiwgImVtYWlsIjoibmFtZUBleGFtcGxlLmNvbSJ9",
}'
```

## Docker

Because the quartermaster application connects to docker, the parent (swarm) docker instance sock is passed in as a volume:

#### Docker run
`$ docker run -d -e TOKEN=4jrs8-534js-345ds-3lrd0 -p 9090:9090 -v /var/run/docker.sock:/var/run/docker.sock stevelacy/quartermaster`

#### Docker swarm
**ensure the service is running only on a manager node**

```bash
docker service create --name quartermaster \
  -e TOKEN=4jrs8-534js-345ds-3lrd0 \
  -p 9090:9090 \
  --constraint 'node.role == manager' \
  --mount type=bind,source=/var/run/docker.sock,destination=/var/run/docker.sock \
  stevelacy/quartermaster
```

### Building the docker image

`$ ./build.sh`


## LICENSE [MIT](LICENSE)
