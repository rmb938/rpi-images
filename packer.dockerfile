# anything higher than 1.0.3 is broken for some reason
FROM mkaczanowski/packer-builder-arm:1.0.3 

RUN apt-get update && apt-get install -y ansible psmisc
