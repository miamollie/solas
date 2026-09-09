# What does Solas do

Solas is a proxy between an LLM consumer and Ollama. It's a toy project created for learning purposes, to explore energy attribution for AI at the token level.

The proxy sits between the LLM client (chat UI for example) and locally run LLM (ollama in this case). It captures requests, and publishes both baseline observability metrics (requests per second) as well as token values. Used in isolation it is helpful information about token counts exposes as prometheus data metric.

## Solas Stack

Refer to the stack docs for more information, but TLDR it's an opinionated observable Local LLM stack.

- Opinionated: Makes choices about local LLM provider (ollama), chatUI (openWebui), metrics visualisation (grafana)
- Observable: runs prometheus and grafana to visualise data from the Solas proxy.
- Local LLM stack: is what it is.
