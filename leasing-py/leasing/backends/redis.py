"""Redis-based lease implementation."""

from typing import Optional

from ..lease import Lease, LeaseConfig, LeaseNotHeldError

try:
    import redis.asyncio as aioredis
except ImportError:
    import redis as aioredis  # Fallback


class RedisLease(Lease):
    """
    Redis-based distributed lease using SET with NX and EX options.

    Provides atomic lease acquisition across multiple processes/nodes.
    Uses Lua scripts for atomic check-and-update operations.
    """

    def __init__(self, client: aioredis.Redis, config: LeaseConfig):
        """
        Create a Redis-based lease.

        Args:
            client: Redis client (should be async)
            config: Lease configuration
        """
        self.client = client
        self.config = config
        self.key = f"lease:{config.name}"

    @classmethod
    async def create(
        cls,
        redis_url: str,
        config: LeaseConfig,
        **kwargs,
    ) -> "RedisLease":
        """
        Create a RedisLease with a new Redis client.

        Args:
            redis_url: Redis connection URL (e.g., "redis://localhost:6379")
            config: Lease configuration
            **kwargs: Additional arguments for Redis client

        Returns:
            RedisLease instance
        """
        client = aioredis.from_url(redis_url, **kwargs)
        return cls(client, config)

    async def acquire(self, holder_id: str, duration: Optional[float] = None) -> bool:
        """Acquire the lease using Redis SET with NX."""
        if duration is None:
            duration = self.config.default_duration

        # Try to set the key only if it doesn't exist (NX)
        # EX sets the expiration time in seconds
        result = await self.client.set(
            self.key,
            holder_id,
            nx=True,
            ex=int(duration),
        )

        # If result is True, we acquired the lease
        return bool(result)

    async def renew(self, holder_id: str, duration: Optional[float] = None) -> None:
        """Renew the lease using a Lua script for atomic check-and-update."""
        if duration is None:
            duration = self.config.default_duration

        # Lua script to atomically check and renew
        script = """
        local current = redis.call('GET', KEYS[1])
        if current == ARGV[1] then
            redis.call('PEXPIRE', KEYS[1], ARGV[2])
            return 1
        else
            return 0
        end
        """

        result = await self.client.eval(
            script,
            1,
            self.key,
            holder_id,
            int(duration * 1000),  # Convert to milliseconds
        )

        if result == 0:
            raise LeaseNotHeldError(f"Lease not held by {holder_id}")

    async def release(self, holder_id: str) -> None:
        """Release the lease using a Lua script for atomic check-and-delete."""
        # Lua script to atomically check and delete
        script = """
        local current = redis.call('GET', KEYS[1])
        if current == ARGV[1] then
            redis.call('DEL', KEYS[1])
            return 1
        else
            return 0
        end
        """

        result = await self.client.eval(script, 1, self.key, holder_id)

        if result == 0:
            raise LeaseNotHeldError(f"Lease not held by {holder_id}")

    def is_held(self, holder_id: str) -> bool:
        """
        Check if lease is held by the specified holder.

        Note: This performs a synchronous Redis GET.
        For async contexts, use get_holder() instead.
        """
        # This is a sync method, so we can't use await
        # In practice, you'd want to use get_holder() in async code
        # This is provided for compatibility with the interface
        try:
            import asyncio

            loop = asyncio.get_event_loop()
            if loop.is_running():
                # Can't do sync call in running loop
                return False
            val = loop.run_until_complete(self.client.get(self.key))
            return val is not None and val.decode() == holder_id
        except Exception:
            return False

    async def get_holder(self) -> Optional[str]:
        """Get the current holder's ID."""
        val = await self.client.get(self.key)
        if val is None:
            return None
        return val.decode() if isinstance(val, bytes) else val

    async def close(self) -> None:
        """Close the Redis client connection."""
        await self.client.close()
