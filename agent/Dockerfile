# Agente de informes (spec 020, ADR 0004). Servicio interno: no publica puertos.
FROM python:3.12-slim AS base
COPY --from=ghcr.io/astral-sh/uv:0.12.1 /uv /usr/local/bin/uv
WORKDIR /app
ENV UV_COMPILE_BYTECODE=1 UV_LINK_MODE=copy \
    PYTHONUNBUFFERED=1 \
    # Nothing leaves the server but the calls to the configured AI service.
    LANGSMITH_TRACING=false LANGCHAIN_TRACING_V2=false
COPY pyproject.toml uv.lock ./
RUN uv sync --frozen --no-dev --no-install-project
COPY kairos_agent ./kairos_agent
RUN useradd --system --uid 10001 agent
USER agent
EXPOSE 8000
CMD ["/app/.venv/bin/uvicorn", "kairos_agent.app:app", "--host", "0.0.0.0", "--port", "8000", "--no-access-log"]
