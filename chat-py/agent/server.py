"""
filepath: ./agent/server.py
description: FastAPI event handling server for the Chat Agent.
run: uvicorn server:app --reload
"""

import logging
import os
from typing import Any, Dict, Optional, TypedDict

import httpx
import uvicorn
from fastapi import FastAPI
from pydantic import BaseModel

CHAT_APP_URL = os.getenv("CHAT_APP_URL", "http://localhost:8000")
# Static for now, will be dynamically fetched from the models module in the future
AGENT_NAME = "ChatAgent"

# Setup logging
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",  # Added logger name
    handlers=[logging.StreamHandler()],
)
logger = logging.getLogger("agent")

app = FastAPI(title="Chat Agent API")


class EventData(BaseModel):
    """Data model for incoming events"""

    event_type: str  # Type of event (e.g., "new_message", "user_joined")
    room_id: str  # ID of the chat room
    data: Dict[str, Any]  # Event payload data
    metadata: Optional[Dict[str, Any]] = None  # Optional metadata


async def send_message_to_chat(room_id: str, content: str):
    """Sends a message from the agent to the specified chat room."""

    injection_url = f"{CHAT_APP_URL}/api/inject_message/{room_id}"
    payload = {
        "sender": AGENT_NAME,
        "content": content,
        # Agent Messages are always saved to the database.
        "save_to_db": True,
    }
    try:
        async with httpx.AsyncClient() as client:
            response = await client.post(injection_url, json=payload, timeout=10.0)
            # Raise an exception for 4xx or 5xx status codes
            response.raise_for_status()
            logger.info(
                f"Successfully injected message to room '{room_id}' via {injection_url}"
            )
            return True

    except httpx.RequestError as e:
        logger.error(
            f"Could not connect to chat app at {e.request.url!r} to inject message. Error: {e}"
        )

    except httpx.HTTPStatusError as e:
        logger.error(
            f"Chat app returned error status {e.response.status_code} for {e.request.url!r}. Response: {e.response.text}"
        )
    except Exception as e:
        logger.error(
            f"An unexpected error occurred while injecting message to room '{room_id}': {e}"
        )

    return False


@app.post("/events")
async def process_event(event: EventData):
    """
    Process incoming chat events, decide on action, and potentially send a response.
    """

    logger.info(f"Received {event.event_type} event in room {event.room_id}")
    logger.debug(f"Event data: {event.data}")  # Use debug for potentially large data

    response_content = None

    if event.event_type == "new_message":
        sender = event.data.get("sender", "Unknown")
        # Avoid agent responding to its own messages
        if sender == AGENT_NAME:
            logger.info(
                f"Ignoring message from self ({AGENT_NAME}) in room {event.room_id}"
            )
        else:
            message = event.data.get("content", "").lower()
            # Simple trigger: message contains "?" and mentions "agent" or "bot"
            if "?" in message and ("agent" in message or "bot" in message):
                logger.info(
                    f"Agent trigger condition met for message in room {event.room_id}"
                )
                logger.info("Hi jae. from the agent framework.")
                # --- LLM Integration Point ---
                # In the future, you would replace this hardcoded response
                # with a call to the Gemini API.
                # Example (Conceptual - requires SDK setup, API keys etc.):
                # try:
                #     # client = genai.Client(...) setup elsewhere
                #     # chat = client.chats.create(...) or load existing context
                #     # llm_response = chat.send_message(f"User asked: {event.data.get('content')}")
                #     # response_content = llm_response.text
                # except Exception as llm_error:
                #     logger.error(f"LLM generation failed: {llm_error}")
                #     response_content = "Sorry, I encountered an error trying to process that."

                # For now, use a placeholder:
                response_content = f"Hi {sender}, I see you mentioned me and asked a question! I'm still learning how to help effectively."
                # --- End LLM Integration Point ---

    # If a response was generated, send it back to the chat app
    message_sent = False
    if response_content:
        logger.info(
            f"Attempting to send response to room {event.room_id}: '{response_content[:50]}...'"
        )
        message_sent = await send_message_to_chat(event.room_id, response_content)

    # Return status about processing and if a response *attempt* was made
    class EventResponse(TypedDict):
        event_processed: bool
        action_taken: str  # More descriptive than should_respond
        message_sent_successfully: Optional[bool]

    action = "none"
    if response_content and message_sent:
        action = "sent_response"
    elif response_content and not message_sent:
        # Clarify if generation or sending failed
        action = "response_generation_failed" if not response_content else "send_failed"

    elif event.event_type == "new_message" and sender != AGENT_NAME:
        action = "ignored_message"  # Explicitly state ignored

    else:
        action = "processed_other_event"

    return EventResponse(
        event_processed=True,
        action_taken=action,
        message_sent_successfully=message_sent if response_content else None,
    )


@app.get("/health")
async def health_check():
    """Health check endpoint"""

    class HealthResponse(TypedDict):
        status: str

    return HealthResponse(status="healthy")


if __name__ == "__main__":
    logger.info(f"Agent starting. Will connect to Chat App at: {CHAT_APP_URL}")
    uvicorn.run("server:app", host="0.0.0.0", port=8001, reload=True)
