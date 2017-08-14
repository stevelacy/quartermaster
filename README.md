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
  "type": "service"
}'
```

## Docker

Because the quartermaster application connects to docker, the parent (swarm) docker instance sock is passed in as a volume:

#### Docker run
`$ docker run -d -e TOKEN=4jrs8-534js-345ds-3lrd0 -p 9090:9090 -v /var/run/docker.sock:/var/run/docker.sock stevelacy/quartermaster`
