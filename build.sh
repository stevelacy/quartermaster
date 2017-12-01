#!/bin/bash

go get
GOOS=linux GOARCH=386 CGO_ENABLED=0 go build
docker build -t stevelacy/quartermaster .
