.PHONY: all build test clean proto backend docker deploy helm-lint helm-template

# Container registry - override with: make docker-push REGISTRY=myregistry.io
REGISTRY ?= git.nonahob.net
IMAGE_NAME ?= jacob/fish-fry-orders-v2
VERSION ?= latest

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

# Docker build
docker:
	docker build -t $(REGISTRY)/$(IMAGE_NAME):$(VERSION) .

# Docker push
docker-push: docker
	docker push $(REGISTRY)/$(IMAGE_NAME):$(VERSION)

# Development server
dev:
	go run cmd/server/main.go

# Database migrations
migrate:
	go run cmd/migrate/main.go -command up

migrate-status:
	go run cmd/migrate/main.go -command status

migrate-down:
	go run cmd/migrate/main.go -command down

# Helm chart operations
helm-lint:
	helm lint helm/fish-fry-orders-v2

helm-template:
	helm template fish-fry-orders-v2 helm/fish-fry-orders-v2 -n fish-fry-orders-v2

helm-install:
	helm install fish-fry-orders-v2 helm/fish-fry-orders-v2 -n fish-fry-orders-v2 --create-namespace

helm-upgrade:
	helm upgrade fish-fry-orders-v2 helm/fish-fry-orders-v2 -n fish-fry-orders-v2

helm-uninstall:
	helm uninstall fish-fry-orders-v2 -n fish-fry-orders-v2

# Create deployment directory (for local/VM deployment without K8s)
deploy: clean backend
	@echo "Creating deployment directory..."
	@mkdir -p _output/ui/templates
	@mkdir -p _output/ui/static
	
	# Copy templates
	@cp -r ui/templates/*.gohtml _output/ui/templates/
	
	# Copy static files
	@cp -r ui/static/* _output/ui/static/
	
	# Copy config
	@cp config.yaml _output

run: deploy
	cd _output && ./server
