from anthropic import Client

from chat.storage import Conversation
from chat.typ import Message, Role


def chat(message: Message, client: Client) -> Message:

    conv = Conversation("./data")
    if message.conversation:
        conv.load(message.conversation)

    conv.append(message)

    response = client.messages.create(
        model="claude-3-5-sonnet-20241022",
        max_tokens=1024,
        messages=conv.get_messages()
    )

    content = response.content[0].text
    response = Message(conversation=conv.id, content=content, role=Role.assistant)
    conv.append(response)
    return response, None
