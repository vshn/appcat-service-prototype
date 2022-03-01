# Set Shell to bash, otherwise some targets fail with dash/zsh etc.
SHELL := /bin/bash

# Disable built-in rules
MAKEFLAGS += --no-builtin-rules
MAKEFLAGS += --no-builtin-variables
.SUFFIXES:
.SECONDARY:
.DEFAULT_GOAL := help

# General variables
include Makefile.vars.mk
# KIND module
include kind/kind.mk

.PHONY: help
help: ## Show this help
	@grep -E -h '\s##\s' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

.PHONY: lint
lint: ## All-in-one linting
	@echo 'Check for uncommitted changes ...'
	git diff --exit-code

build: crossplane-setup provision-redis

.PHONY: crossplane-setup
crossplane-setup: $(crossplane_marker)

.service-redis: crossplane-setup
	kubectl apply -f crossplane/composite-redis.yaml
	kubectl apply -f crossplane/composition-redis.yaml

provision-redis: export KUBECONFIG = $(KIND_KUBECONFIG)
provision-redis: .service-redis
	kubectl apply -f service/prototype-instance.yaml
	kubectl wait -n my-app --for condition=Ready RedisInstance.syn.tools/redis1 --timeout 180s
	kubectl apply -f service/test-job.yaml
	kubectl wait -n my-app --for condition=Complete job/service-connection-verify

deprovision-redis: export KUBECONFIG = $(KIND_KUBECONFIG)
deprovision-redis: kind-setup
	kubectl delete -f service/prototype-instance.yaml --ignore-not-found

$(crossplane_marker): export KUBECONFIG = $(KIND_KUBECONFIG)
$(crossplane_marker): $(KIND_KUBECONFIG)
	helm repo add crossplane https://charts.crossplane.io/stable
	helm repo add mittwald https://helm.mittwald.de
	helm upgrade --install crossplane --create-namespace --namespace crossplane-system crossplane/crossplane --set "args[0]='--debug'" --wait
	helm upgrade --install secret-generator --create-namespace --namespace secret-generator mittwald/kubernetes-secret-generator --wait
	kubectl apply -f crossplane/provider.yaml
	kubectl wait --for condition=Healthy provider.pkg.crossplane.io/provider-helm --timeout 60s
	kubectl apply -f crossplane/provider-config.yaml
	kubectl create clusterrolebinding crossplane:provider-helm-admin --clusterrole cluster-admin --serviceaccount crossplane-system:$$(kubectl get sa -n crossplane-system -o custom-columns=NAME:.metadata.name --no-headers | grep provider-helm)
	@touch $@

clean: kind-clean
