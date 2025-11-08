"""Example using Kubernetes Lease API."""

import asyncio
import os
import sys

from leasing import LeaseConfig
from leasing.backends import K8sLease


async def main():
    """Main example function."""
    # Get namespace and pod name from environment (typically set by k8s)
    namespace = os.getenv("NAMESPACE", "default")
    pod_name = os.getenv("POD_NAME", f"local-{os.getpid()}")

    # Create a lease configuration
    config = LeaseConfig(name="my-worker-lease", default_duration=15.0)

    # Create a Kubernetes Lease API-based lease
    # This will use in-cluster configuration when running in a pod
    try:
        lease = await K8sLease.create_in_cluster(namespace, config)
    except Exception:
        # Fall back to kubeconfig for local development
        print("Not running in cluster, using kubeconfig...")
        lease = await K8sLease.create_from_kubeconfig(namespace, config)

    # Try to acquire the lease
    print(f"Pod {pod_name} attempting to acquire lease...")
    acquired = await lease.acquire(pod_name, duration=30.0)

    if acquired:
        print(f"✓ Pod {pod_name} acquired the lease!")

        try:
            # Do work while holding the lease
            print("Doing work as leader...")
            await do_work(lease, pod_name)
        finally:
            # Release the lease when done
            print("Releasing lease...")
            await lease.release(pod_name)
            print("✓ Lease released")
    else:
        print("✗ Lease is held by another pod")

        # Check who holds it
        holder = await lease.get_holder()
        if holder:
            print(f"Current leader: {holder}")

        print("Standing by as follower...")


async def do_work(lease: K8sLease, holder_id: str):
    """Simulate work while holding the lease, with periodic renewals."""
    work_duration = 20  # seconds
    renew_interval = 5  # seconds

    end_time = asyncio.get_event_loop().time() + work_duration

    while asyncio.get_event_loop().time() < end_time:
        # Do some work
        await asyncio.sleep(renew_interval)

        # Renew the lease
        try:
            await lease.renew(holder_id, duration=30.0)
            print("✓ Lease renewed, still leading")
        except Exception as e:
            print(f"Failed to renew lease: {e}")
            break

    print("Work completed!")


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        print("\nShutting down...")
        sys.exit(0)
