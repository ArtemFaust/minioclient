#!/bin/bash
echo "============ BUILD BINARIS ============"
cp -r src/go.mod builder/go.mod
cp -r src/go.sum builder/go.sum
cd ./builder
./builder.sh build arm64
./builder.sh build amd64
cd ../src
CGO_ENABLED=1 go build .
mv minioclient ../dist/macos/aarch64
echo "============ BUILD DONE ============"

echo "============ BUILD DOCKER IMAGE ============"
cd ../dist
docker rmi minioclient:amd64
docker rmi minioclient:arm64
docker build --platform=linux/amd64 . -t minioclient:amd64
docker build --platform=linux/arm64 . -t minioclient:arm64
echo "============ BUILD DONE ============"