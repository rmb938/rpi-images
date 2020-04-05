FROM golang:1.14-buster

RUN git clone https://github.com/mkaczanowski/packer-builder-arm.git && \
    cd packer-builder-arm && \
    git checkout 6f9a6c3850ffbcd1bac401ec71309855009e7a19 && \
    go mod download && \
    go build

FROM debian:buster-slim

RUN apt update && apt install -y wget unzip qemu-user-static gdisk dosfstools python3-distutils ansible

ENV PACKER_VERSION=1.5.5

RUN wget -q https://releases.hashicorp.com/packer/${PACKER_VERSION}/packer_${PACKER_VERSION}_linux_amd64.zip && \
    wget -q https://releases.hashicorp.com/packer/${PACKER_VERSION}/packer_${PACKER_VERSION}_SHA256SUMS && \
    sed -i '/.*linux_amd64.zip/!d' packer_${PACKER_VERSION}_SHA256SUMS && \
    sha256sum -c packer_${PACKER_VERSION}_SHA256SUMS && \
    unzip packer_${PACKER_VERSION}_linux_amd64.zip -d /bin && \
    rm -f packer_${PACKER_VERSION}_linux_amd64.zip

COPY entrypoint.sh /entrypoint.sh
COPY --from=0 /go/packer-builder-arm/packer-builder-arm /root/.packer.d/plugins/

ENTRYPOINT ["/entrypoint.sh"]
