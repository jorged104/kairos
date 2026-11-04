# Plan 014 — Gamificación

Plan de implementación de [spec.md](spec.md), aprobada el 2026-11-04. Aquí van las decisiones técnicas; el comportamiento lo fija la spec.

## Dependencias y esquema
No hay dependencias nuevas.

Migración `00012_gamificacion.sql`, solo aditiva:

- **`point_ledger`**: el registro inmutable.
  - Columnas:
    - `id uuid` como clave primaria;
    - `seq bigint` identidad única, que sirve de cursor;
    - `user_id`;
    - `xp int` y `coins int`;
    - `reason`, con `close`, `reopen` o `adjust`;
    - `work_item_id`, que puede ser nulo;
    - `event_id`, que puede ser nulo: el evento que lo causó;
    - `reverses_event_id`, que puede ser nulo: en una reapertura, el cierre que compensa;
    - `note`, que puede ser nulo: el motivo de un ajuste;
    - `actor_id`;
    - `created_at`.
  - `UNIQUE (event_id, user_id)` hace que un evento repetido no pague dos veces (RF-12).
  - Un trigger rechaza `UPDATE` y `DELETE` (RF-2).
  - Índices por `(user_id, seq)` y por `work_item_id`.
- **`point_values`**: la tabla de puntos.
  - Columnas: `size_id`, que puede ser nulo y entonces es «sin talla»; `xp`; `coins`. Los dos últimos van de 0 a 1000 con `CHECK`.
  - Un índice único sobre `size_id` y otro único parcial para la fila nula.
  - Una talla sin fila vale 0 XP y 0 monedas. Así una talla nueva no necesita ningún trigger (RF-25).
  - Se siembra RF-27 buscando las tallas por nombre normalizado.
- **`levels`**:
  - Columnas: `id`, `name` y `min_xp`, este último único y mayor o igual que 0.
  - Se siembran los 10 niveles de RF-34.
  - El nivel con `min_xp = 0` se protege en el servicio.
- **Arranque en 0 (RF-53):** la migración inserta el consumidor `gamification` en `event_consumers` con el `max(id)` de `domain_events`. Así nunca procesa eventos anteriores.

## Eventos
- `work_item.closed` añade `size_id` al payload. Es un campo nuevo; los anteriores no cambian.
- El consumidor lee `size_id` del payload. Si falta, porque el evento lo emitió una versión anterior, usa la talla actual del work item.
- Los asignados salen de `assignee_ids`.

## Backend (`internal/gamification`)
- **Consumidor** registrado en el dispatcher con el nombre `gamification`. Cada evento se procesa en una sola transacción.
  - **closed:**
    - si el proyecto es plantilla, no hay asignados o el valor es 0/0, no hace nada (RF-8 a RF-10);
    - si no, por cada asignado inserta una fila `close` con `ON CONFLICT DO NOTHING` (RF-6, RF-7, RF-11 a RF-13).
  - **reopened:**
    - busca el último `event_id` de un cierre con filas para el work item que aún no tenga compensación (`reverses_event_id`);
    - inserta las filas contrarias (RF-14, RF-16, RF-54).
  - Sin último cierre, no hace nada. El cambio entre dos estados cerrados ya no emite eventos (RF-17).
- **Permiso nuevo:** `gamification:manage`, solo para admin. Va en la matriz y en `matrix_work_test.go`.
- **Endpoints:**
  - `GET /api/me/points`: saldo, nivel, siguiente nivel y `last_seq` (RF-37, RF-38).
  - `GET /api/me/points/movements?page=&after_seq=`: historial de 50 en 50, con la clave y el título del work item (RF-39). `after_seq` sirve para el aviso.
  - `GET /api/gamification/users/{id}/points` y `.../movements`: lo mismo para otra persona, con `gamification:manage` (RF-40).
  - `POST /api/gamification/users/{id}/adjustments`: el ajuste, con `{xp, coins, note}` (RF-18 a RF-23).
    - Bloquea el saldo de la persona con `pg_advisory_xact_lock` antes de comprobar el mínimo 0.
    - Audita en la misma transacción.
  - `GET /api/gamification/values` (lectura con sesión) y `PUT /api/gamification/values` (`gamification:manage`): la tabla completa, con las tallas activas y las retiradas que tengan valor (RF-24 a RF-28). Se audita.
  - `GET /api/gamification/levels` (lectura con sesión), y `POST`, `PATCH /{id}` y `DELETE /{id}` (`gamification:manage`) (RF-30 a RF-33, RF-36).
    - Un `min_xp` repetido responde `level_conflict`.
    - Tocar el mínimo del primer nivel responde `level_locked`.
    - Se audita.
  - `GET /api/gamification/teams/{id}/ranking?period=week|month|all&today=` (RF-43 a RF-52).
    - Miembros activos del equipo y de sus subequipos (`TeamDescendantIDs`).
    - Se suma el XP del periodo y se calcula el nivel con el XP total.
    - Si quien consulta no es miembro directo y no tiene `gamification:manage`, responde `forbidden`.
- **Nivel:** se calcula en Go a partir de la lista de niveles. No se guarda (RF-5, RF-29, RF-35).
- **Tests:**
  - apitest del ciclo cerrar, reabrir y volver a cerrar con otros asignados;
  - un evento repetido;
  - una plantilla;
  - un cierre sin asignados;
  - la tabla cambiada entre el cierre y la reapertura;
  - una talla retirada;
  - los ajustes con sus errores y su auditoría;
  - los niveles y sus errores;
  - el ranking por periodo, con inactivos y fuera del equipo;
  - el arranque en 0;
  - la inmutabilidad del registro;
  - filas en la matriz.

## Frontend
- **API:** `api/gamification.ts`.
- **Mi panel:** `ProfileCard` muestra el nivel, el XP, las monedas y la barra hacia el siguiente nivel, o «Nivel máximo» (RF-37, RF-38).
- **Mi cuenta:** una sección «Mis puntos» con el mismo resumen y el historial paginado (`features/gamification/PointsHistory.vue`) (RF-39).
- **Ranking:** la página `/ranking` en la navegación de trabajo. Lleva un selector de equipo (los equipos directos de `auth.me`) y otro de periodo (Semana, Mes o Siempre, con Mes por defecto) (RF-43 a RF-52).
- **Administración:** la página `/admin/gamification` en Configuración, con `gamification:manage`. Tiene tres secciones:
  - la tabla de puntos;
  - los niveles;
  - una persona: se elige, se ve su resumen y su historial, y se le hace un ajuste.
- **Aviso (RF-41, RF-42):**
  - `features/gamification/PointsNotice.vue` va en el layout;
  - consulta `/api/me/points` cada 5 s mientras la pestaña está visible;
  - si `last_seq` crece, pide los movimientos con `after_seq`, suma los de tipo `close` y muestra «+N XP · +M monedas»;
  - si el número de nivel sube, añade el nombre del nivel nuevo;
  - la primera lectura solo fija el cursor.
  - El aviso usa `aria-live="polite"` y desaparece a los 6 s.
- **Textos:** `gamification.*` y `nav.ranking` y `nav.gamification` en `es.json`.
- **Vitest:** tarjeta y barra, nivel máximo, historial, ranking con periodo y sin equipos, administración (tabla, niveles, ajuste y errores), y el aviso con subida de nivel.
