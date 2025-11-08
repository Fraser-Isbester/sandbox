"""Kubernetes Lease API implementation."""

from datetime import datetime, timedelta, timezone
from typing import Optional

from kubernetes import client, config
from kubernetes.client.rest import ApiException

from ..lease import Lease, LeaseConfig, LeaseNotHeldError


class K8sLease(Lease):
    """
    Kubernetes Lease API-based distributed lease.

    Uses the native Kubernetes coordination.k8s.io/v1 Lease resource
    for leader election across pods.
    """

    def __init__(
        self,
        api_client: client.CoordinationV1Api,
        namespace: str,
        config: LeaseConfig,
    ):
        """
        Create a Kubernetes Lease API-based lease.

        Args:
            api_client: Kubernetes coordination API client
            namespace: Kubernetes namespace
            config: Lease configuration
        """
        self.api = api_client
        self.namespace = namespace
        self.config = config
        self.lease_name = config.name

    @classmethod
    async def create_in_cluster(
        cls,
        namespace: str,
        config: LeaseConfig,
    ) -> "K8sLease":
        """
        Create a K8sLease using in-cluster configuration.

        This is the typical way to create a lease when running inside a pod.

        Args:
            namespace: Kubernetes namespace
            config: Lease configuration

        Returns:
            K8sLease instance
        """
        # Load in-cluster config
        k8s_config.load_incluster_config()
        api_client = client.CoordinationV1Api()

        return cls(api_client, namespace, config)

    @classmethod
    async def create_from_kubeconfig(
        cls,
        namespace: str,
        config: LeaseConfig,
        kubeconfig_path: Optional[str] = None,
    ) -> "K8sLease":
        """
        Create a K8sLease using kubeconfig file.

        Useful for local development.

        Args:
            namespace: Kubernetes namespace
            config: Lease configuration
            kubeconfig_path: Path to kubeconfig file (None for default)

        Returns:
            K8sLease instance
        """
        if kubeconfig_path:
            k8s_config.load_kube_config(config_file=kubeconfig_path)
        else:
            k8s_config.load_kube_config()

        api_client = client.CoordinationV1Api()

        return cls(api_client, namespace, config)

    async def acquire(self, holder_id: str, duration: Optional[float] = None) -> bool:
        """Acquire the lease."""
        if duration is None:
            duration = self.config.default_duration

        try:
            # Try to get existing lease
            existing = self.api.read_namespaced_lease(
                name=self.lease_name,
                namespace=self.namespace,
            )

            # Check if we can acquire it
            now = datetime.now(timezone.utc)

            if existing.spec.holder_identity:
                if existing.spec.holder_identity == holder_id:
                    # We already hold it, renew it
                    await self._update_lease(existing, holder_id, duration)
                    return True

                # Check if expired
                if existing.spec.renew_time:
                    expire_time = existing.spec.renew_time + timedelta(
                        seconds=existing.spec.lease_duration_seconds
                    )
                    if now < expire_time:
                        # Still valid, held by another
                        return False

            # Lease is available (expired or no holder), try to acquire
            await self._update_lease(existing, holder_id, duration)
            return True

        except ApiException as e:
            if e.status == 404:
                # Lease doesn't exist, create it
                await self._create_lease(holder_id, duration)
                return True
            raise

    async def renew(self, holder_id: str, duration: Optional[float] = None) -> None:
        """Renew the lease."""
        if duration is None:
            duration = self.config.default_duration

        try:
            existing = self.api.read_namespaced_lease(
                name=self.lease_name,
                namespace=self.namespace,
            )

            # Verify we're the holder
            if (
                not existing.spec.holder_identity
                or existing.spec.holder_identity != holder_id
            ):
                raise LeaseNotHeldError(f"Lease not held by {holder_id}")

            # Check if expired
            now = datetime.now(timezone.utc)
            if existing.spec.renew_time:
                expire_time = existing.spec.renew_time + timedelta(
                    seconds=existing.spec.lease_duration_seconds
                )
                if now > expire_time:
                    raise LeaseNotHeldError("Lease has expired")

            await self._update_lease(existing, holder_id, duration)

        except ApiException as e:
            if e.status == 404:
                raise LeaseNotHeldError(f"Lease not found")
            raise

    async def release(self, holder_id: str) -> None:
        """Release the lease."""
        try:
            existing = self.api.read_namespaced_lease(
                name=self.lease_name,
                namespace=self.namespace,
            )

            # Verify we're the holder
            if (
                not existing.spec.holder_identity
                or existing.spec.holder_identity != holder_id
            ):
                raise LeaseNotHeldError(f"Lease not held by {holder_id}")

            # Delete the lease
            self.api.delete_namespaced_lease(
                name=self.lease_name,
                namespace=self.namespace,
            )

        except ApiException as e:
            if e.status == 404:
                return  # Already released
            raise

    def is_held(self, holder_id: str) -> bool:
        """Check if lease is held by the specified holder."""
        try:
            import asyncio

            loop = asyncio.get_event_loop()
            if loop.is_running():
                # Can't do sync call in running loop
                return False
            holder = loop.run_until_complete(self.get_holder())
            return holder == holder_id
        except Exception:
            return False

    async def get_holder(self) -> Optional[str]:
        """Get the current holder's ID."""
        try:
            lease = self.api.read_namespaced_lease(
                name=self.lease_name,
                namespace=self.namespace,
            )

            # Check if expired
            if lease.spec.renew_time and lease.spec.lease_duration_seconds:
                now = datetime.now(timezone.utc)
                expire_time = lease.spec.renew_time + timedelta(
                    seconds=lease.spec.lease_duration_seconds
                )
                if now > expire_time:
                    return None

            return lease.spec.holder_identity

        except ApiException as e:
            if e.status == 404:
                return None
            raise

    async def _create_lease(self, holder_id: str, duration: float) -> None:
        """Create a new lease resource."""
        now = datetime.now(timezone.utc)

        lease = client.V1Lease(
            metadata=client.V1ObjectMeta(
                name=self.lease_name,
                namespace=self.namespace,
            ),
            spec=client.V1LeaseSpec(
                holder_identity=holder_id,
                lease_duration_seconds=int(duration),
                acquire_time=now,
                renew_time=now,
            ),
        )

        self.api.create_namespaced_lease(
            namespace=self.namespace,
            body=lease,
        )

    async def _update_lease(
        self,
        lease: client.V1Lease,
        holder_id: str,
        duration: float,
    ) -> None:
        """Update an existing lease."""
        now = datetime.now(timezone.utc)

        lease.spec.holder_identity = holder_id
        lease.spec.lease_duration_seconds = int(duration)
        lease.spec.renew_time = now

        # Set acquire time if not already set
        if not lease.spec.acquire_time:
            lease.spec.acquire_time = now

        self.api.replace_namespaced_lease(
            name=self.lease_name,
            namespace=self.namespace,
            body=lease,
        )
