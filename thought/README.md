# Thought
Thought is for watching/exploring how machines (or not machines) think and implementing a simple interface for changing that behvaiour.

In general, as an Agent (ie. an llm) steps through it's nodes, it will be instructed to make a tool-call at the end of every interaction to update the server with a structured form of that thought. The server will receive the data and non-atomicly vectorize them into an embedding. This will give us both a Graph of thoughts that led to other thoughts, and a vector space of thoughts that can be used to compare thoughts. Thoughts will begin decaying upon inactivity, and will be pruned from the system after a certain amount of time. If the agent accesses a thought its decay will be reset. How thoughts are injected into the graph of throught is still up for debate.


## Usage

```shell
thought serve --detach # Starts the server & detached the proccess
thought ask "I wonder what the weather is like in San Francisco" --detach # Initializes an agent genesis node
thought listen # streams SSE of thought summaries
```
