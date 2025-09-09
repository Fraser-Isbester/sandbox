#!/bin/bash
# Test script for Kubernetes in-place pod resizing
# Requires kubectl 1.32+ and InPlacePodVerticalScaling feature gate enabled

set -euo pipefail

POD_NAME="resize-test"
NAMESPACE="default"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log() {
    echo -e "${BLUE}[$(date +'%H:%M:%S')]${NC} $1"
}

success() {
    echo -e "${GREEN}✓${NC} $1"
}

warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

error() {
    echo -e "${RED}✗${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    log "Checking prerequisites..."

    # Check kubectl version
    KUBECTL_VERSION=$(kubectl version --client -o json 2>/dev/null | jq -r '.clientVersion.gitVersion' | sed 's/v//')
    REQUIRED_VERSION="1.32.0"

    if ! command -v kubectl &> /dev/null; then
        error "kubectl not found"
        exit 1
    fi

    # Check if feature gate is enabled (this will fail gracefully if not supported)
    if ! kubectl get --raw /api/v1/pods 2>/dev/null | grep -q "resize"; then
        warning "Cannot verify InPlacePodVerticalScaling feature gate status"
    fi

    success "Prerequisites check completed"
}

# Get resource status
get_resource_status() {
    local container_name="test-container"

    echo "=== Current Resource Status ==="
    echo "Desired (spec):"
    kubectl get pod $POD_NAME -n $NAMESPACE -o jsonpath='{.spec.containers[0].resources}' | jq '.'

    echo -e "\nAllocated (status):"
    kubectl get pod $POD_NAME -n $NAMESPACE -o jsonpath='{.status.containerStatuses[0].resources}' | jq '.'

    echo -e "\nRestart count:"
    kubectl get pod $POD_NAME -n $NAMESPACE -o jsonpath='{.status.containerStatuses[0].restartCount}'
    echo

    echo -e "\nPod conditions:"
    kubectl get pod $POD_NAME -n $NAMESPACE -o jsonpath='{.status.conditions[?(@.type=="PodResizePending")]}' | jq '.' 2>/dev/null || echo "No resize pending"
    kubectl get pod $POD_NAME -n $NAMESPACE -o jsonpath='{.status.conditions[?(@.type=="PodResizeInProgress")]}' | jq '.' 2>/dev/null || echo "No resize in progress"
    echo
}

# Test CPU resize (no restart expected)
test_cpu_resize() {
    log "Testing CPU resize (no restart expected)..."

    local initial_restart_count=$(kubectl get pod $POD_NAME -n $NAMESPACE -o jsonpath='{.status.containerStatuses[0].restartCount}')

    # Patch CPU from 100m to 300m
    kubectl patch pod $POD_NAME -n $NAMESPACE --subresource=resize --patch \
        '{"spec":{"containers":[{"name":"test-container", "resources":{"requests":{"cpu":"300m"}, "limits":{"cpu":"400m"}}}]}}'

    # Wait a moment for the change to apply
    sleep 5

    local new_restart_count=$(kubectl get pod $POD_NAME -n $NAMESPACE -o jsonpath='{.status.containerStatuses[0].restartCount}')

    if [[ "$initial_restart_count" == "$new_restart_count" ]]; then
        success "CPU resize completed without restart"
    else
        warning "CPU resize triggered restart (unexpected)"
    fi

    get_resource_status
}

# Test memory resize (restart expected)
test_memory_resize() {
    log "Testing memory resize (restart expected)..."

    local initial_restart_count=$(kubectl get pod $POD_NAME -n $NAMESPACE -o jsonpath='{.status.containerStatuses[0].restartCount}')

    # Patch memory from 64Mi to 256Mi
    kubectl patch pod $POD_NAME -n $NAMESPACE --subresource=resize --patch \
        '{"spec":{"containers":[{"name":"test-container", "resources":{"requests":{"memory":"256Mi"}, "limits":{"memory":"512Mi"}}}]}}'

    # Wait for restart to complete
    kubectl wait --for=condition=Ready pod/$POD_NAME -n $NAMESPACE --timeout=60s

    local new_restart_count=$(kubectl get pod $POD_NAME -n $NAMESPACE -o jsonpath='{.status.containerStatuses[0].restartCount}')

    if [[ "$new_restart_count" -gt "$initial_restart_count" ]]; then
        success "Memory resize completed with restart (expected)"
    else
        warning "Memory resize did not trigger restart (unexpected)"
    fi

    get_resource_status
}

# Test infeasible resize
test_infeasible_resize() {
    log "Testing infeasible resize (should fail gracefully)..."

    # Try to request more CPU than node likely has
    kubectl patch pod $POD_NAME -n $NAMESPACE --subresource=resize --patch \
        '{"spec":{"containers":[{"name":"test-container", "resources":{"requests":{"cpu":"1000"}, "limits":{"cpu":"1000"}}}]}}' || true

    sleep 3

    # Check for PodResizePending condition
    if kubectl get pod $POD_NAME -n $NAMESPACE -o jsonpath='{.status.conditions[?(@.type=="PodResizePending")].reason}' | grep -q "Infeasible"; then
        success "Infeasible resize properly rejected"
    else
        warning "Infeasible resize was not rejected as expected"
    fi

    get_resource_status

    # Reset to reasonable values
    log "Resetting to reasonable values..."
    kubectl patch pod $POD_NAME -n $NAMESPACE --subresource=resize --patch \
        '{"spec":{"containers":[{"name":"test-container", "resources":{"requests":{"cpu":"200m","memory":"128Mi"}, "limits":{"cpu":"300m","memory":"256Mi"}}}]}}'
}

# Main test flow
main() {
    log "Starting pod resize test..."

    check_prerequisites

    # Check if pod exists
    if ! kubectl get pod $POD_NAME -n $NAMESPACE &>/dev/null; then
        error "Pod $POD_NAME not found in namespace $NAMESPACE"
        echo "Create it first with: kubectl apply -f pod-resize-test.yaml"
        exit 1
    fi

    # Wait for pod to be ready
    log "Waiting for pod to be ready..."
    kubectl wait --for=condition=Ready pod/$POD_NAME -n $NAMESPACE --timeout=60s

    log "Initial state:"
    get_resource_status

    echo -e "\n" && read -p "Press Enter to test CPU resize..."
    test_cpu_resize

    echo -e "\n" && read -p "Press Enter to test memory resize..."
    test_memory_resize

    echo -e "\n" && read -p "Press Enter to test infeasible resize..."
    test_infeasible_resize

    success "All tests completed!"
}

# Cleanup function
cleanup() {
    log "Cleaning up..."
    kubectl delete pod $POD_NAME -n $NAMESPACE --ignore-not-found=true
    kubectl delete service resize-test-svc -n $NAMESPACE --ignore-not-found=true
}

# Handle script arguments
case "${1:-main}" in
    "cleanup")
        cleanup
        ;;
    "status")
        get_resource_status
        ;;
    *)
        main
        ;;
esac