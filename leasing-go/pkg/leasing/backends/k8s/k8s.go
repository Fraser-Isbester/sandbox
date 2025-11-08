package k8s

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Fraser-Isbester/sandbox/leasing-go/pkg/leasing"
	coordinationv1 "k8s.io/api/coordination/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	ErrLeaseHeldByAnother = errors.New("lease is held by another process")
	ErrNotLeaseHolder     = errors.New("not the current lease holder")
)

// K8sLease implements a Kubernetes Lease API-based distributed lease.
// This uses the native Kubernetes coordination.k8s.io/v1 Lease resource.
type K8sLease struct {
	client    kubernetes.Interface
	config    leasing.Config
	namespace string
	leaseName string
}

// New creates a new Kubernetes Lease API-based lease.
// The client should already be configured with appropriate credentials.
func New(client kubernetes.Interface, namespace string, config leasing.Config) (*K8sLease, error) {
	if client == nil {
		return nil, errors.New("kubernetes client is required")
	}
	if namespace == "" {
		return nil, errors.New("namespace is required")
	}
	if config.Name == "" {
		return nil, errors.New("lease name is required")
	}

	return &K8sLease{
		client:    client,
		config:    config,
		namespace: namespace,
		leaseName: config.Name,
	}, nil
}

// NewInCluster creates a K8sLease using in-cluster configuration.
// This is the typical way to create a lease when running inside a pod.
func NewInCluster(namespace string, config leasing.Config) (*K8sLease, error) {
	clusterConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return New(clientset, namespace, config)
}

// Acquire attempts to acquire the lease.
func (k *K8sLease) Acquire(ctx context.Context, holderID string, duration time.Duration) (bool, error) {
	leaseClient := k.client.CoordinationV1().Leases(k.namespace)

	// Try to get existing lease
	existingLease, err := leaseClient.Get(ctx, k.leaseName, metav1.GetOptions{})
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return false, fmt.Errorf("failed to get lease: %w", err)
		}

		// Lease doesn't exist, create it
		if err := k.createLease(ctx, holderID, duration); err != nil {
			return false, err
		}
		return true, nil
	}

	// Lease exists, check if we can acquire it
	now := metav1.NowMicro()

	// Check if lease is expired or we're the current holder
	if existingLease.Spec.HolderIdentity != nil {
		if *existingLease.Spec.HolderIdentity == holderID {
			// We already hold it, renew it
			return true, k.updateLease(ctx, existingLease, holderID, duration)
		}

		// Check if expired
		if existingLease.Spec.RenewTime != nil {
			expireTime := existingLease.Spec.RenewTime.Add(time.Duration(*existingLease.Spec.LeaseDurationSeconds) * time.Second)
			if now.Time.Before(expireTime) {
				// Still valid, held by another
				return false, nil
			}
		}
	}

	// Lease is available (expired or no holder), try to acquire
	if err := k.updateLease(ctx, existingLease, holderID, duration); err != nil {
		return false, err
	}

	return true, nil
}

// Renew extends the lease duration.
func (k *K8sLease) Renew(ctx context.Context, holderID string, duration time.Duration) error {
	leaseClient := k.client.CoordinationV1().Leases(k.namespace)

	existingLease, err := leaseClient.Get(ctx, k.leaseName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return ErrNotLeaseHolder
		}
		return fmt.Errorf("failed to get lease: %w", err)
	}

	// Verify we're the holder
	if existingLease.Spec.HolderIdentity == nil || *existingLease.Spec.HolderIdentity != holderID {
		return ErrNotLeaseHolder
	}

	// Check if expired
	now := metav1.NowMicro()
	if existingLease.Spec.RenewTime != nil {
		expireTime := existingLease.Spec.RenewTime.Add(time.Duration(*existingLease.Spec.LeaseDurationSeconds) * time.Second)
		if now.Time.After(expireTime) {
			return ErrNotLeaseHolder
		}
	}

	return k.updateLease(ctx, existingLease, holderID, duration)
}

// Release releases the lease.
func (k *K8sLease) Release(ctx context.Context, holderID string) error {
	leaseClient := k.client.CoordinationV1().Leases(k.namespace)

	existingLease, err := leaseClient.Get(ctx, k.leaseName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil // Already released
		}
		return fmt.Errorf("failed to get lease: %w", err)
	}

	// Verify we're the holder
	if existingLease.Spec.HolderIdentity == nil || *existingLease.Spec.HolderIdentity != holderID {
		return ErrNotLeaseHolder
	}

	// Delete the lease
	if err := leaseClient.Delete(ctx, k.leaseName, metav1.DeleteOptions{}); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to delete lease: %w", err)
	}

	return nil
}

// IsHeld checks if the lease is held by the specified holder.
func (k *K8sLease) IsHeld(holderID string) bool {
	ctx := context.Background()
	holder, err := k.GetHolder(ctx)
	if err != nil {
		return false
	}
	return holder == holderID
}

// GetHolder returns the current holder's ID.
func (k *K8sLease) GetHolder(ctx context.Context) (string, error) {
	leaseClient := k.client.CoordinationV1().Leases(k.namespace)

	lease, err := leaseClient.Get(ctx, k.leaseName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to get lease: %w", err)
	}

	// Check if expired
	if lease.Spec.RenewTime != nil && lease.Spec.LeaseDurationSeconds != nil {
		now := metav1.NowMicro()
		expireTime := lease.Spec.RenewTime.Add(time.Duration(*lease.Spec.LeaseDurationSeconds) * time.Second)
		if now.Time.After(expireTime) {
			return "", nil
		}
	}

	if lease.Spec.HolderIdentity == nil {
		return "", nil
	}

	return *lease.Spec.HolderIdentity, nil
}

// createLease creates a new lease resource.
func (k *K8sLease) createLease(ctx context.Context, holderID string, duration time.Duration) error {
	leaseClient := k.client.CoordinationV1().Leases(k.namespace)

	now := metav1.NowMicro()
	durationSeconds := int32(duration.Seconds())

	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k.leaseName,
			Namespace: k.namespace,
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       &holderID,
			LeaseDurationSeconds: &durationSeconds,
			AcquireTime:          &now,
			RenewTime:            &now,
		},
	}

	_, err := leaseClient.Create(ctx, lease, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create lease: %w", err)
	}

	return nil
}

// updateLease updates an existing lease.
func (k *K8sLease) updateLease(ctx context.Context, lease *coordinationv1.Lease, holderID string, duration time.Duration) error {
	leaseClient := k.client.CoordinationV1().Leases(k.namespace)

	now := metav1.NowMicro()
	durationSeconds := int32(duration.Seconds())

	// Update the lease spec
	lease.Spec.HolderIdentity = &holderID
	lease.Spec.LeaseDurationSeconds = &durationSeconds
	lease.Spec.RenewTime = &now

	// Set acquire time if not already set
	if lease.Spec.AcquireTime == nil {
		lease.Spec.AcquireTime = &now
	}

	_, err := leaseClient.Update(ctx, lease, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update lease: %w", err)
	}

	return nil
}
