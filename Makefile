.PHONY: all build test clean proto backend docker deploy helm-lint helm-template frontend build-chart

# Container registry - override with: make docker-push REGISTRY=myregistry.io
REGISTRY ?= git.nonahob.net
IMAGE_NAME_API ?= jacob/fish-fry-orders-api
IMAGE_NAME_FRONTEND ?= jacob/fish-fry-orders-frontend
VERSION ?= latest

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

# Build chart output (gitignored via _output/)
BUILD_CHART := _output/helm/fish-fry-orders
BUILD_VALUES_FILE := $(BUILD_CHART)/configurations/$(CONFIG_DIR)/values.yaml

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
# Usage: make helm-deploy ENV=demo VERSION=v1.2.0
# =============================================================================

# Build a temporary copy of the chart with __VERSION__ replaced
build-chart:
	@echo "Building chart with VERSION=$(VERSION) for ENV=$(ENV)..."
	@rm -rf $(BUILD_CHART)
	@mkdir -p _output/helm
	@cp -r $(HELM_CHART) $(BUILD_CHART)
	@sed -i 's/__VERSION__/$(VERSION)/g' $(BUILD_VALUES_FILE)

helm-lint: build-chart
	helm lint $(BUILD_CHART)

# Template the chart with environment-specific values
helm-template: build-chart
	@echo "Templating for environment: $(ENV) (namespace: $(NAMESPACE), version: $(VERSION))"
	helm template $(RELEASE_NAME) $(BUILD_CHART) \
		-f $(BUILD_VALUES_FILE) \
		-n $(NAMESPACE)

# Install the chart for the specified environment
helm-install: build-chart
	@echo "Installing to environment: $(ENV) (namespace: $(NAMESPACE), version: $(VERSION))"
	helm install $(RELEASE_NAME) $(BUILD_CHART) \
		-f $(BUILD_VALUES_FILE) \
		-n $(NAMESPACE) \
		--create-namespace

# Upgrade the chart for the specified environment
helm-upgrade: build-chart
	@echo "Upgrading environment: $(ENV) (namespace: $(NAMESPACE), version: $(VERSION))"
	helm upgrade $(RELEASE_NAME) $(BUILD_CHART) \
		-f $(BUILD_VALUES_FILE) \
		-n $(NAMESPACE)

# Install or upgrade (idempotent deploy)
helm-deploy: build-chart
	@echo "Deploying to environment: $(ENV) (namespace: $(NAMESPACE), version: $(VERSION))"
	helm upgrade --install $(RELEASE_NAME) $(BUILD_CHART) \
		-f $(BUILD_VALUES_FILE) \
		-n $(NAMESPACE) \
		--create-namespace

# Uninstall the chart for the specified environment
helm-uninstall:
	@echo "Uninstalling from environment: $(ENV) (namespace: $(NAMESPACE))"
	helm uninstall $(RELEASE_NAME) -n $(NAMESPACE)

# Show the status of the release
helm-status:
	helm status $(RELEASE_NAME) -n $(NAMESPACE)

# Show what would change in an upgrade
helm-diff: build-chart
	helm diff upgrade $(RELEASE_NAME) $(BUILD_CHART) \
		-f $(BUILD_VALUES_FILE) \
		-n $(NAMESPACE)

# =============================================================================
# Convenience targets for specific environments
# =============================================================================

deploy-prod:
	$(MAKE) helm-deploy ENV=prod

deploy-demo:
	$(MAKE) helm-deploy ENV=demo

deploy-dev:
	$(MAKE) helm-deploy ENV=dev

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
