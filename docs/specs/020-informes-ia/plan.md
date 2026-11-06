# Plan — Spec 020 «Informes de proyecto con IA»

## Piezas

```
navegador ──POST /api/projects/{id}/reports/generate (NDJSON en streaming)──► api (Go)
                                                                               │  sesión de generación
                                                                               │  (token, árbol, contador)
                                                                               ▼
                                                    agent (Python, deepagents) ◄── POST /generate (NDJSON)
                                                          │            │
                     servicio de IA compatible con OpenAI ◄┘            └──► api: POST /api/reports/agent/tools/{tool}
                                                                             (Bearer token de la sesión)
```

- **api (Go)** sigue mandando:
  - comprueba los permisos;
  - guarda las plantillas, los informes y la configuración;
  - audita;
  - ejecuta **todas** las consultas de datos.
- **agent (Python)** es un servicio interno sin puerto público, sin estado y sin base de datos. Solo:
  - construye el agente deepagents;
  - habla con el servicio de IA;
  - reenvía lo que pasa como líneas NDJSON.
- Las herramientas que ve el modelo las define la API. Viajan en cada petición como esquemas JSON, y el agente las convierte en herramientas de LangChain que hacen POST de vuelta a la API. Así solo hay una fuente de verdad para lo que la IA puede leer (RF-45 a RF-50).

Decisión registrada en [ADR 0004](../../adr/0004-agente-de-informes-en-python.md).

## Base de datos — migración `00018_informes.sql` (aditiva)
- `system_settings` gana:
  - `ai_enabled boolean default false`;
  - `ai_base_url text default ''`;
  - `ai_api_key bytea`, sellada con la caja de la spec 015;
  - `ai_model text default ''`.
- Tabla nueva `report_templates`:
  - columnas: `id`, `name`, `description`, `prompt`, `archived_at`, `created_at`, `updated_at`;
  - índice único sobre `lower(name)` con `archived_at IS NULL`;
  - se siembra «Estado del proyecto» (RF-23).
- Tabla nueva `project_reports`:
  - columnas: `id`, `project_id`, `template_id` (nullable), `template_name` (copia, RF-26), `title`, `content`, `author_id`, `updated_by`, `created_at`, `updated_at`, `version`;
  - índice `(project_id, updated_at desc)`.
- Consultas sqlc en `queries/reports.sql`, más `SetAISettings` en `system_settings.sql`.

## Backend
- **`internal/config`:**
  - `AGENT_URL`: si está vacía, no se puede generar (RF-16);
  - `AGENT_CALLBACK_URL`: cómo ve el agente a la API; por defecto `http://api:8080`.
- **`internal/llm`:** `Check(ctx, cfg)` hace una petición mínima a `{base}/chat/completions` con una herramienta `ping` y `tool_choice: required`. Clasifica el resultado en `connect`, `auth`, `model` o `tools` (RF-11 a RF-13).
- **`internal/settings`:**
  - vista `AIView` sin la clave;
  - `UpdateAI` con la clave que se conserva si llega vacía (RF-9);
  - `TestAI`;
  - campos auditables, con la clave como «changed» (RF-15);
  - `Holder.AI()`;
  - rutas `PUT /system/settings/ai` y `POST /system/settings/ai/test`.
- **`internal/mcp/reports.go`:** las herramientas de solo lectura para el agente reutilizan el despacho interno y `shape` del MCP:
  - herramientas: `project_tree`, `get_project`, `project_overview`, `list_milestones`, `get_milestone`, `list_work_items` (hasta 200, con `total`) y `get_work_item` (sin comentarios);
  - cada una pasa por un `guard`: el proyecto, o el proyecto del hito o la tarea, tiene que estar en el árbol (RF-48);
  - se quitan correos y atributos (RF-50).
- **`internal/reports`:**
  - **plantillas:** CRUD y archivo con `SystemManage` (RF-18 a RF-27); `GET /report-templates` devuelve las activas para los generadores;
  - **informes:**
    - rutas: `GET/POST /projects/{id}/reports`, `GET/PUT/DELETE /reports/{id}`;
    - permisos: lectura con `CanSeeProject`; escritura con `CanManageProject` (RF-28 a RF-34, RF-62 a RF-70);
    - `version_conflict`;
    - `doc` = `richtext.FromMarkdown` + `Clean`, para pintarlo con el visor TipTap; nunca HTML (RF-31, RF-32);
    - `origin` (`ai`, `ai_edited` o `manual`) va en la auditoría (RF-70);
  - **generación:** `POST /projects/{id}/reports/generate` con `{template_id, instructions}` o con `{draft, change}`. El proceso:
    1. abre una sesión: un token de 256 bits, el árbol visible y un contador de consultas;
    2. llama al agente con el prompt fijo (RF-53), el de la plantilla, la tarea, las herramientas y la configuración de IA de ese momento;
    3. reenvía al navegador `todos`, `text`, `reset`, `tool` (con el nombre del proyecto), `ping` cada 10 s, `done` y `error`;
    4. corta a los 10 min (RF-72);
    5. si el navegador se va, cancela.

    Máximo 2 por persona (RF-77): si no, `report_generation_busy` 429.
  - **herramientas del agente:** `POST /reports/agent/tools/{tool}` es una ruta sin sesión de navegador; la autoriza el Bearer token de la sesión de generación. Pasada la consulta 60 responde «límite alcanzado» (RF-52).

## Agente — `agent/` (Python 3.12, uv)
- **Dependencias:** `deepagents`, `langchain-openai`, `starlette`, `uvicorn`, `httpx`. Para los tests: `pytest`.
- **`POST /generate`:**
  - el modelo es `ChatOpenAI(base_url, api_key, model, timeout=30, max_retries=1, streaming=True, use_responses_api=False)`;
  - el agente es `create_deep_agent` con `TodoListMiddleware` (RF-38);
  - un `HarnessProfile` quita `execute` y `task` y desactiva el subagente general;
  - se usa `astream(stream_mode=["updates", "messages"])`;
  - los textos de mensajes con herramientas se anulan con `reset`;
  - los errores de OpenAI y httpx se traducen a códigos `llm_*`.
- **`GET /health`.**
- **Dockerfile** con `uv sync --frozen`.
- **Tests:**
  - un servicio OpenAI falso y la API de herramientas falsa con `httpx.MockTransport`;
  - comprueban streaming, todos, reset y la clasificación de errores.

## Frontend
- **Sistema → «Informes con IA»:** `AISettings.vue` (configuración, prueba y aviso, RF-14) y `ReportTemplatesAdmin.vue`.
- **Proyecto → «Informes»:** `features/reports/`:
  - `ReportsTab` lista;
  - `ReportView` pinta el `doc` con `RichTextView`;
  - `ReportEditor` tiene título, Markdown, vista previa (`POST /reports/preview`), «Pedir cambio», «Deshacer cambio» y guardar;
  - `GenerateDialog` pide plantilla e instrucciones;
  - `useReportStream` lee el NDJSON con `fetch`, con CSRF y abortable.
- **Navegación:** query `?report=<id>` o `?report=new` en la pestaña.
- **Aviso al salir con cambios:** `onBeforeRouteLeave` y `beforeunload` (RF-43).
- **Textos:** todos en `es.json`.

## Infra y documentación
- **Contenedores:**
  - servicio `agent` en `docker-compose.yml`, `deploy/preview/compose.yml` y `deploy/servidor/compose.yml`;
  - la API recibe `AGENT_URL=http://agent:8000`.
- **Makefile:** `test-agent` (uv), dentro de `make test`.
- **Workflow:** job de tests del agente.
- **Documentación:** `AGENTS.md` (stack y estructura), `docs/guia/*` (variables y pestaña nueva) y ADR 0004.

## Pruebas
- **apitest:**
  - **configuración de IA:** guardar, conservar la clave, prueba con un servidor OpenAI falso y auditoría;
  - **plantillas;**
  - **informes:**
    - CRUD, permisos y `version_conflict`;
    - `doc` sin HTML;
  - **generación con un agente falso** (servidor HTTP de test que emite NDJSON y llama a las herramientas con el token):
    - árbol aislado;
    - límite de 60 consultas;
    - 200 tareas como máximo;
    - sin comentarios ni correos;
    - el token deja de valer al terminar;
    - máximo 2 en curso;
  - **matriz:** filas nuevas.
- **pytest** del agente.
- **Vitest:** Sistema, lista, editor y streaming simulado.
