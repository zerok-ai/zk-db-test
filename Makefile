NAME = zk-redis-test
IMAGE_NAME = zk-redis-test
IMAGE_VERSION = latest

LOCATION ?= us-west1
PROJECT_ID ?= zerok-dev
REPOSITORY ?= zk-client

export GO111MODULE=on
export BUILDER_NAME=multi-platform-builder
export GOPRIVATE=github.com/zerok-ai/zk-utils-go

sync:
	go get -v ./...

build: sync
	$(GOOS) $(ARCH) go build -o bin/$(NAME) cmd/main.go

run: build
	go run cmd/main.go -c ./config/config.yaml

docker-build: build
	# generate a random hash of 3 characters
	$(eval BUILD_NUMBER := $(shell head -c 3 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | fold -w 3 | head -n 1))

	echo "BUILD_NUMBER=$(BUILD_NUMBER)"

	# run docker build
	docker build $(DockerFile) --build-arg BUILD_NUMBER=$(BUILD_NUMBER) -t $(IMAGE_PREFIX)$(IMAGE_NAME)$(IMAGE_NAME_SUFFIX):$(IMAGE_VERSION) .

docker-push:
	docker push $(IMAGE_PREFIX)$(IMAGE_NAME)$(IMAGE_NAME_SUFFIX):$(IMAGE_VERSION)


# ------- GKE ------------

# build app image
docker-build-gke: GOOS := GOOS=linux
docker-build-gke: ARCH := GOARCH=amd64
docker-build-gke: IMAGE_PREFIX := $(LOCATION)-docker.pkg.dev/$(PROJECT_ID)/$(REPOSITORY)/
docker-build-gke: DockerFile := -f Dockerfile
docker-build-gke: docker-build

# push app image
docker-push-gke: IMAGE_PREFIX := $(LOCATION)-docker.pkg.dev/$(PROJECT_ID)/$(REPOSITORY)/
docker-push-gke: docker-push

# build and push
docker-build-push-gke: docker-build-gke docker-push-gke

