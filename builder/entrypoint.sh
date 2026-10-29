#!/bin/bash
cd /opt/src
if [[ $(uname -m) == "x86_64" ]];
then
  CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build .
  mv minioclient /opt/dist/linux/amd64/minioclient
elif [[ $(uname -m) == "aarch64" ]];
then
  CGO_ENABLED=1 GOOS=linux GOARCH=arm64 go build .
  mv minioclient /opt/dist/linux/aarch64/minioclient
fi