#!/bin/bash

# Navigate to the sandbox directory
cd "$(dirname "$0")/sandbox"

# Build the Docker image
docker build -t sandbox-cli .

# Allow local connections to the X server
xhost +local:root

# Run the Docker container with X11 forwarding
docker run -it \
    --env="DISPLAY" \
    --env="XAUTHORITY=${XAUTHORITY}" \
    --volume="/tmp/.X11-unix:/tmp/.X11-unix:rw" \
    sandbox-cli

# Revoke local connections to the X server
xhost -local:root