# ADR 0004 — Agente de informes en un servicio Python con deepagents

**Estado:** aceptada (2026-11-05), spec 020.

## Contexto
La spec 020 pide que una IA escriba informes de proyecto consultando por sí misma proyectos, hitos y tareas. Mientras trabaja debe mostrar la lista de pasos que se ha marcado. El usuario pidió expresamente construir ese agente con **deepagents** (LangChain).

deepagents existe en Python (la versión principal) y en TypeScript (un port). El backend de Kairos es Go.

## Decisión
- El agente vive en un **servicio interno nuevo, `agent/`, en Python 3.12** gestionado con uv:
  - usa deepagents y `langchain-openai`;
  - no tiene base de datos ni estado;
  - no publica puertos fuera de la red de Docker.
- La **API en Go sigue siendo la única que lee datos.** Las herramientas del modelo son llamadas HTTP de vuelta a la API, y las autoriza un token de un solo uso por generación. La API aplica:
  - los permisos de la persona;
  - el árbol del proyecto;
  - los límites.
- La API define las herramientas (nombre, descripción y esquema JSON) y las envía en cada petición. El agente no conoce el modelo de datos de Kairos.
- El agente:
  - habla con cualquier servicio compatible con la API de chat de OpenAI;
  - recibe la configuración (dirección, clave y modelo) en cada petición desde la API.

  La clave no se guarda en el agente.
- Se desactivan las herramientas de deepagents que no tienen sentido aquí:
  - `execute`;
  - los subagentes (`task`), que la spec deja fuera de alcance.

  La lista de pasos sale de `TodoListMiddleware`.

## Consecuencias
- **El stack crece:** Python, uv y un contenedor más, en desarrollo, en previsualización y en servidor. `make test` y el workflow pasan también los tests del agente.
- **Si el agente no está configurado** (`AGENT_URL` vacía) o no responde, generar falla con un error claro. Los informes guardados se leen y editan igual.
- **La seguridad no depende del agente:** aunque el modelo pidiera otra cosa, la API solo responde lo que la persona puede ver, dentro del árbol y en modo lectura.
- **Actualizar deepagents** se hace en `agent/pyproject.toml` y `uv.lock`. Es beta en parte (perfiles); fijamos la versión.
