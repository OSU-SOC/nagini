#!/bin/bash

docker build -t nagini . --no-cache
container_id=$(docker run -d -it nagini bash)
echo $container_id
rm -f output/nagini || true
docker cp $container_id:/root/go/bin/nagini ./output/nagini
docker kill $container_id
docker rmi nagini --force