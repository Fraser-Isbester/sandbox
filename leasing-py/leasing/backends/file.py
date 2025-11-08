"""File-based lease implementation."""

import asyncio
import json
import os
from datetime import datetime, timedelta
from pathlib import Path
from typing import Optional

from ..lease import Lease, LeaseConfig, LeaseNotHeldError


class FileLease(Lease):
    """
    File-based lease using JSON files and file locking.

    Suitable for single-node scenarios or shared filesystems.
    Uses atomic file operations for concurrent access safety.
    """

    def __init__(self, config: LeaseConfig, directory: str):
        """
        Create a file-based lease.

        Args:
            config: Lease configuration
            directory: Directory to store lease files
        """
        self.config = config
        self.directory = Path(directory)
        self.file_path = self.directory / f"{config.name}.lease"
        self._lock = asyncio.Lock()
        self._current_holder: Optional[str] = None
        self._current_expiry: Optional[datetime] = None

        # Ensure directory exists
        self.directory.mkdir(parents=True, exist_ok=True)

    async def acquire(self, holder_id: str, duration: Optional[float] = None) -> bool:
        """Acquire the lease."""
        if duration is None:
            duration = self.config.default_duration

        async with self._lock:
            # Read current lease state
            current = await self._read_lease()

            now = datetime.utcnow()

            # Check if lease exists and is still valid
            if current:
                expires_at = datetime.fromisoformat(current["expires_at"])
                if now < expires_at:
                    if current["holder_id"] != holder_id:
                        return False  # Held by another process
                    # We already hold it, extend it
                    await self._write_lease(holder_id, duration)
                    return True

            # Lease is available or expired, acquire it
            await self._write_lease(holder_id, duration)
            return True

    async def renew(self, holder_id: str, duration: Optional[float] = None) -> None:
        """Renew the lease."""
        if duration is None:
            duration = self.config.default_duration

        async with self._lock:
            current = await self._read_lease()

            if not current:
                raise LeaseNotHeldError(f"Lease not held by {holder_id}")

            now = datetime.utcnow()
            expires_at = datetime.fromisoformat(current["expires_at"])

            # Verify we're the holder
            if current["holder_id"] != holder_id:
                raise LeaseNotHeldError(f"Lease held by {current['holder_id']}")

            # Check if expired
            if now > expires_at:
                raise LeaseNotHeldError("Lease has expired")

            # Renew the lease
            await self._write_lease(holder_id, duration)

    async def release(self, holder_id: str) -> None:
        """Release the lease."""
        async with self._lock:
            current = await self._read_lease()

            if not current:
                return  # Already released

            # Verify we're the holder
            if current["holder_id"] != holder_id:
                raise LeaseNotHeldError(f"Lease held by {current['holder_id']}")

            # Remove the lease file
            try:
                self.file_path.unlink()
                self._current_holder = None
                self._current_expiry = None
            except FileNotFoundError:
                pass  # Already removed

    def is_held(self, holder_id: str) -> bool:
        """Check if lease is held by the specified holder."""
        if self._current_holder != holder_id:
            return False

        if self._current_expiry is None:
            return False

        return datetime.utcnow() < self._current_expiry

    async def get_holder(self) -> Optional[str]:
        """Get the current holder's ID."""
        async with self._lock:
            current = await self._read_lease()

            if not current:
                return None

            # Check if expired
            expires_at = datetime.fromisoformat(current["expires_at"])
            if datetime.utcnow() > expires_at:
                return None

            return current["holder_id"]

    async def _read_lease(self) -> Optional[dict]:
        """Read the lease file."""
        try:
            # Run blocking I/O in executor
            loop = asyncio.get_event_loop()
            data = await loop.run_in_executor(None, self.file_path.read_text)
            return json.loads(data)
        except FileNotFoundError:
            return None
        except json.JSONDecodeError:
            return None

    async def _write_lease(self, holder_id: str, duration: float) -> None:
        """Write the lease file atomically."""
        now = datetime.utcnow()
        expires_at = now + timedelta(seconds=duration)

        data = {
            "holder_id": holder_id,
            "expires_at": expires_at.isoformat(),
            "acquired_at": now.isoformat(),
        }

        # Write atomically using temp file and rename
        tmp_path = self.file_path.with_suffix(".tmp")

        loop = asyncio.get_event_loop()
        await loop.run_in_executor(
            None, tmp_path.write_text, json.dumps(data, indent=2)
        )
        await loop.run_in_executor(None, tmp_path.replace, self.file_path)

        self._current_holder = holder_id
        self._current_expiry = expires_at
