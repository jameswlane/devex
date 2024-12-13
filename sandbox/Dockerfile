# Dockerfile
FROM golang:1.21 AS builder

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go.mod and go.sum files from the root of the project
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod file is not changed
RUN go mod download

# Copy the source from the root directory to the Working Directory inside the container
COPY . .

# Build the Go app and place the binary in the bin directory
RUN go build -o bin/devex cmd/devex/main.go

# Start a new stage from the debian base image
FROM debian:bookworm

# Install GNOME, dbus-x11, systemd, and other dependencies
RUN apt-get update && \
    apt-get install -y gnome-session gnome-terminal x11-apps sudo wget dbus-x11 systemd systemd-sysv

# Add a non-root user
RUN useradd -ms /bin/bash sandboxuser && \
    echo 'sandboxuser ALL=(ALL) ALL' >> /etc/sudoers

# Switch to the non-root user
USER sandboxuser
WORKDIR /home/sandboxuser

# Create the necessary directories
RUN mkdir -p /home/sandboxuser/.local/share/devex/bin \
    /home/sandboxuser/.local/share/devex/config \
    /home/sandboxuser/.local/share/devex/assets

# Copy the pre-built binary from the builder stage
COPY --from=builder /app/bin/devex /home/sandboxuser/.local/share/devex/bin/devex

# Copy the configuration and assets files
COPY --from=builder /app/config /home/sandboxuser/.local/share/devex/config
COPY --from=builder /app/assets /home/sandboxuser/.local/share/devex/assets

# Update the PATH environment variable
ENV PATH="/home/sandboxuser/.local/share/devex/bin:${PATH}"

# Enable systemd
ENV container docker
STOPSIGNAL SIGRTMIN+3
CMD ["/lib/systemd/systemd"]