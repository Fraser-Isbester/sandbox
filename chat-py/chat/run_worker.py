import asyncio

from temporalio.client import Client
from temporalio.worker import Worker

from chat.activities import say_hello
from chat.workflows import SayHello


async def main():
    client = await Client.connect("localhost:7233", namespace="default")
    # Run the worker
    worker = Worker(
        client, task_queue="hello-task-queue", workflows=[SayHello], activities=[say_hello]
    )
    await worker.run()


if __name__ == "__main__":
    asyncio.run(main())
