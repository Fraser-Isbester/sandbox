"""Core lease interface and configuration."""

from abc import ABC, abstractmethod
from dataclasses import dataclass
from typing import Optional


@dataclass
class LeaseConfig:
    """Configuration for a lease."""

    name: str
    """Unique name/key for this lease"""

    default_duration: float = 15.0
    """Default lease duration in seconds"""


class Lease(ABC):
    """
    Abstract base class for distributed leases.

    A lease allows only one holder at a time, enabling leader election
    or exclusive access patterns across multiple processes.
    """

    @abstractmethod
    async def acquire(self, holder_id: str, duration: Optional[float] = None) -> bool:
        """
        Attempt to acquire the lease.

        Args:
            holder_id: Unique identifier for this process
            duration: Lease duration in seconds (uses default if None)

        Returns:
            True if acquired, False if held by another process

        Raises:
            Exception: On backend errors
        """
        pass

    @abstractmethod
    async def renew(self, holder_id: str, duration: Optional[float] = None) -> None:
        """
        Extend the lease duration for the current holder.

        Args:
            holder_id: Unique identifier for this process
            duration: Lease duration in seconds (uses default if None)

        Raises:
            LeaseNotHeldError: If the lease is not held by holder_id
            Exception: On backend errors
        """
        pass

    @abstractmethod
    async def release(self, holder_id: str) -> None:
        """
        Release the lease.

        Args:
            holder_id: Unique identifier for this process

        Raises:
            LeaseNotHeldError: If the lease is not held by holder_id
            Exception: On backend errors
        """
        pass

    @abstractmethod
    def is_held(self, holder_id: str) -> bool:
        """
        Check if the lease is held by the specified holder.

        This is a non-blocking check of local state.

        Args:
            holder_id: Unique identifier to check

        Returns:
            True if held by holder_id, False otherwise
        """
        pass

    @abstractmethod
    async def get_holder(self) -> Optional[str]:
        """
        Get the current holder's ID.

        Returns:
            Holder ID if lease is held, None otherwise

        Raises:
            Exception: On backend errors
        """
        pass


class LeaseNotHeldError(Exception):
    """Raised when attempting to renew/release a lease not held by the caller."""

    pass


class LeaseHeldByAnotherError(Exception):
    """Raised when attempting to acquire a lease held by another process."""

    pass
