import os
from setuptools import setup, find_packages

# Read README if it exists
readme_path = os.path.join(os.path.dirname(__file__), "..", "README.md")
long_description = ""
if os.path.exists(readme_path):
    with open(readme_path, "r", encoding="utf-8") as f:
        long_description = f.read()

setup(
    name="pubsubscheduler",
    version="0.1.0",
    packages=find_packages(),
    install_requires=[
        "google-cloud-pubsub>=2.18.0",
        "psycopg2-binary>=2.9.0",
        "croniter>=2.0.0",
    ],
    python_requires=">=3.8",
    author="Fraser Isbester",
    description="Durable message scheduling library for Google Cloud Pub/Sub",
    long_description=long_description,
    long_description_content_type="text/markdown",
)
