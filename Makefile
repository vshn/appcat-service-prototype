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

build: crossplane-setup instance-redis service-broker

crossplane-setup: export KUBECONFIG = $(KIND_KUBECONFIG)
crossplane-setup: $(kind_dir)/crossplane-ready

service-broker: export KUBECONFIG = $(KIND_KUBECONFIG)
service-broker: kind-setup-ingress
	kubectl apply -k service-broker

.service-redis:
	kubectl apply -f crossplane/composite-redis.yaml
	kubectl apply -f crossplane/composition-redis-small.yaml

instance-redis: export KUBECONFIG = $(KIND_KUBECONFIG)
instance-redis: .service-redis
	kubectl apply -f service/prototype-instance.yaml
	kubectl wait -n my-app --for condition=Ready RedisInstance.syn.tools/redis1 --timeout 180s

svcat: export KUBECONFIG = $(KIND_KUBECONFIG)
svcat: $(kind_dir)/.svcat-ready

$(kind_dir)/svcat-ready:
	helm repo add service-catalog https://kubernetes-sigs.github.io/service-catalog
	helm install catalog --create-namespace --namespace catalog service-catalog/catalog --wait
	@touch $@

$(kind_dir)/crossplane-ready: kind-setup
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
