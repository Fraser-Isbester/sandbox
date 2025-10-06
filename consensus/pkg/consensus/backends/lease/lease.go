package lease

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	// ErrK8sConnection indicates failure to connect to Kubernetes
	ErrK8sConnection = errors.New("failed to connect to kubernetes")
	// ErrInvalidConfig indicates invalid configuration
	ErrInvalidConfig = errors.New("invalid configuration")
)

// Backend implements consensus.Backend using Kubernetes Lease objects.
type Backend struct {
	client    kubernetes.Interface
	namespace string
	name      string
}

// NewBackend creates a new Kubernetes Lease backend.
func NewBackend(client kubernetes.Interface, namespace, name string) *Backend {
	return &Backend{
		client:    client,
		namespace: namespace,
		name:      name,
	}
}

// NewFromEnv creates a Lease backend using in-cluster Kubernetes config.
// leaseName: name of the Lease object to use for coordination
// Environment variables:
//
//	POD_NAMESPACE - namespace for lease object (default: "default")
func NewFromEnv(leaseName string) (*Backend, error) {
	if leaseName == "" {
		return nil, fmt.Errorf("%w: leaseName cannot be empty", ErrInvalidConfig)
	}

	namespace := os.Getenv("POD_NAMESPACE")
	if namespace == "" {
		namespace = "default"
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrK8sConnection, err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create clientset: %v", ErrK8sConnection, err)
	}

	return NewBackend(clientset, namespace, leaseName), nil
}

// TryAcquire attempts to acquire or renew leadership.
func (b *Backend) TryAcquire(ctx context.Context, identity string, leaseDuration time.Duration) (bool, error) {
	leaseClient := b.client.CoordinationV1().Leases(b.namespace)

	lease, err := leaseClient.Get(ctx, b.name, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return false, fmt.Errorf("failed to get lease: %w", err)
		}

		// Lease doesn't exist - create it
		lease = &coordinationv1.Lease{
			ObjectMeta: metav1.ObjectMeta{
				Name:      b.name,
				Namespace: b.namespace,
			},
			Spec: coordinationv1.LeaseSpec{
				HolderIdentity:       &identity,
				LeaseDurationSeconds: ptr(int32(leaseDuration.Seconds())),
				AcquireTime:          &metav1.MicroTime{Time: time.Now()},
				RenewTime:            &metav1.MicroTime{Time: time.Now()},
			},
		}

		_, err = leaseClient.Create(ctx, lease, metav1.CreateOptions{})
		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				// Race condition - someone else created it
				return false, nil
			}
			return false, fmt.Errorf("failed to create lease: %w", err)
		}

		return true, nil
	}

	// Lease exists - check if we can acquire it
	now := time.Now()

	// If we're already the holder, renew it
	if lease.Spec.HolderIdentity != nil && *lease.Spec.HolderIdentity == identity {
		lease.Spec.RenewTime = &metav1.MicroTime{Time: now}
		lease.Spec.LeaseDurationSeconds = ptr(int32(leaseDuration.Seconds()))
		_, err = leaseClient.Update(ctx, lease, metav1.UpdateOptions{})
		if err != nil {
			return false, fmt.Errorf("failed to renew lease: %w", err)
		}
		return true, nil
	}

	// Different holder - check if lease has expired
	if lease.Spec.RenewTime != nil && lease.Spec.LeaseDurationSeconds != nil {
		elapsed := now.Sub(lease.Spec.RenewTime.Time)
		ttl := time.Duration(*lease.Spec.LeaseDurationSeconds) * time.Second
		if elapsed < ttl {
			// Lease is still valid
			return false, nil
		}
	}

	// Lease has expired - take it over
	lease.Spec.HolderIdentity = &identity
	lease.Spec.AcquireTime = &metav1.MicroTime{Time: now}
	lease.Spec.RenewTime = &metav1.MicroTime{Time: now}
	lease.Spec.LeaseDurationSeconds = ptr(int32(leaseDuration.Seconds()))

	_, err = leaseClient.Update(ctx, lease, metav1.UpdateOptions{})
	if err != nil {
		if apierrors.IsConflict(err) {
			// Someone else updated it
			return false, nil
		}
		return false, fmt.Errorf("failed to acquire expired lease: %w", err)
	}

	return true, nil
}

// Renew extends the current leader's lease.
func (b *Backend) Renew(ctx context.Context, identity string, leaseDuration time.Duration) error {
	leaseClient := b.client.CoordinationV1().Leases(b.namespace)

	lease, err := leaseClient.Get(ctx, b.name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get lease for renewal: %w", err)
	}

	// Verify we're the holder
	if lease.Spec.HolderIdentity == nil || *lease.Spec.HolderIdentity != identity {
		return fmt.Errorf("not the lease holder")
	}

	// Update renewal time and duration
	lease.Spec.RenewTime = &metav1.MicroTime{Time: time.Now()}
	lease.Spec.LeaseDurationSeconds = ptr(int32(leaseDuration.Seconds()))

	_, err = leaseClient.Update(ctx, lease, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update lease: %w", err)
	}

	return nil
}

// Release explicitly gives up leadership.
func (b *Backend) Release(ctx context.Context, identity string) error {
	leaseClient := b.client.CoordinationV1().Leases(b.namespace)

	lease, err := leaseClient.Get(ctx, b.name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Lease doesn't exist - nothing to release
			return nil
		}
		return fmt.Errorf("failed to get lease for release: %w", err)
	}

	// Only release if we're the holder
	if lease.Spec.HolderIdentity != nil && *lease.Spec.HolderIdentity == identity {
		lease.Spec.HolderIdentity = nil
		_, err = leaseClient.Update(ctx, lease, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to release lease: %w", err)
		}
	}

	return nil
}

// ptr is a helper to get a pointer to a value.
func ptr[T any](v T) *T {
	return &v
}
