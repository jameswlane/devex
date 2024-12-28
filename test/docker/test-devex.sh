#!/bin/bash

# Build and run containers
docker-compose up --build --abort-on-container-exit

# Check exit status of each service
EXIT_CODE=0

for service in debian-test ubuntu-test; do
  STATUS=$(docker inspect --format='{{.State.ExitCode}}' devex_${service})
  if [ "$STATUS" -ne 0 ]; then
    echo "Test failed for $service with exit code $STATUS"
    EXIT_CODE=1
  fi
done

# Cleanup
docker-compose down

exit $EXIT_CODE
