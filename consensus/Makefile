.PHONY: build deploy logs clean

IMAGE_NAME := consensus:latest
CLUSTER_NAME ?= test

# Build and load into kind
build:
	docker build -t $(IMAGE_NAME) .
	kind load docker-image $(IMAGE_NAME) --name $(CLUSTER_NAME)

# Deploy to Kubernetes
deploy:
	kubectl apply -f k8s/deployment.yaml

# Watch logs from all pods
logs:
	kubectl -n consensus logs -l app=consensus -f --prefix=true

# Clean up all resources
clean:
	kubectl delete -f k8s/deployment.yaml --ignore-not-found=true
	kubectl delete configmap consensus-leader --ignore-not-found=true
