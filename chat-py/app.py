"""
filepath: ./app.py
description: FastAPI WebSocket chat application with SQLite database and event broadcasting.
run: uvicorn app:app --reload
"""

import datetime
import json
import os
import re
import sqlite3
from typing import Any, Dict, List, Optional, Set  # Added Optional

import httpx
from fastapi import (
    FastAPI,
    Form,
    HTTPException,
    Request,
    WebSocket,
    WebSocketDisconnect,
)
from fastapi.responses import HTMLResponse, RedirectResponse
from fastapi.templating import Jinja2Templates
from pydantic import BaseModel, Field

app = FastAPI(title="WebSocket Chat Application")

# Setup Jinja2 templates
templates = Jinja2Templates(directory="templates")

# Create a directory for static files if it doesn't exist
if not os.path.exists("static"):
    os.makedirs("static")

# WebSocket connection manager
class ConnectionManager:
    def __init__(self):
        # room_id -> set of connections
        self.active_connections: Dict[str, Set[WebSocket]] = {}

    async def connect(self, websocket: WebSocket, room_id: str):
        await websocket.accept()
        if room_id not in self.active_connections:
            self.active_connections[room_id] = set()
        self.active_connections[room_id].add(websocket)
        print(f"WebSocket connected to room '{room_id}'. Total rooms: {len(self.active_connections)}") # Debug log

    def disconnect(self, websocket: WebSocket, room_id: str):
        if room_id in self.active_connections:
            self.active_connections[room_id].discard(websocket)
            print(f"WebSocket disconnected from room '{room_id}'. Connections left: {len(self.active_connections[room_id])}") # Debug log
            # Remove the room if there are no more connections
            if not self.active_connections[room_id]:
                del self.active_connections[room_id]
                print(f"Room '{room_id}' removed as it's empty. Total rooms: {len(self.active_connections)}") # Debug log

    async def broadcast(self, message: Dict[str, Any], room_id: str):
        """Send a message to all connections in a room"""
        if room_id in self.active_connections:
            # Create a list of connections to remove
            to_remove: Set[WebSocket] = set()
            connections_in_room = list(self.active_connections[room_id]) # Make a copy to iterate safely
            print(f"Broadcasting to {len(connections_in_room)} connections in room '{room_id}': {message}") # Debug log
            for connection in connections_in_room:
                try:
                    await connection.send_json(message)
                except Exception as e: # Catch broader exceptions during send
                    print(f"Error sending to a websocket in room {room_id}: {e}. Marking for removal.")
                    to_remove.add(connection)

            # Clean up any disconnected websockets
            if to_remove:
                print(f"Cleaning up {len(to_remove)} disconnected websockets in room {room_id}")
            for connection in to_remove:
                # Disconnect handles the removal from the set
                self.disconnect(connection, room_id)
        else:
             print(f"Cannot broadcast: Room '{room_id}' has no active connections or does not exist.") # Debug log


manager = ConnectionManager()

# Database models
class MessageCreate(BaseModel):
    sender: str = "Anonymous"
    content: str

class RoomCreate(BaseModel):
    room_name: str

class Message(BaseModel):
    id: int
    room_id: str
    sender: str
    content: str
    timestamp: str

class Room(BaseModel):
    id: str
    name: str
    created_at: str

class ExternalMessageInject(BaseModel):
    sender: str = Field(..., min_length=1, description="Identifier for the external system sending the message")
    content: str = Field(..., min_length=1, description="The message content")
    save_to_db: bool = Field(True, description="Whether to save this message to the database history")

# Database setup
def init_db():
    """Initialize database and create tables if they don't exist"""
    if not os.path.exists('chat.db'):
        print("Initializing database...")
        conn = sqlite3.connect('chat.db')
        cursor = conn.cursor()

        # Create messages table with room_id
        cursor.execute('''
        CREATE TABLE IF NOT EXISTS messages (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            room_id TEXT NOT NULL,
            sender TEXT NOT NULL,
            content TEXT NOT NULL,
            timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
        )
        ''')

        # Create rooms table
        cursor.execute('''
        CREATE TABLE IF NOT EXISTS rooms (
            id TEXT PRIMARY KEY,
            name TEXT NOT NULL,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP
        )
        ''')

        # Add a default room if rooms table is empty
        cursor.execute("SELECT COUNT(*) FROM rooms")
        if cursor.fetchone()[0] == 0:
             print("Adding default 'general' room...")
             cursor.execute('''
             INSERT INTO rooms (id, name) VALUES ('general', 'General Chat')
             ''')
        conn.commit()
        conn.close()
        print("Database initialized.")
    else:
        print("Database file 'chat.db' already exists.")


def dict_factory(cursor: Any, row: Any) -> Dict[str, Any]:
    """Convert database row objects to dictionaries"""
    fields = [column[0] for column in cursor.description]
    return {key: value for key, value in zip(fields, row)}

def get_db_connection():
    """Get a database connection with row factory set"""
    conn = sqlite3.connect('chat.db')
    conn.row_factory = dict_factory
    return conn

def get_room(room_id: str) -> Optional[Dict[str, Any]]:
    """Retrieve a specific chat room from the database"""
    conn = get_db_connection()
    cursor = conn.cursor()
    cursor.execute('SELECT * FROM rooms WHERE id = ?', (room_id,))
    room = cursor.fetchone()
    conn.close()
    return room

def get_rooms() -> List[Dict[str, Any]]:
    """Retrieve all chat rooms from the database"""
    conn = get_db_connection()
    cursor = conn.cursor()
    cursor.execute('SELECT * FROM rooms ORDER BY name')
    rooms = cursor.fetchall()
    conn.close()
    return rooms

def create_room(room_id: str, room_name: str) -> bool:
    """Create a new chat room"""
    conn = get_db_connection()
    cursor = conn.cursor()
    try:
        cursor.execute('INSERT INTO rooms (id, name) VALUES (?, ?)',
                      (room_id, room_name))
        conn.commit()
        print(f"Created room: id='{room_id}', name='{room_name}'")
        return True
    except sqlite3.IntegrityError:
        print(f"Room creation failed: ID '{room_id}' already exists.")
        # Room ID already exists
        return False
    finally:
        conn.close()

def get_messages(room_id: str = 'general') -> List[Dict[str, Any]]:
    """Retrieve all messages from the database for a specific room"""
    conn = get_db_connection()
    cursor = conn.cursor()
    cursor.execute('SELECT * FROM messages WHERE room_id = ? ORDER BY timestamp', (room_id,))
    messages = cursor.fetchall()
    conn.close()
    return messages

def add_message(sender: str, content: str, room_id: str = 'general') -> Dict[str, Any]:
    """Add a new message to the database for a specific room and return the message with ID"""
    print(f"Adding message to DB: room='{room_id}', sender='{sender}'")
    conn = get_db_connection()
    cursor = conn.cursor()
    cursor.execute('INSERT INTO messages (room_id, sender, content) VALUES (?, ?, ?)',
                  (room_id, sender, content))
    message_id = cursor.lastrowid
    conn.commit()

    # Get the inserted message with timestamp
    cursor.execute('SELECT * FROM messages WHERE id = ?', (message_id,))
    message = cursor.fetchone()
    conn.close()
    print(f"Message added with ID: {message_id}")
    return message

# Initialize database on startup
init_db()

# Create a template helper function for URL generation
templates.env.globals['url_for'] = app.url_path_for

# Routes
@app.get("/", response_class=HTMLResponse)
async def index(request: Request):
    """Main page that displays the room selection interface"""
    rooms = get_rooms()
    return templates.TemplateResponse(
        "rooms.html",
        {"request": request, "rooms": rooms}
    )

@app.get("/room/{room_id}", response_class=HTMLResponse)
async def room(request: Request, room_id: str):
    """Chat interface for a specific room"""
    current_room = get_room(room_id)
    if not current_room:
        # Optionally: redirect to index or show a 404 template
        return RedirectResponse(url="/", status_code=303) # Redirect if room doesn't exist

    rooms = get_rooms() # Get list of all rooms for sidebar/navigation
    messages = get_messages(room_id)
    return templates.TemplateResponse(
        "index.html",
        {
            "request": request,
            "messages": messages,
            "room_id": room_id,
            "room_name": current_room['name'], # Pass room name to template
            "rooms": rooms, # Pass list of all rooms
        }
    )

@app.post("/create_room")
async def create_room_route(request: Request, room_name: str = Form(...)):
    """Handle creating a new chat room from the form"""
    room_name = room_name.strip()
    if not room_name:
         # Handle empty room name, maybe flash a message if using sessions
        return RedirectResponse(url=request.headers.get("referer", "/"), status_code=303)

    # Create a URL-friendly ID from the name
    room_id = room_name.lower().replace(' ', '-')
    # Remove any non-alphanumeric characters except hyphens
    room_id = re.sub(r'[^a-z0-9-]', '', room_id)

    # Avoid empty room IDs
    if not room_id:
         room_id = f"room-{hash(room_name) % 10000}" # Fallback ID

    if create_room(room_id, room_name):
        # Redirect to the newly created room
        return RedirectResponse(url=f"/room/{room_id}", status_code=303)
    else:
        # Room ID likely already exists (IntegrityError), redirect to the existing room
        # Optionally, add a query parameter to show a message like "?error=exists"
        existing_room = get_room(room_id)
        if existing_room:
             # Redirect to the existing room even if creation failed because ID exists
             return RedirectResponse(url=f"/room/{room_id}", status_code=303)
        else:
             # Should not happen if IntegrityError was the cause, but handle gracefully
             return RedirectResponse(url="/?error=creation_failed", status_code=303)


# WebSocket endpoint
@app.websocket("/ws/{room_id}")
async def websocket_endpoint(websocket: WebSocket, room_id: str):
    # Ensure room exists before connecting WebSocket
    if not get_room(room_id):
        print(f"WebSocket connection rejected: Room '{room_id}' not found.")
        await websocket.close(code=1008) # Policy Violation or similar code
        return

    await manager.connect(websocket, room_id)
    try:
        while True:
            # Receive the message as JSON
            data = await websocket.receive_text()
            try:
                 message_data = json.loads(data)
            except json.JSONDecodeError:
                 print(f"Received invalid JSON via WebSocket in room {room_id}: {data}")
                 continue # Ignore malformed messages

            # Add message to database
            sender = message_data.get("sender", "Anonymous")
            content = message_data.get("content", "")

            if not sender:
                print(f"Received message without sender in room '{room_id}'. Ignoring.")
                continue

            if not content:
                print(f"Received empty message from sender '{sender}' in room '{room_id}'. Ignoring.")
                continue

            # Store in database and get the complete message with ID and timestamp
            db_message = add_message(sender, content, room_id)

            # Broadcast to all connected clients in the room
            await manager.broadcast(db_message, room_id)

            # Fire event (optional - keep if needed for other external systems)
            try:
                async with httpx.AsyncClient() as client:
                    # Ensure the event target URL is correct and service is running
                    await client.post(
                        "http://localhost:8001/events", # Make sure this is configurable/correct
                        json={
                            "event_type": "new_message",
                            "room_id": room_id,
                            "data": {
                                "sender": sender,
                                "content": content,
                                "message_id": db_message["id"]
                            }
                        },
                        timeout=5.0 # Add a timeout
                    )
            except httpx.RequestError as e:
                # More specific error handling for network/HTTP issues
                print(f"Failed to send event to http://localhost:8001/events : {e}")
            except Exception as e:
                print(f"An unexpected error occurred while sending event: {e}")

    except WebSocketDisconnect:
        print(f"WebSocket disconnected normally from room '{room_id}'.")
        manager.disconnect(websocket, room_id)
    except Exception as e:
        # Catch and log other potential errors during websocket handling
        print(f"WebSocket error in room '{room_id}': {e}")
        manager.disconnect(websocket, room_id)
        # Optionally re-raise or handle specific exceptions differently
        # await websocket.close(code=1011) # Internal Server Error


# --- NEW ENDPOINT for External Message Injection ---
@app.post("/api/inject_message/{room_id}")
async def inject_message_api(
    room_id: str,
    message_data: ExternalMessageInject,
    # Example: Add API Key security
    # api_key: str = Depends(get_api_key) # See notes below
):
    """
    Injects a message into a specific chat room from an external system.
    """
    print(f"Received request to inject message into room '{room_id}': {message_data}")

    # 1. Check if the target room exists
    target_room = get_room(room_id)
    if not target_room:
        print(f"Injection failed: Room '{room_id}' not found.")
        raise HTTPException(status_code=404, detail=f"Room '{room_id}' not found")

    # 2. Prepare the message payload
    message_to_broadcast: Dict[str, Any]

    if message_data.save_to_db:
        # Add message to the database and get the full message object back
        try:
            message_to_broadcast = add_message(
                sender=message_data.sender,
                content=message_data.content,
                room_id=room_id
            )
        except Exception as e:
            print(f"Error saving injected message to DB for room {room_id}: {e}")
            raise HTTPException(status_code=500, detail="Failed to save message to database")
    else:
        # Construct a message dictionary manually if not saving
        # Note: This message won't have a DB ID and uses the current server time
        message_to_broadcast = {
            "id": None, # Indicate it's not from DB or use 0, -1 etc.
            "room_id": room_id,
            "sender": message_data.sender,
            "content": message_data.content,
            "timestamp": datetime.datetime.utcnow().isoformat() + "Z", # Use ISO format UTC
        }
        print(f"Injecting message to room '{room_id}' without saving to DB.")


    # 3. Broadcast the message using the ConnectionManager
    if room_id in manager.active_connections and manager.active_connections[room_id]:
        await manager.broadcast(message_to_broadcast, room_id)
        print(f"Successfully broadcasted injected message to room '{room_id}'.")
        return {"status": "success", "message": "Message injected and broadcasted."}
    else:
        # Room exists in DB but no clients are connected via WebSocket currently
        print(f"Message injected for room '{room_id}' (saved={message_data.save_to_db}), but no clients currently connected to broadcast.")
        if message_data.save_to_db:
             return {"status": "success", "message": "Message saved, but no clients connected to broadcast."}
        else:
             return {"status": "success", "message": "Message processed (not saved), but no clients connected to broadcast."}


# API Endpoints for rooms (still needed for room management)
@app.get("/api/rooms")
async def get_rooms_api():
    """API endpoint to get all chat rooms"""
    return get_rooms()

@app.post("/api/rooms")
async def create_room_api(room: RoomCreate):
    """API endpoint to create a new chat room"""
    room_name = room.room_name.strip()
    if not room_name:
        raise HTTPException(status_code=400, detail="Room name is required")

    # Create a URL-friendly ID from the name
    room_id = room_name.lower().replace(' ', '-')
    # Remove any non-alphanumeric characters except hyphens
    room_id = re.sub(r'[^a-z0-9-]', '', room_id)

    if not room_id:
         raise HTTPException(status_code=400, detail="Could not generate a valid room ID from the name")

    if create_room(room_id, room_name):
        new_room = get_room(room_id) # Fetch the created room details
        return {"status": "success", "room": new_room}
    else:
        # If create_room returns False, it means the ID already exists
        existing_room = get_room(room_id)
        if existing_room:
             raise HTTPException(status_code=409, detail=f"Room ID '{room_id}' already exists.", headers={"Location": f"/api/rooms/{room_id}"}) # 409 Conflict
        else:
             # This case should be rare if IntegrityError is the only reason for False
             raise HTTPException(status_code=500, detail="Failed to create room for an unknown reason")

# Get specific room info
@app.get("/api/rooms/{room_id}")
async def get_room_api(room_id: str):
    """API endpoint to get details for a specific room"""
    room = get_room(room_id)
    if not room:
        raise HTTPException(status_code=404, detail=f"Room '{room_id}' not found")
    return room

# For initial loading of messages when a user joins a room
@app.get("/api/messages/{room_id}")
async def get_messages_api(room_id: str):
    """API endpoint to get all messages for a specific room"""
    # Check if room exists first
    if not get_room(room_id):
         raise HTTPException(status_code=404, detail=f"Room '{room_id}' not found")
    return get_messages(room_id)


if __name__ == '__main__':
    import uvicorn
    # Ensure templates and static directory exist
    if not os.path.exists("templates"):
        print("Error: 'templates' directory not found.")
        exit(1)
    if not os.path.exists("static"):
        os.makedirs("static")
        print("Created 'static' directory.")

    print("Starting Uvicorn server...")
    uvicorn.run("app:app", host="0.0.0.0", port=8000, reload=True)
