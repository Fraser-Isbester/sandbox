# Distributed Consensus Demo

Basic distributed consensus in Go for Kubernetes. Only one pod prints "Doing Work..." while others print "Standing by...". When the working pod is killed, another takes over.

## Prerequisites

- Docker
- kind cluster
- kubectl

## Setup Kind Cluster

```bash
# Create kind cluster (or use existing one)
kind create cluster --name test

# Verify cluster
kubectl cluster-info
```

## Deploy and Test

```bash
# Build and deploy (uses cluster named "test" by default)
make build
make deploy

# Watch the consensus in action
make logs
```

To use a different cluster name:
```bash
make build CLUSTER_NAME=your-cluster-name
```

You should see output like:
```
[pod/consensus-deployment-xxx] Doing Work...
[pod/consensus-deployment-yyy] Standing by...
[pod/consensus-deployment-zzz] Standing by...
```

## Test Failover

In another terminal, kill the working pod:

```bash
# Find the working pod
kubectl get pods -l app=consensus

# Delete it
kubectl delete pod <working-pod-name>

# Watch logs to see another pod take over
```

## Cleanup

```bash
# Remove deployment
make clean

# Delete kind cluster
kind delete cluster --name consensus-demo
```

## How It Works

- Uses Kubernetes ConfigMap for leader election
- Leader renews lease every 5 seconds
- If leader fails to renew within 10 seconds, others can take over
- Minimal dependencies (only Kubernetes client-go)
