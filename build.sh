#!/bin/bash

go get
CGO_ENABLED=0 go build
docker build -t stevelacy/quartermaster .
