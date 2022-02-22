
## These are some common variables for Make

PROJECT_ROOT_DIR = .
# TODO: Adjust project meta
PROJECT_NAME ?= appcat-service-prototype
PROJECT_OWNER ?= vshn

## BUILD:docker
DOCKER_CMD ?= docker

## KIND:setup

# https://hub.docker.com/r/kindest/node/tags
KIND_NODE_VERSION ?= v1.23.0
KIND_IMAGE ?= docker.io/kindest/node:$(KIND_NODE_VERSION)
KIND ?= go run sigs.k8s.io/kind
KIND_KUBECONFIG ?= $(kind_dir)/kind-kubeconfig-$(KIND_NODE_VERSION)
KIND_CLUSTER ?= $(PROJECT_NAME)
