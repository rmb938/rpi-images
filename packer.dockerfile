FROM mkaczanowski/packer-builder-arm

RUN apt-get update && apt-get install -y ansible docker.io python3-pip && pip3 install docker
