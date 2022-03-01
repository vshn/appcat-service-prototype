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

.PHONY: crossplane-setup
crossplane-setup: $(crossplane_sentinel) ## Install local Kubernetes cluster and install Crossplane

.PHONY: .service-definition
.service-definition: crossplane-setup
	kubectl apply -f crossplane/composite.yaml
	kubectl apply -f crossplane/composition.yaml

.PHONY: provision
provision: export KUBECONFIG = $(KIND_KUBECONFIG)
provision: .service-definition ## Install local Kubernetes cluster and provision the service instance
	kubectl apply -f service/prototype-instance.yaml
	kubectl wait -n my-app --for condition=Ready RedisInstance.syn.tools/redis1 --timeout 180s
	kubectl apply -f service/test-job.yaml
	kubectl wait -n my-app --for condition=Complete job/service-connection-verify

.PHONY: deprovision
deprovision: export KUBECONFIG = $(KIND_KUBECONFIG)
deprovision: kind-setup ## Uninstall the service instance
	ns=$$(kubectl -n my-app get RedisInstance.syn.tools redis1 -o jsonpath={.spec.resourceRef.name}) && \
	kubectl delete -f service/prototype-instance.yaml && \
	kubectl delete ns $${ns}

$(crossplane_sentinel): export KUBECONFIG = $(KIND_KUBECONFIG)
$(crossplane_sentinel): $(KIND_KUBECONFIG)
	helm repo add crossplane https://charts.crossplane.io/stable
	helm repo add mittwald https://helm.mittwald.de
	helm upgrade --install crossplane --create-namespace --namespace crossplane-system crossplane/crossplane --set "args[0]='--debug'" --wait
	helm upgrade --install secret-generator --create-namespace --namespace secret-generator mittwald/kubernetes-secret-generator --wait
	kubectl apply -f crossplane/provider.yaml
	kubectl wait --for condition=Healthy provider.pkg.crossplane.io/provider-helm --timeout 60s
	kubectl apply -f crossplane/provider-config.yaml
	kubectl create clusterrolebinding crossplane:provider-helm-admin --clusterrole cluster-admin --serviceaccount crossplane-system:$$(kubectl get sa -n crossplane-system -o custom-columns=NAME:.metadata.name --no-headers | grep provider-helm)
	@touch $@

.PHONY: clean
clean: kind-clean ## Clean up local dev environment
