import logging
import uuid
from pathlib import Path

from chat.typ import Message

class Conversation:

    def __init__(self, data_dir: str):
        self.data_dir = Path(data_dir)
        self.id = str(uuid.uuid4())
        self.path = data_dir / Path(f"conv-{self.id}.ndjson")
        self.messages: list[Message] = []

    def append(self, msg):
        self.messages.append(msg)
        with self.path.open("a+") as f:
            msg = self.messages[-1].model_dump_json() + "\n"
            f.write(msg)

    def get_messages(self):
        ordered_messages = sorted(self.messages, key=lambda msg: msg.timestamp, reverse=True)
        filtered = [{"content": msg.content, "role": msg.role} for msg in ordered_messages]
        return filtered

    def load(self, id):
        path = self.data_dir / Path(f"conv-{id}.ndjson")
        if not path.exists():
            err = FileNotFoundError(f"This conversation was not found: {id}")
            logging.error(err)
            return err

        # Update conversation
        self.id = id
        self.path = path

        with path.open("r") as f:
            for line in f.readlines():
                msg = Message.model_validate_json(line)
                self.append(msg)
