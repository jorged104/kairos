# Tareas 013 — Mi panel

- [x] **T1 — API de Mi panel**
  - RF: RF-3 a RF-6, RF-10 a RF-14, RF-23 a RF-25, RF-28 a RF-34, RF-36
  - Hecho cuando: hay apitest de tareas, papeles, series, carga y aislamiento entre usuarios, y una fila en la matriz.
- [x] **T2 — Frontend: inicio, perfil y proyectos**
  - RF: RF-1, RF-2, RF-5 a RF-7, RF-23 a RF-27
  - Hecho cuando: hay Vitest de la redirección, de la tarjeta de perfil y de la lista de proyectos.
- [x] **T3 — Frontend: Kanban de mis tareas**
  - RF: RF-8 a RF-22
  - Hecho cuando: hay Vitest de columnas, orden, arrastre con flujo, menú, error y tope.
- [x] **T4 — Frontend: gráficas**
  - RF: RF-28 a RF-35
  - Hecho cuando: hay Vitest de las tres gráficas, «Sin datos», «Otros» y las tablas de datos.
- [ ] **T5 — Prueba manual** (tú)
  - Hecho cuando: la demo de los criterios de finalización funciona en la previsualización.

## Cambio de 2026-11-06: panel de otra persona (RF-37 a RF-51)
- [x] T9. Permisos `dashboard:others:all` (admin) y `dashboard:others:team` (líder) en la matriz y sus filas en `matrix_work_test.go`.
- [x] T10. API: `GET /api/me/dashboard/people` y `GET /api/users/{id}/dashboard` (solo lectura, 404 si no se puede ver), con `TestDashboardOfOthers`.
- [x] T11. Frontend: selector y aviso en Mi panel, títulos en tercera persona, Kanban sin mover, enlace a la ficha y «Ver panel» en Usuarios; Vitest.
- [ ] T12. Demo manual (usuario): pasos 7 y 8 de los criterios de finalización.
