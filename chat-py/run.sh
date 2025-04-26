#!/bin/bash

# Function to cleanup background processes on exit
cleanup() {
    echo "Shutting down servers..."
    kill $(jobs -p)
    exit 0
}

# Tells genai to use the vertex API instead of the Gemini API.
export GOOGLE_GENAI_USE_VERTEXAI=true

# Set up cleanup on script exit
trap cleanup EXIT

# Start both servers and combine their output
echo "Starting chat servers..."
(cd agent && python server.py) & # Run agent server from its directory
(python app.py) & # Run main app

# Wait for both background processes
wait
