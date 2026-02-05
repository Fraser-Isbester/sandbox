FROM python:3.11-slim

WORKDIR /app

# Install system dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    gcc \
    libpq-dev && \
    rm -rf /var/lib/apt/lists/*

# Copy package files
COPY python/pubsubscheduler /app/pubsubscheduler
COPY python/setup.py /app/setup.py
COPY python/examples/main.py /app/main.py

# Install Python dependencies
RUN pip install --no-cache-dir -e .

# Run the worker
CMD ["python", "main.py"]
