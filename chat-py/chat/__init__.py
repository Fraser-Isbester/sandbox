from fastapi import FastAPI, Depends, HTTPException
import anthropic

from chat.typ import Message
from chat import handlers
from chat.llm import new_client

app = FastAPI()

@app.get("/")
def read_root():
    return {"Hello": "World"}

@app.post("/chat", response_model=Message)
async def chat(message: Message, client: anthropic.Anthropic = Depends(new_client)):

    response, err = handlers.chat(message, client)
    if err:
        return HTTPException(status_code=500, detail=err)

    return response
