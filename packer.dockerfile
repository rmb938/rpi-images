FROM mkaczanowski/packer-builder-arm

RUN apt-get update && apt-get install -y ansible psmisc
