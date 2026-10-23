# ADR 0002 — Eventos de dominio con outbox en PostgreSQL

**Estado:** aceptada (2026-10-23) · **Spec:** [004-proyectos](../specs/004-proyectos/spec.md)

## Contexto
La gamificación (spec futura) da XP y monedas cuando se cierra un work item o se cumple un hito.
Cada punto queda en un ledger inmutable, así que un evento perdido es un punto que no se da y un
evento inventado (por un cambio que luego se deshizo) es un punto que no se puede quitar.
La spec 004 exige registrar el evento en la misma transacción que el cambio y entregarlo al
menos una vez, en orden (RF-120, RF-121).

## Decisión
- **Outbox en la misma BD:** `events.Emit` inserta en `domain_events` con las queries de la
  transacción del cambio, como `audit.Record`. Si el cambio se deshace, no queda evento.
- **Orden de confirmación = orden de id:** `Emit` toma antes `pg_advisory_xact_lock`. Así una
  transacción no puede confirmar un id mayor mientras otra con un id menor sigue abierta, y un
  lector que avanza por cursor nunca se salta un evento.
- **Entrega por cursor:** `events.Dispatcher` (goroutine iniciada en `main`) lee cada segundo los
  eventos posteriores al cursor de cada consumidor (`event_consumers`). Avanza el cursor después
  de que el consumidor termina bien; si falla, reintenta el mismo evento en la siguiente vuelta.
- **Idempotencia a cargo del consumidor:** cada evento lleva un `event_id` único; un consumidor
  que pueda recibir un evento dos veces lo usa para ignorar el duplicado.
- Los eventos no tienen claves foráneas: sobreviven al borrado de lo que describen.
- Se emiten también las reaperturas (`work_item.reopened`, `milestone.reopened`) para que la
  gamificación pueda compensar.

## Alternativas descartadas
- **Llamar al consumidor dentro de la transacción:** acopla la gamificación a cada cambio de
  estado y un fallo suyo tumbaría el cambio.
- **Emitir después del commit (en memoria o a un broker):** se pierden eventos si la API cae
  entre el commit y la emisión.
- **LISTEN/NOTIFY sin tabla:** no guarda nada si no hay oyente conectado.
- **Broker externo (Kafka, NATS):** una pieza más de infraestructura para unas decenas de eventos
  por hora.

## Consecuencias
- El bloqueo serializa las transacciones que emiten eventos. Con el volumen previsto es
  despreciable; con mucha más carga se pasaría a cursores por `xid8`/`pg_snapshot`.
- Con varias instancias de API, varios dispatchers podrían entregar el mismo evento; la entrega
  sigue siendo «al menos una vez» y el `event_id` permite deduplicar.
- `domain_events` crece sin purga. La spec de gamificación decidirá la retención.
