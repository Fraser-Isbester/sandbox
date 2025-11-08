"""Example using Redis-based lease."""

import asyncio
import os
import sys

from leasing import LeaseConfig
from leasing.backends import RedisLease


async def main():
    """Main example function."""
    # Get Redis URL from environment or use default
    redis_url = os.getenv("REDIS_URL", "redis://localhost:6379")

    # Create a lease configuration
    config = LeaseConfig(name="my-worker-lease", default_duration=10.0)

    # Create a Redis-based lease
    lease = await RedisLease.create(redis_url, config)

    # Get our identity
    holder_id = os.getenv("WORKER_ID", f"worker-{os.getpid()}")

    try:
        # Try to acquire the lease
        print(f"Worker {holder_id} attempting to acquire lease...")
        acquired = await lease.acquire(holder_id, duration=30.0)

        if acquired:
            print(f"✓ Worker {holder_id} acquired the lease!")

            try:
                # Do work while holding the lease
                print("Doing work...")
                await do_work(lease, holder_id)
            finally:
                # Release the lease when done
                print("Releasing lease...")
                await lease.release(holder_id)
                print("✓ Lease released")
        else:
            print("✗ Lease is held by another worker")

            # Check who holds it
            holder = await lease.get_holder()
            if holder:
                print(f"Current holder: {holder}")
    finally:
        # Close Redis connection
        await lease.close()


async def do_work(lease: RedisLease, holder_id: str):
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
            print("✓ Lease renewed")
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
