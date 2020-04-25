DOCKER_IMAGE=local/packer-builder-arm
DOCKER_CMD=docker run --rm -it --privileged -v /dev:/dev -v $$(pwd)/images:/images -w /images ${DOCKER_IMAGE} build

docker-build:
	docker build -t ${DOCKER_IMAGE} -f packer.dockerfile .


build-%:
	 ${DOCKER_CMD} $*/packer.json -extra-system-packages=gdisk

build: docker-build build-base build-cloud-init build-harden

clean:
	sudo rm -rf images/*.img
	sudo rm -rf images/*.tar.gz
	sudo rm -rf images/packer_cache
