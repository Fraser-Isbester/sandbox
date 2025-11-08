"""Backend implementations for distributed leases."""

from .file import FileLease
from .redis import RedisLease

try:
    from .k8s import K8sLease
except ImportError:
    # Kubernetes client is optional
    K8sLease = None

__all__ = ["FileLease", "RedisLease", "K8sLease"]
