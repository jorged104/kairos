"""HTTP service of the report agent: POST /generate streams NDJSON (ADR 0004)."""

from __future__ import annotations

import json
import logging
from collections.abc import AsyncIterator
from typing import Any

import httpx
from starlette.applications import Starlette
from starlette.requests import Request
from starlette.responses import JSONResponse, StreamingResponse
from starlette.routing import Route

from kairos_agent import agent

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s %(message)s")

REQUIRED = ("llm", "system_prompt", "prompt", "tools", "tools_url", "token")


def create_app(client: httpx.AsyncClient | None = None, model_factory: agent.ModelFactory = agent.build_model) -> Starlette:
    """The app; the tests pass a client and a model on fake transports."""

    async def health(_: Request) -> JSONResponse:
        return JSONResponse({"status": "ok"})

    async def generate(request: Request) -> JSONResponse | StreamingResponse:
        try:
            req: dict[str, Any] = await request.json()
        except json.JSONDecodeError:
            return JSONResponse({"error": {"code": "validation_failed", "message": "JSON no válido"}}, status_code=400)
        missing = [k for k in REQUIRED if not req.get(k) and k != "tools"]
        if missing or not isinstance(req.get("tools"), list) or not {"base_url", "model"} <= set(req["llm"] or {}):
            return JSONResponse({"error": {"code": "validation_failed", "message": "Faltan campos", "details": {"fields": missing}}}, status_code=400)

        async def lines() -> AsyncIterator[bytes]:
            async with httpx.AsyncClient() as own:
                async for event in agent.generate(req, client or own, model_factory):
                    yield (json.dumps(event, ensure_ascii=False) + "\n").encode()

        return StreamingResponse(lines(), media_type="application/x-ndjson")

    return Starlette(routes=[Route("/health", health), Route("/generate", generate, methods=["POST"])])


app = create_app()
