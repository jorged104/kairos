"""The agent against a fake AI service and a fake Kairos (spec 020, ADR 0004)."""

from __future__ import annotations

import json
from typing import Any

import httpx
import pytest
from starlette.testclient import TestClient

from kairos_agent import agent
from kairos_agent.app import create_app

TOOLS = [
    {"name": "project_tree", "description": "El árbol.", "parameters": {"type": "object", "properties": {}}},
    {
        "name": "list_work_items",
        "description": "Tareas.",
        "parameters": {"type": "object", "properties": {"project": {"type": "string"}, "overdue": {"type": "boolean"}}, "required": ["project"]},
    },
]


def request(**extra: Any) -> dict[str, Any]:
    return {
        "llm": {"base_url": "http://ia.test/v1", "api_key": "sk-x", "model": "modelo-x"},
        "system_prompt": "Reglas.",
        "prompt": "Escribe el informe.",
        "tools": TOOLS,
        "tools_url": "http://kairos.test/api/reports/agent/tools",
        "token": "tok-123",
        **extra,
    }


def sse(chunks: list[dict[str, Any]]) -> bytes:
    out = ""
    for delta, finish in chunks:
        out += "data: " + json.dumps({
            "id": "c1", "object": "chat.completion.chunk", "created": 0, "model": "modelo-x",
            "choices": [{"index": 0, "delta": delta, "finish_reason": finish}],
        }) + "\n\n"
    return (out + "data: [DONE]\n\n").encode()


def tool_call(name: str, args: dict[str, Any], text: str = "") -> bytes:
    first: dict[str, Any] = {"role": "assistant", "content": text or None}
    chunks: list[Any] = [(first, None)] if text else []
    call = {"index": 0, "id": f"call_{name}", "type": "function", "function": {"name": name, "arguments": json.dumps(args)}}
    chunks.append(({"role": "assistant", "tool_calls": [call]} if not text else {"tool_calls": [call]}, None))
    chunks.append(({}, "tool_calls"))
    return sse(chunks)


def answer(*pieces: str) -> bytes:
    chunks: list[Any] = [({"role": "assistant", "content": ""}, None)]
    chunks += [({"content": p}, None) for p in pieces]
    chunks.append(({}, "stop"))
    return sse(chunks)


class FakeAI:
    """Answers each call of the model with the next scripted response."""

    def __init__(self, *responses: bytes | httpx.Response | Exception) -> None:
        self.responses = list(responses)
        self.bodies: list[dict[str, Any]] = []

    def handler(self, req: httpx.Request) -> httpx.Response:
        self.bodies.append(json.loads(req.content))
        r = self.responses.pop(0) if len(self.responses) > 1 else self.responses[0]
        if isinstance(r, Exception):
            raise r
        if isinstance(r, httpx.Response):
            return r
        return httpx.Response(200, content=r, headers={"content-type": "text/event-stream"})

    def factory(self, llm: dict[str, str]):
        return agent.build_model(llm, httpx.AsyncClient(transport=httpx.MockTransport(self.handler)))


class FakeKairos:
    def __init__(self) -> None:
        self.calls: list[tuple[str, dict[str, Any], str]] = []

    def handler(self, req: httpx.Request) -> httpx.Response:
        name = req.url.path.rsplit("/", 1)[-1]
        self.calls.append((name, json.loads(req.content), req.headers.get("authorization", "")))
        return httpx.Response(200, json={"content": json.dumps({"items": [{"name": "Programa"}]})})


def run(ai: FakeAI, kairos: FakeKairos | None = None, body: dict[str, Any] | None = None) -> tuple[int, list[dict[str, Any]]]:
    kairos = kairos or FakeKairos()
    client = httpx.AsyncClient(transport=httpx.MockTransport(kairos.handler))
    with TestClient(create_app(client=client, model_factory=ai.factory)) as tc:
        res = tc.post("/generate", json=body or request())
    if res.status_code != 200:
        return res.status_code, []
    assert res.headers["content-type"].startswith("application/x-ndjson")
    return 200, [json.loads(line) for line in res.text.splitlines() if line.strip()]


def test_writes_the_report_with_its_plan_and_queries() -> None:
    ai = FakeAI(
        tool_call("write_todos", {"todos": [{"content": "Leer el árbol", "status": "in_progress"}, {"content": "Escribir", "status": "pending"}]}),
        tool_call("project_tree", {}),
        tool_call("list_work_items", {"project": "p1", "overdue": True}, text="Voy a mirar las tareas."),
        answer("# Informe\n\n", "Todo **bien**."),
    )
    kairos = FakeKairos()
    status, events = run(ai, kairos)
    assert status == 200

    # The plan of the agent (RF-38).
    todos = [e for e in events if e["type"] == "todos"]
    assert todos and todos[0]["todos"] == [{"content": "Leer el árbol", "status": "in_progress"}, {"content": "Escribir", "status": "pending"}]

    # The queries go back to Kairos with the token of the generation.
    assert [(n, a) for n, a, _ in kairos.calls] == [("project_tree", {}), ("list_work_items", {"project": "p1", "overdue": True})]
    assert all(auth == "Bearer tok-123" for *_, auth in kairos.calls)

    # A note before a query is withdrawn; the report is the last text.
    kinds = [e["type"] for e in events]
    assert kinds[-1] == "done"
    text = ""
    for e in events:
        if e["type"] == "reset":
            text = ""
        elif e["type"] == "text":
            text += e["delta"]
    assert text == "# Informe\n\nTodo **bien**."
    assert "reset" in kinds and kinds.index("reset") > kinds.index("text")

    # The model gets the prompts and only the tools allowed: no shell, no subagents.
    first = ai.bodies[0]
    assert first["model"] == "modelo-x" and first["stream"] is True
    offered = {t["function"]["name"] for t in first["tools"]}
    assert {"project_tree", "list_work_items", "write_todos"} <= offered
    assert not offered & {"execute", "task"}
    system = first["messages"][0]
    assert system["role"] == "system"
    assert "Reglas." in (system["content"] if isinstance(system["content"], str) else "".join(b["text"] for b in system["content"]))
    assert first["messages"][-1] == {"role": "user", "content": "Escribe el informe."}


@pytest.mark.parametrize(
    ("response", "code"),
    [
        (httpx.Response(401, json={"error": {"message": "bad key", "code": "invalid_api_key"}}), "ai_auth"),
        (httpx.Response(404, json={"error": {"message": "The model does not exist", "code": "model_not_found"}}), "ai_model"),
        (httpx.Response(400, json={"error": {"message": "maximum context length exceeded", "code": "context_length_exceeded"}}), "ai_context_length"),
        (httpx.Response(400, json={"error": {"message": "tools are not supported", "code": None}}), "ai_tools"),
        (httpx.ConnectError("refused"), "ai_unreachable"),
    ],
)
def test_errors_of_the_ai_service_have_a_code(response: Any, code: str) -> None:
    # One retry per call (max_retries=1): two scripted failures.
    status, events = run(FakeAI(response, response))
    assert status == 200
    assert events[-1] == {"type": "error", "code": code}


def test_rejects_an_incomplete_request() -> None:
    status, _ = run(FakeAI(), body={"prompt": "x"})
    assert status == 400


def test_health() -> None:
    with TestClient(create_app()) as tc:
        assert tc.get("/health").json() == {"status": "ok"}
