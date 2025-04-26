# Contributing to the Chat Application

## Architecture

```
chat-py/
├── app.py              # Main chat app - handles messages and emits events
├── agent/
│   └── server.py      # Independent agent server - processes events
├── static/            # Static assets
└── templates/         # HTML templates
    ├── index.html     # Chat interface
    └── rooms.html     # Room selection
```

## Key Components

### 1. Main Application (app.py)

- WebSocket-based chat with room support
- SQLite for message persistence
- Fire-and-forget event emission
- No knowledge of agent or event processing

### 2. Agent System (agent/server.py)

- Independent event processing server
- Receives events via HTTP POST to `/events`
- Configurable response patterns
- Maintains separation of concerns

## Development Guidelines

### 1. Event-Driven Architecture

The system follows a strict event-driven architecture:
- Chat app emits events without waiting for responses
- Agent server processes events independently
- No direct coupling between components

### 2. Adding Features

Chat App:
- Focus on chat functionality only
- Emit events for relevant actions
- Don't handle agent-specific logic

Agent Server:
- Process events independently
- Implement response patterns
- Handle agent-specific functionality

### 3. Best Practices

- Keep components decoupled
- Use fire-and-forget event emission
- Handle failures gracefully
- Log errors appropriately
