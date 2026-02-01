.PHONY: all build test clean proto backend docker deploy helm-lint helm-template frontend

# Container registry - override with: make docker-push REGISTRY=myregistry.io
REGISTRY ?= git.nonahob.net
IMAGE_NAME_API ?= jacob/fish-fry-orders-api
IMAGE_NAME_FRONTEND ?= jacob/fish-fry-orders-frontend
VERSION ?= latest
NAMESPACE ?= fish-fry-orders

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

# Helm chart operations
helm-lint:
	helm lint helm/fish-fry-orders

helm-template:
	helm template fish-fry-orders helm/fish-fry-orders -n $(NAMESPACE)

helm-install:
	helm install fish-fry-orders helm/fish-fry-orders -n $(NAMESPACE) --create-namespace

helm-upgrade:
	helm upgrade fish-fry-orders helm/fish-fry-orders -n $(NAMESPACE)

helm-uninstall:
	helm uninstall fish-fry-orders -n $(NAMESPACE)

# Create deployment directory (for local/VM deployment without K8s)
deploy: clean backend
	@echo "Creating deployment directory..."
	@mkdir -p _output
	
	# Copy config
	@cp config.yaml _output

run: deploy
	cd _output && ./server
