# Spec 021 — Cronograma por persona: en el proyecto y en Mi equipo

**Estado:** Aprobada (2026-11-09) («Dale»).

## Contexto y objetivo
Hoy la pestaña Equipo del proyecto dice cuánta carga tiene cada persona, pero no **cuándo** la
tiene: no se ve en qué fechas está ocupada ni a partir de qué día queda libre para otra cosa. Y
el líder de un equipo no tiene ninguna pantalla que reúna a su gente: para saber cómo va cada
persona tiene que entrar en su panel uno a uno.

Esta spec añade un **cronograma por persona**: una fila por persona con las barras de sus tareas
abiertas y la fecha en que queda libre. Aparece en dos sitios:
- en la pestaña **Equipo** del proyecto, con las tareas de ese proyecto;
- en una pantalla nueva, **Mi equipo**, donde el líder ve el listado de su gente y su cronograma
  con las tareas de todos sus proyectos.

Desde el nombre de cada persona se abre su panel (spec 013).

## Usuarios / actores
- **Quien gestiona el equipo del proyecto** (spec 008, RF-33): ve el cronograma por persona en la pestaña Equipo.
- **Líder:** ve Mi equipo con los miembros y observadores de los equipos que lidera y de sus subequipos, es decir, las mismas personas cuyo panel puede ver (spec 013, RF-37 y siguientes).
- **Administrador:** ve Mi equipo con todos los usuarios activos.
- Nadie más ve Mi equipo.

## Conceptos
- **Tarea con fechas:** tarea abierta asignada a la persona que tiene fecha de fin. Si no tiene fecha de inicio, su barra ocupa solo el día de fin.
- **Libre desde:** el día siguiente a la fecha de fin más tardía de las tareas abiertas con fechas de la persona. Si alguna está vencida, cuenta como que termina hoy.
- **Ámbito del cronograma:** en el proyecto, solo sus tareas (sin las de subproyectos); en Mi equipo, las tareas de todos los proyectos salvo las plantillas.

## Historias de usuario
- H1: Como PM quiero ver en una línea de tiempo cuándo está ocupada cada persona del proyecto para saber cuándo puedo darle más trabajo.
- H2: Como líder quiero ver en una pantalla el listado de mi equipo y su cronograma para repartir el trabajo entre proyectos.
- H3: Como líder quiero pasar del cronograma al panel de una persona con un clic.

## Requisitos funcionales (criterios de aceptación en EARS)

### Cronograma por persona (común)
- RF-1: EL SISTEMA mostrará una fila por persona y, en ella, una barra por cada tarea con fechas, de su fecha de inicio a su fecha de fin.
- RF-2: EL SISTEMA mostrará junto al nombre de cada persona su fecha «Libre desde».
- RF-3: SI una persona no tiene tareas con fechas, ENTONCES EL SISTEMA mostrará «Libre» en lugar de la fecha y dejará su fila vacía.
- RF-4: EL SISTEMA mostrará junto al nombre el número de tareas abiertas sin fecha de fin de esa persona, que no se dibujan.
- RF-5: EL SISTEMA marcará en rojo las barras de las tareas vencidas.
- RF-6: EL SISTEMA marcará con una línea el día de hoy.
- RF-7: EL SISTEMA ordenará las filas por «Libre desde», primero quien antes queda libre, y las personas libres al principio.
- RF-8: CUANDO el usuario pase el puntero o el foco por una barra, EL SISTEMA mostrará la clave, el título, el proyecto, las fechas y el avance de la tarea.
- RF-9: CUANDO el usuario pulse una barra, EL SISTEMA abrirá la tarea en el panel lateral (spec 005).
- RF-10: EL SISTEMA mostrará el cronograma en solo lectura: las barras no se arrastran.
- RF-11: EL SISTEMA permitirá cambiar la escala entre días y semanas.
- RF-12: EL SISTEMA abrirá el cronograma centrado en la semana actual, con desplazamiento horizontal hacia el pasado y el futuro.
- RF-13: SI una tarea tiene varios asignados, ENTONCES EL SISTEMA la dibujará en la fila de cada uno.
- RF-14: DONDE el usuario pueda ver el panel de una persona (spec 013), EL SISTEMA mostrará su nombre como enlace a ese panel.
- RF-15: SI el usuario no puede ver el panel de una persona, ENTONCES EL SISTEMA mostrará su nombre sin enlace.

### En la pestaña Equipo del proyecto
- RF-16: EL SISTEMA mostrará en la pestaña Equipo, debajo del listado de miembros, el cronograma por persona del proyecto.
- RF-17: EL SISTEMA incluirá en el cronograma del proyecto a los miembros con papel PM, líder técnico o miembro, y no a los observadores ni a quien solo tiene el rol global de lectura (`viewer`).
- RF-18: EL SISTEMA dibujará en el cronograma del proyecto solo las tareas de ese proyecto.

### Mi equipo
- RF-19: DONDE el usuario sea administrador o líder, EL SISTEMA mostrará en el menú la entrada «Mi equipo».
- RF-20: SI un usuario sin esos permisos abre la dirección de Mi equipo, ENTONCES EL SISTEMA no le mostrará ningún dato y le indicará que no tiene acceso.
- RF-21: EL SISTEMA mostrará en Mi equipo el listado de las personas que el usuario puede ver (Usuarios/actores), sin incluirse a sí mismo.
- RF-22: EL SISTEMA mostrará en el listado, por persona: nombre, equipos, capacidad semanal, carga total, número de tareas abiertas y «Libre desde».
- RF-23: SI la carga total de una persona supera su capacidad, ENTONCES EL SISTEMA la marcará como sobrecargada, como en la pestaña Equipo (spec 008, RF-49).
- RF-24: EL SISTEMA mostrará en Mi equipo el cronograma por persona de esas mismas personas, con sus tareas de todos los proyectos salvo plantillas.
- RF-25: EL SISTEMA distinguirá en el cronograma de Mi equipo las barras de cada proyecto y mostrará una leyenda con los proyectos.
- RF-26: DONDE el usuario lidere más de un equipo, o sea administrador, EL SISTEMA permitirá filtrar el listado y el cronograma por equipo.
- RF-27: SI el usuario no tiene a nadie en su ámbito, ENTONCES EL SISTEMA mostrará un aviso de que su equipo no tiene personas.

## Requisitos no funcionales
- **Rendimiento** (entorno local, 50 personas con 2 000 tareas abiertas en total): Mi equipo carga en < 1 s; la pestaña Equipo con su cronograma, en < 500 ms.
- **Accesibilidad:** cada barra se alcanza con el teclado y su detalle (RF-8) se lee sin ratón; los colores de proyecto llevan además la leyenda.
- **Permisos:** no se añade ningún permiso; Mi equipo usa los de ver el panel de otra persona (spec 013).
- **Idioma:** todos los textos en el catálogo de traducciones.

## Casos límite
- **Tarea con fecha de inicio posterior a la de fin:** se dibuja solo el día de fin.
- **Tarea vencida hace meses:** su barra se dibuja en el pasado, en rojo, y la persona cuenta como ocupada hasta hoy.
- **Persona desactivada:** no aparece en Mi equipo; en el proyecto aparece si sigue siendo miembro, sin enlace a su panel.
- **Persona en dos equipos del líder:** aparece una sola vez, con ambos equipos.
- **Tarea de un proyecto en pausa o cerrado:** cuenta mientras la tarea siga abierta.
- **Más de 8 proyectos en Mi equipo:** los que pasan de 8 comparten el color «Otros» en la leyenda.

## Fuera de alcance
- Mover o redimensionar barras para cambiar fechas o reasignar desde estos cronogramas.
- Vacaciones, festivos o capacidad por día.
- Calcular «Libre desde» según capacidad o peso (solo cuenta la última fecha de fin).
- Mi equipo para el PM de un proyecto que no es líder.
- Exportar el cronograma.

## Criterios de finalización
- Todos los RF con tests en verde: API (contenido y permisos de Mi equipo en el test de matriz) y frontend (cronograma, orden, «Libre desde», enlaces según permisos, filtro por equipo).
- Demo manual:
  1. Como PM, abrir la pestaña Equipo de un proyecto y ver quién queda libre antes.
  2. Como líder, abrir Mi equipo, ver el listado y el cronograma, y pulsar el nombre de una persona para llegar a su panel.
  3. Como miembro, abrir la dirección de Mi equipo y no ver datos.

## Decisiones (2026-11-09)
- **Administrador:** también tiene Mi equipo, con todos los usuarios activos menos él mismo.
- **Observadores de los equipos del líder:** aparecen en Mi equipo, como en el panel de otra persona (spec 013).

## Dudas abiertas
- Ninguna.
