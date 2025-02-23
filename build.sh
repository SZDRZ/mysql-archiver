#!/bin/bash

module_name=$(cat go.mod | grep "module" | head -n1 | awk '{print $2}')

if command -v docker &>/dev/null; then
    echo "Docker detected. Starting build with Docker..."
    sudo rm -rf bin/${module_name} && sudo docker run -v $PWD:/workspace golang:1.21.10 build -o bin/${module_name}
    build_status=$?
else
    echo "Docker command not found. Try to local build..."
    go build -o bin/
    build_status=$?
fi

if [ $build_status -eq 0 ]; then
    echo "Build success!"
else
    echo "Build failed!"
fi