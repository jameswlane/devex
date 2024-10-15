# Dockerfile
FROM debian:bullseye

# Install GNOME and other dependencies
RUN apt-get update && \
    apt-get install -y gnome-session gnome-terminal x11-apps sudo golang

# Add a non-root user
RUN useradd -ms /bin/bash sandboxuser && \
    echo 'sandboxuser ALL=(ALL) NOPASSWD:ALL' >> /etc/sudoers

# Switch to the non-root user
USER sandboxuser
WORKDIR /home/sandboxuser

# Copy your CLI application into the container
COPY . /home/sandboxuser/app
WORKDIR /home/sandboxuser/app

# Build the CLI application
RUN go build -o your-cli .

# Run your CLI application
CMD ["gnome-session"]