from datetime import datetime, timezone
from enum import Enum

from pydantic import BaseModel


class Role(str, Enum):
    user = "user"
    assistant = "assistant"
    unknown = "unknown"

class Message(BaseModel):
    conversation: str | None = None
    timestamp: datetime | None = datetime.now(timezone.utc)
    content: str
    role: Role
