"""The deepagents agent that writes a project report (spec 020, ADR 0004).

The API of Kairos sends everything in each request: the AI service, the
prompts and the definitions of the tools. Each tool is a POST back to the API
with the token of the generation, so this service never reads data itself and
keeps no state.
"""

from __future__ import annotations

import logging
from collections.abc import AsyncIterator, Callable
from typing import Any

import httpx
import openai
from deepagents import (
    GeneralPurposeSubagentProfile,
    HarnessProfile,
    create_deep_agent,
    register_harness_profile,
)
from langchain.agents.middleware import TodoListMiddleware
from langchain_core.exceptions import ContextOverflowError
from langchain_core.messages import AIMessage, AIMessageChunk
from langchain_core.tools import StructuredTool
from langchain_openai import ChatOpenAI

log = logging.getLogger("kairos_agent")

# The report needs no shell and no subagents (spec 020, out of scope).
register_harness_profile(
    "openai",
    HarnessProfile(
        excluded_tools=frozenset({"execute", "task"}),
        general_purpose_subagent=GeneralPurposeSubagentProfile(enabled=False),
    ),
)

# Seconds without an answer of the AI service before giving up (RF-71).
LLM_TIMEOUT = 30

Event = dict[str, Any]
ModelFactory = Callable[[dict[str, str]], ChatOpenAI]


def build_model(llm: dict[str, str], http_client: httpx.AsyncClient | None = None) -> ChatOpenAI:
    """A chat model on any service compatible with OpenAI's chat API."""
    return ChatOpenAI(
        model=llm["model"],
        base_url=llm["base_url"],
        api_key=llm.get("api_key") or "sin-clave",
        timeout=LLM_TIMEOUT,
        max_retries=1,
        streaming=True,
        use_responses_api=False,
        http_async_client=http_client,
    )


def build_tools(definitions: list[dict[str, Any]], tools_url: str, token: str, client: httpx.AsyncClient) -> list[StructuredTool]:
    """The read-only tools of Kairos, each one a call back to its API."""

    def make(name: str, description: str, schema: dict[str, Any]) -> StructuredTool:
        async def run(**kwargs: Any) -> str:
            res = await client.post(
                f"{tools_url}/{name}",
                json={k: v for k, v in kwargs.items() if v is not None},
                headers={"Authorization": f"Bearer {token}"},
                timeout=60,
            )
            if res.status_code != 200:
                return f"Error: Kairos respondió {res.status_code}."
            return res.json().get("content", "")

        return StructuredTool.from_function(coroutine=run, name=name, description=description, args_schema=schema)

    return [make(d["name"], d.get("description", ""), d.get("parameters") or {"type": "object", "properties": {}}) for d in definitions]


def error_code(exc: BaseException) -> str:
    """The code the interface translates (RF-73)."""
    if isinstance(exc, ContextOverflowError):
        return "ai_context_length"
    if isinstance(exc, openai.APITimeoutError):
        return "ai_timeout"
    if isinstance(exc, openai.APIConnectionError):
        return "ai_unreachable"
    if isinstance(exc, (openai.AuthenticationError, openai.PermissionDeniedError)):
        return "ai_auth"
    if isinstance(exc, openai.NotFoundError):
        return "ai_model"
    if isinstance(exc, openai.RateLimitError):
        return "ai_quota"
    if isinstance(exc, openai.APIStatusError):
        text = str(exc).lower()
        if "context_length" in text or "maximum context" in text or "too long" in text:
            return "ai_context_length"
        if "quota" in text:
            return "ai_quota"
        if "tool" in text or "function" in text:
            return "ai_tools"
        return "ai_rejected"
    return "ai_error"


def _text(chunk: AIMessageChunk) -> str:
    content = chunk.content
    if isinstance(content, str):
        return content
    return "".join(block.get("text", "") for block in content if isinstance(block, dict) and block.get("type") == "text")


async def generate(
    req: dict[str, Any],
    client: httpx.AsyncClient,
    model_factory: ModelFactory = build_model,
) -> AsyncIterator[Event]:
    """Runs the agent and yields the events of the stream: todos, text,
    reset (the text so far was not the report but a note before a query),
    done and error."""
    try:
        agent = create_deep_agent(
            model=model_factory(req["llm"]),
            tools=build_tools(req["tools"], req["tools_url"], req["token"], client),
            system_prompt=req["system_prompt"],
            middleware=[TodoListMiddleware()],
        )
        wrote = False  # text sent since the last reset
        stream = agent.astream(
            {"messages": [{"role": "user", "content": req["prompt"]}]},
            stream_mode=["updates", "messages"],
            config={"recursion_limit": 300},
        )
        async for mode, payload in stream:
            if mode == "messages":
                chunk, meta = payload
                if not isinstance(chunk, AIMessageChunk) or meta.get("langgraph_node") != "model":
                    continue
                if chunk.tool_call_chunks and wrote:
                    yield {"type": "reset"}
                    wrote = False
                if text := _text(chunk):
                    wrote = True
                    yield {"type": "text", "delta": text}
                continue
            for update in (payload or {}).values():
                if not isinstance(update, dict):
                    continue
                if isinstance(update.get("todos"), list):
                    yield {"type": "todos", "todos": [{"content": t.get("content", ""), "status": t.get("status", "pending")} for t in update["todos"]]}
                messages = update.get("messages")
                messages = messages if isinstance(messages, list) else [messages]
                for m in messages:
                    if isinstance(m, AIMessage) and m.tool_calls and wrote:
                        yield {"type": "reset"}
                        wrote = False
        yield {"type": "done"}
    except Exception as exc:  # every failure ends the stream with a code
        code = error_code(exc)
        if code == "ai_error":
            log.exception("generación de informe")
        else:
            log.warning("generación de informe: %s", code)
        yield {"type": "error", "code": code}
