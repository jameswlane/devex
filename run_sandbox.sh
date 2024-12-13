# Build the Docker image with the root directory as the build context
docker build -t sandbox-cli -f sandbox/Dockerfile .

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