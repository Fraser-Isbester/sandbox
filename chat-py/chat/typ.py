from pydantic import BaseModel
from enum import Enum
from datetime import datetime, timezone

class Role(str, Enum):
    user = "user"
    assistant = "assistant"
    unknown = "unknown"

class Message(BaseModel):
    conversation: str | None = None
    timestamp: datetime | None = datetime.now(timezone.utc)
    content: str
    role: Role
