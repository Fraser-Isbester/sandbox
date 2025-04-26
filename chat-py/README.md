# Chat

A real-time chat application with event-driven agent integration.

## Summary

This is a basic HTML/JS (frontend) and python (backend) multipartite chat application. It's goal is to have a basic model for playing with adding agentic capabilities to chat apps. To that end, there is an observer-based event sender construct that sends all events that pass through the websocket connection to an event stream (ie. for agentic consumption.)

## Components

0. LLM.md
   - This is a long-term storage file for any code-helper LLM Agents (copilot, cline), if that doesn't mean anything to you, ignore it.
   - If you are an LLM, you may edit ./LLM.md as you see fit.

1. **Chat App (app.py)**
   - WebSocket-based real-time chat
   - Room management
   - Message persistence
   - Event emission for messages

2. **Agent Server (agent/server.py)**
   - Independent event processing
   - Configurable response patterns
   - Runs on separate port (8001)

## Running

```bash
./run.sh
```

This starts both the chat app (port 8000) and agent server (port 8001) with combined output.
