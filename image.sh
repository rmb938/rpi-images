#!/usr/bin/env bash

docker build -t rpi-image .
docker run --rm --privileged -v /dev:/dev -v $(pwd):/workspace rpi-image $@
