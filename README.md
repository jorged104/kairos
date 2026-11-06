# Kairos

Gestor de proyectos con gamificación, pensado para cualquier tipo de organización. Organiza el trabajo de la estrategia a la tarea:

```
Objetivo → Programa → Proyecto → Hito → Work item (actividad, subactividad…)
```

Cada persona gana **XP** y **monedas** al cerrar su trabajo. Todos los puntos quedan en un registro inmutable y el nivel se calcula a partir del XP; nunca se guarda como un valor editable.

## Qué incluye

- **Proyectos y trabajo:** metodologías y flujos configurables, hitos, work items con tallas, varios asignados, dependencias, etiquetas, campos personalizados y plantillas.
- **Vistas:** tabla, kanban, cronograma y vistas guardadas, por proyecto y entre proyectos.
- **Planificación y seguimiento:** línea base, plan frente a real, semáforo, programas, portafolio, seguimiento y objetivos con resultados clave.
- **Documentos:** páginas tipo Notion para proyectos, hitos y work items, con archivos en la BD o en S3.
- **Colaboración:** comentarios con menciones, reuniones con plantillas, notificaciones en tiempo real y «Mi panel».
- **Gamificación:** XP, monedas, niveles y ranking por equipo.
- **Usuarios y acceso:** usuario y contraseña, SSO (OIDC), roles y permisos, equipos jerárquicos, atributos, invitaciones por correo y recuperación de contraseña.
- **Administración:** pantalla Sistema para acceso, SSO, almacenamiento, apariencia, correo y conexiones de IA, más auditoría de cambios.
- **IA:** conexión de clientes como Claude o ChatGPT por MCP, con una URL propia para cada conexión.

## Instalación

| Quiero… | Guía |
|---|---|
| Probarlo o desarrollar en mi máquina | [Instalación local](docs/guia/instalacion-local.md) |
| Ponerlo en un servidor para mi equipo | [Instalación en un servidor](docs/guia/instalacion-servidor.md) |
| Ver todas las opciones de configuración | [Configuración](docs/guia/configuracion.md) |
| Pasar a una versión nueva | [Actualización](docs/guia/actualizacion.md) |

Prueba rápida en local (Docker, Compose v2 y `make`):

```bash
cp .env.example .env
make up        # BD + API + frontend
make migrate   # crea las tablas
```

Abre http://localhost:5173 y regístrate con `admin@kairos.local` para entrar como administrador.

## Stack

| Capa | Tecnología |
|---|---|
| Frontend | Vue 3, Vite, TypeScript, Vue Router, Pinia, TanStack Query, vue-i18n |
| UI | Tailwind CSS v4, shadcn-vue (Reka UI), Lucide, TipTap |
| Backend | Go, Chi, pgx/v5, sqlc, Squirrel, slog |
| Tiempo real | WebSocket (Gorilla) |
| Base de datos | PostgreSQL 17, migraciones con Goose |
| Infra | Docker + Docker Compose |
| CI / despliegue | GitHub Actions; previsualización con runner self-hosted |

## Comandos de desarrollo

```bash
make up          # BD + API + frontend en contenedores
make up-sso      # … más Keycloak de prueba
make up-s3       # … con los archivos en S3 (SeaweedFS)
make up-mail     # … más Mailpit para ver los correos
make down        # para los contenedores
make migrate     # aplica migraciones
make sqlc        # regenera el código de sqlc
make test        # tests de backend y frontend
make dev-api     # API en el host
make dev-web     # Vite en el host
```

Detalles en la [instalación local](docs/guia/instalacion-local.md).

## API

La API vive bajo `/api` y responde en JSON `snake_case`. Los errores siguen siempre el formato `{ "error": { "code": "...", "message": "..." } }`.

| Ruta | Respuesta |
|---|---|
| `GET /health/live` | `200` mientras el proceso está vivo; no consulta la BD |
| `GET /health/ready` | `200` si la BD responde en 2 s; si no, `503` con `database_unavailable` |

## Previsualización

Cada push a `main` pasa los tests en GitHub Actions y, si están en verde, se despliega en el servidor de previsualización; si la versión nueva no responde, vuelve sola a la anterior. Ver [deploy/preview/README.md](deploy/preview/README.md).

## Cómo se trabaja en este repo

El desarrollo está guiado por especificaciones: nada se implementa si no está en una spec aprobada.

1. **Spec** (`docs/specs/NNN-nombre/spec.md`): qué se construye y por qué, con requisitos en notación EARS. Se redacta con la skill `spec-generator`.
2. **Plan** (`plan.md`): las decisiones técnicas.
3. **Tareas** (`tasks.md`): pasos en orden, con los RF que cubre cada uno.
4. **Implementación** en una rama `feat/NNN-descripcion`, con commits pequeños y los tests en verde.

El trabajo también puede empezar con un **issue en GitHub**: la skill `issue-a-pr` (`/issue-a-pr N`) prepara la spec y el plan y abre una PR en borrador, que se implementa cuando alguien responde `/aprobar`.

| Spec | Tema |
|---|---|
| [001](docs/specs/001-infraestructura-base/spec.md) | Infraestructura base |
| [002](docs/specs/002-entorno-previsualizacion/spec.md) | Entorno de previsualización |
| [003](docs/specs/003-usuarios/spec.md) | Usuarios, roles, permisos y equipos |
| [004](docs/specs/004-proyectos/spec.md) | Proyectos, metodologías, hitos, work items y objetivos |
| [005](docs/specs/005-paginas-tipo-notion/spec.md) | Páginas tipo Notion |
| [006](docs/specs/006-tallas-asignados-comentarios/spec.md) | Tallas, varios asignados y comentarios |
| [007](docs/specs/007-tareas-metricas-etiquetas/spec.md) | Métricas, etiquetas e historial |
| [008](docs/specs/008-pantallas-proyecto/spec.md) | Pantallas de proyecto |
| [009](docs/specs/009-plan-linea-base/spec.md) | Plan, línea base y semáforo |
| [010](docs/specs/010-programas-portafolio/spec.md) | Programas y portafolio |
| [011](docs/specs/011-seguimiento/spec.md) | Seguimiento de proyectos |
| [012](docs/specs/012-vistas-proyectos/spec.md) | Vistas de proyectos |
| [013](docs/specs/013-mi-panel/spec.md) | Mi panel |
| [014](docs/specs/014-gamificacion/spec.md) | Gamificación |
| [015](docs/specs/015-administracion-sistema/spec.md) | Administración del sistema |
| [016](docs/specs/016-mcp/spec.md) | Conexión de clientes de IA (MCP) |
| [017](docs/specs/017-notificaciones/spec.md) | Notificaciones |
| [018](docs/specs/018-reuniones/spec.md) | Reuniones |
| [019](docs/specs/019-correo/spec.md) | Correo: invitaciones y recuperación de contraseña |

Las reglas para agentes de código (Claude Code, Codex, Copilot…) están en [AGENTS.md](AGENTS.md).
