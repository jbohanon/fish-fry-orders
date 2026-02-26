.PHONY: all build test clean proto backend docker deploy helm-lint helm-template frontend

# Container registry - override with: make docker-push REGISTRY=myregistry.io
REGISTRY ?= git.nonahob.net
IMAGE_NAME_API ?= jacob/fish-fry-orders-api
IMAGE_NAME_FRONTEND ?= jacob/fish-fry-orders-frontend
VERSION ?= $(shell git rev-parse HEAD)

# Environment configuration
# Usage: make helm-deploy ENV=demo
#   ENV=prod  -> fish-fry-orders namespace, stmichaelfishfry.com
#   ENV=demo  -> fish-fry-orders-demo namespace, fish-fry-demo.nonahob.net
#   ENV=dev   -> fish-fry-orders-dev namespace, fish-fry-dev.nonahob.net
ENV ?= prod

# Map environment to configuration
ifeq ($(ENV),prod)
  CONFIG_DIR := fish-fry-orders
  NAMESPACE := fish-fry-orders
  RELEASE_NAME := fish-fry-orders
else ifeq ($(ENV),demo)
  CONFIG_DIR := fish-fry-orders-demo
  NAMESPACE := fish-fry-orders-demo
  RELEASE_NAME := fish-fry-orders-demo
else ifeq ($(ENV),dev)
  CONFIG_DIR := fish-fry-orders-dev
  NAMESPACE := fish-fry-orders-dev
  RELEASE_NAME := fish-fry-orders-dev
else
  $(error Unknown ENV: $(ENV). Use prod, demo, or dev)
endif

HELM_CHART := helm/fish-fry-orders
VALUES_FILE := $(HELM_CHART)/configurations/$(CONFIG_DIR)/values.yaml
HELM_IMAGE_SET_ARGS := --set backend.image.tag=$(VERSION) --set frontend.image.tag=$(VERSION)
HELM_FRONTEND_VERSION_SET_ARGS :=

ifeq ($(ENV),dev)
  BUILD_DATE_UTC ?= $(shell date -u +%Y%m%dT%H%M%SZ)
  HELM_FRONTEND_VERSION_SET_ARGS := --set-string frontend.version=dev-$(BUILD_DATE_UTC)-$(VERSION)
endif

HELM_SET_ARGS := $(HELM_IMAGE_SET_ARGS) $(HELM_FRONTEND_VERSION_SET_ARGS)

all: proto backend

# Protocol Buffer generation
proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/*.proto

# Backend build
backend:
	mkdir -p _output
	go mod tidy
	go build -o _output/server cmd/server/main.go

# Testing
test:
	go test ./... -v

# Run E2E tests (requires Docker)
test-e2e:
	go test ./testing/... -v

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf _output/

# Frontend build (React)
frontend:
	cd frontend && npm ci && npm run build

# Docker build
docker-api:
	docker build -t $(REGISTRY)/$(IMAGE_NAME_API):$(VERSION) .

docker-frontend:
	docker build -t $(REGISTRY)/$(IMAGE_NAME_FRONTEND):$(VERSION) frontend/

docker: docker-api docker-frontend

# Docker push
docker-push: docker
	docker push $(REGISTRY)/$(IMAGE_NAME_API):$(VERSION)
	docker push $(REGISTRY)/$(IMAGE_NAME_FRONTEND):$(VERSION)

# Development server (Go backend)
dev:
	go run cmd/server/main.go

# Development server (React frontend) - runs on port 5173, proxies to Go backend on 8080
dev-frontend:
	cd frontend && npm run dev

# Database migrations
migrate:
	go run cmd/migrate/main.go -command up

migrate-status:
	go run cmd/migrate/main.go -command status

migrate-down:
	go run cmd/migrate/main.go -command down

# =============================================================================
# Helm chart operations (environment-aware)
# Usage: make helm-deploy ENV=demo
# =============================================================================

helm-lint:
	helm lint $(HELM_CHART)

# Template the chart with environment-specific values
helm-template:
	@echo "Templating for environment: $(ENV) (namespace: $(NAMESPACE))"
	helm template $(RELEASE_NAME) $(HELM_CHART) \
		-f $(VALUES_FILE) \
		-n $(NAMESPACE) \
		$(HELM_SET_ARGS)

# Install the chart for the specified environment
helm-install:
	@echo "Installing to environment: $(ENV) (namespace: $(NAMESPACE))"
	helm install $(RELEASE_NAME) $(HELM_CHART) \
		-f $(VALUES_FILE) \
		-n $(NAMESPACE) \
		--create-namespace \
		$(HELM_SET_ARGS)

# Upgrade the chart for the specified environment
helm-upgrade:
	@echo "Upgrading environment: $(ENV) (namespace: $(NAMESPACE))"
	helm upgrade $(RELEASE_NAME) $(HELM_CHART) \
		-f $(VALUES_FILE) \
		-n $(NAMESPACE) \
		$(HELM_SET_ARGS)

# Install or upgrade (idempotent deploy)
helm-deploy:
	@echo "Deploying to environment: $(ENV) (namespace: $(NAMESPACE))"
	helm upgrade --install $(RELEASE_NAME) $(HELM_CHART) \
		-f $(VALUES_FILE) \
		-n $(NAMESPACE) \
		--create-namespace \
		$(HELM_SET_ARGS)

# Uninstall the chart for the specified environment
helm-uninstall:
	@echo "Uninstalling from environment: $(ENV) (namespace: $(NAMESPACE))"
	helm uninstall $(RELEASE_NAME) -n $(NAMESPACE)

# Show the status of the release
helm-status:
	helm status $(RELEASE_NAME) -n $(NAMESPACE)

# Show what would change in an upgrade
helm-diff:
	helm diff upgrade $(RELEASE_NAME) $(HELM_CHART) \
		-f $(VALUES_FILE) \
		-n $(NAMESPACE) \
		$(HELM_SET_ARGS)

# =============================================================================
# Convenience targets for specific environments
# =============================================================================

deploy-prod:
	@if [ "$(VERSION)" = "latest" ]; then echo "ERROR: VERSION is required for deploy-prod (example: VERSION=$$(git rev-parse HEAD))" >&2; exit 1; fi
	$(MAKE) helm-deploy ENV=prod VERSION=$(VERSION)

deploy-demo:
	@if [ "$(VERSION)" = "latest" ]; then echo "ERROR: VERSION is required for deploy-demo (example: VERSION=$$(git rev-parse HEAD))" >&2; exit 1; fi
	$(MAKE) helm-deploy ENV=demo VERSION=$(VERSION)

deploy-dev:
	$(MAKE) helm-deploy ENV=dev VERSION=$(VERSION)

deploy-all: deploy-prod deploy-demo deploy-dev

# =============================================================================
# Secret management helpers
# =============================================================================

# Copy secrets from prod to demo namespace
copy-secrets-demo:
	@echo "Copying secrets to fish-fry-orders-demo namespace..."
	kubectl get secret postgres -n fish-fry-orders -o yaml | \
		sed 's/namespace: fish-fry-orders/namespace: fish-fry-orders-demo/' | \
		sed '/resourceVersion/d' | sed '/uid/d' | sed '/creationTimestamp/d' | \
		kubectl apply -f -
	kubectl get secret regcred -n fish-fry-orders -o yaml | \
		sed 's/namespace: fish-fry-orders/namespace: fish-fry-orders-demo/' | \
		sed '/resourceVersion/d' | sed '/uid/d' | sed '/creationTimestamp/d' | \
		kubectl apply -f -
	kubectl get secret user-auth -n fish-fry-orders -o yaml | \
		sed 's/namespace: fish-fry-orders/namespace: fish-fry-orders-demo/' | \
		sed '/resourceVersion/d' | sed '/uid/d' | sed '/creationTimestamp/d' | \
		kubectl apply -f -

# Copy secrets from prod to dev namespace
copy-secrets-dev:
	@echo "Copying secrets to fish-fry-orders-dev namespace..."
	kubectl get secret postgres -n fish-fry-orders -o yaml | \
		sed 's/namespace: fish-fry-orders/namespace: fish-fry-orders-dev/' | \
		sed '/resourceVersion/d' | sed '/uid/d' | sed '/creationTimestamp/d' | \
		kubectl apply -f -
	kubectl get secret regcred -n fish-fry-orders -o yaml | \
		sed 's/namespace: fish-fry-orders/namespace: fish-fry-orders-dev/' | \
		sed '/resourceVersion/d' | sed '/uid/d' | sed '/creationTimestamp/d' | \
		kubectl apply -f -
	kubectl get secret user-auth -n fish-fry-orders -o yaml | \
		sed 's/namespace: fish-fry-orders/namespace: fish-fry-orders-dev/' | \
		sed '/resourceVersion/d' | sed '/uid/d' | sed '/creationTimestamp/d' | \
		kubectl apply -f -

# Setup namespaces and secrets for all environments
setup-environments:
	kubectl create namespace fish-fry-orders-demo --dry-run=client -o yaml | kubectl apply -f -
	kubectl create namespace fish-fry-orders-dev --dry-run=client -o yaml | kubectl apply -f -
	$(MAKE) copy-secrets-demo
	$(MAKE) copy-secrets-dev

# =============================================================================
# Local deployment (non-K8s)
# =============================================================================

deploy: clean backend
	@echo "Creating deployment directory..."
	@mkdir -p _output
	
	# Copy config
	@cp config.yaml _output

run: deploy
	cd _output && ./server
