# Spec 022 — Tareas sin proyecto: para una persona o para un equipo

**Estado:** Aprobada (2026-11-09) («si dale»).

**Ajustes al implementar (2026-11-09):** la descripción se escribe en la ficha que se abre al crear la tarea (RF-2), y el líder de una persona también ve y edita sus tareas sin proyecto, como ya pedían el cronograma y el panel de esa persona (RF-14, RF-15).

**Diseño de referencia:** lienzo «Kairos — Tareas sin proyecto y tareas de equipo» (tres pantallas: Nueva tarea, Mi panel y Mi equipo).

## Contexto y objetivo
Hoy toda tarea pertenece a un proyecto. Pero mucho trabajo del día a día no encaja en ninguno:
- una petición puntual a una persona;
- una incidencia que tiene que atender cualquiera del equipo;
- algo que tiene que hacer cada miembro, como una formación o una firma.

Para registrarlo hoy hay que inventar un proyecto, y eso ensucia el Portafolio, el Seguimiento y las métricas de avance.

Esta spec permite crear **tareas sin proyecto**:
- **para una persona;**
- **para un equipo**, en uno de dos modos:
  - **«Una persona la toma»:** la primera persona que la reclama se la queda;
  - **«Cada persona la hace»:** cada miembro recibe la suya y el líder ve el avance del equipo.

Son tareas como las demás: tienen estados, comentarios, panel lateral y puntos al cerrarlas, y aparecen en Mi panel y en el cronograma por persona.

## Usuarios / actores
- **Cualquier usuario que puede tener tareas** (no solo lectura): crea tareas sin proyecto para sí mismo, y toma o devuelve las de sus equipos.
- **Líder:** además, asigna tareas sin proyecto a las personas cuyo panel puede ver y crea tareas para los equipos que lidera y sus subequipos (spec 013).
- **Administrador:** asigna tareas sin proyecto a cualquier usuario activo y crea tareas para cualquier equipo.

## Conceptos
- **Tarea sin proyecto:** tarea que no pertenece a ningún proyecto. Tiene clave, título, descripción, estado, prioridad, peso, fecha de fin opcional y comentarios, como cualquier tarea.
- **Tarea personal:** tarea sin proyecto con una persona asignada desde el principio.
- **Tarea de equipo, «Una persona la toma»:** tarea sin proyecto dirigida a un equipo que nadie tiene hasta que alguien la toma.
- **Tarea de equipo, «Cada persona la hace»:** tarea de equipo que se reparte en una tarea por cada miembro. La tarea del equipo solo agrupa esas tareas y muestra su avance.
- **Miembros de un equipo:** sus miembros y los de sus subequipos, activos y que pueden tener tareas. No cuenta quien solo tiene el rol `viewer`.
- **Creador:** quien creó la tarea sin proyecto.

## Historias de usuario
- H1: Como persona quiero apuntarme tareas que no son de ningún proyecto para tener todo mi trabajo en Mi panel.
- H2: Como líder quiero asignar una tarea suelta a alguien de mi equipo sin crear un proyecto para ello.
- H3: Como líder quiero publicar una tarea para mi equipo para que la haga quien pueda.
- H4: Como líder quiero pedir algo a cada persona de mi equipo y ver cuántas lo han hecho.
- H5: Como persona del equipo quiero ver qué tareas del equipo están sin tomar y quedarme con una.

## Requisitos funcionales (criterios de aceptación en EARS)

### Crear
- RF-1: EL SISTEMA ofrecerá «Nueva tarea» en Mi panel y en Mi equipo para crear una tarea sin proyecto.
- RF-2: EL SISTEMA pedirá al crear una tarea sin proyecto: título obligatorio y, opcionales, fecha de fin, peso y prioridad; la descripción se escribe en la ficha de la tarea, que se abre al crearla.
- RF-3: EL SISTEMA pedirá elegir el destino de la tarea: «Una persona» o «Un equipo».
- RF-4: EL SISTEMA ofrecerá como persona destino al propio usuario y a las personas activas cuyo panel puede ver (spec 013).
- RF-5: EL SISTEMA ofrecerá como equipo destino, al líder, los equipos que lidera y sus subequipos, y al administrador, todos los equipos no archivados.
- RF-6: SI se intenta crear una tarea para una persona o un equipo fuera de los permitidos (RF-4, RF-5), ENTONCES EL SISTEMA la rechazará como si la persona o el equipo no existieran.
- RF-7: SI el destino es un equipo, ENTONCES EL SISTEMA pedirá elegir el modo: «Una persona la toma» o «Cada persona la hace».
- RF-8: SI se elige «Cada persona la hace» para un equipo sin miembros, ENTONCES EL SISTEMA rechazará la tarea e indicará que el equipo no tiene a nadie.
- RF-9: CUANDO se cree una tarea «Cada persona la hace», EL SISTEMA creará una tarea para cada miembro del equipo, con el mismo título, descripción, fecha, peso y prioridad, asignada a esa persona.
- RF-10: EL SISTEMA dará a cada tarea sin proyecto una clave única, como al resto de tareas.
- RF-11: EL SISTEMA registrará en la auditoría la creación de cada tarea sin proyecto, su asignación, su toma, su devolución y su paso a un proyecto.

### Trabajar en ellas
- RF-12: EL SISTEMA aplicará a las tareas sin proyecto los mismos estados, flujo, comentarios, panel lateral y menciones que a las tareas de proyecto.
- RF-13: CUANDO se cierre una tarea sin proyecto asignada, EL SISTEMA dará a la persona asignada los puntos que da cerrar una tarea (spec 014).
- RF-14: EL SISTEMA permitirá ver y comentar una tarea sin proyecto a su creador, a sus asignados, a los líderes de sus asignados, a los miembros del equipo destino, a los líderes de ese equipo y al administrador.
- RF-15: EL SISTEMA permitirá editar y cambiar de estado una tarea sin proyecto a su creador, a sus asignados, a los líderes de sus asignados, a los líderes del equipo destino y al administrador.
- RF-16: EL SISTEMA permitirá borrar una tarea sin proyecto a su creador y al administrador.
- RF-17: SI alguien sin permiso pide una tarea sin proyecto, ENTONCES EL SISTEMA responderá como si no existiera.

### «Una persona la toma»
- RF-18: MIENTRAS una tarea «Una persona la toma» no tenga a nadie, EL SISTEMA la mostrará en Mi panel de cada miembro del equipo, en la sección «Para tomar en tu equipo».
- RF-19: CUANDO un miembro pulse «La tomo», EL SISTEMA le asignará la tarea.
- RF-20: SI dos personas toman la misma tarea casi a la vez, ENTONCES EL SISTEMA se la asignará solo a la primera e indicará a la segunda que ya la tomó otra persona.
- RF-21: CUANDO una tarea esté tomada, EL SISTEMA dejará de mostrarla en «Para tomar» y la mostrará en Mi panel de quien la tomó, marcada como del equipo.
- RF-22: MIENTRAS una tarea tomada siga abierta, EL SISTEMA permitirá a quien la tomó devolverla al equipo, y entonces volverá a «Para tomar».
- RF-23: EL SISTEMA permitirá al líder del equipo y al administrador asignar una tarea «Una persona la toma» a un miembro del equipo, la haya tomado alguien o no.

### «Cada persona la hace»
- RF-24: EL SISTEMA mostrará en cada tarea de un miembro el avance del equipo: cuántas de las tareas del grupo están cerradas.
- RF-25: CUANDO alguien entre en el equipo mientras la tarea del equipo sigue abierta, EL SISTEMA creará también su tarea.
- RF-26: CUANDO alguien salga del equipo, EL SISTEMA mantendrá su tarea tal como esté, y el líder podrá cerrarla, reasignarla o borrarla.
- RF-27: CUANDO todas las tareas del grupo estén cerradas, EL SISTEMA dará por cerrada la tarea del equipo.
- RF-28: SI se borra la tarea del equipo, ENTONCES EL SISTEMA borrará las tareas de los miembros que sigan abiertas y conservará las cerradas, sin quitar sus puntos.

### Mi panel
- RF-29: EL SISTEMA mostrará en el Kanban de Mi panel las tareas sin proyecto asignadas a la persona, junto a las de proyecto.
- RF-30: EL SISTEMA marcará cada tarjeta sin proyecto con «Sin proyecto» o, si es de un equipo, con el nombre del equipo y el modo.
- RF-31: EL SISTEMA permitirá filtrar el Kanban de Mi panel por origen: todas, de proyectos o sin proyecto.
- RF-32: EL SISTEMA no contará las tareas sin proyecto en la lista de proyectos ni en las cifras por proyecto de Mi panel (spec 013).

### Mi equipo
- RF-33: EL SISTEMA mostrará en Mi equipo la pestaña «Tareas del equipo» con las tareas de equipo de los equipos visibles para el usuario.
- RF-34: EL SISTEMA mostrará de cada tarea «Una persona la toma» su clave, título, equipo, fecha de fin y quién la tiene, o «Sin tomar» con los días que lleva así.
- RF-35: EL SISTEMA mostrará de cada tarea «Cada persona la hace» su avance («N de M») y la lista de miembros con su tarea hecha o pendiente.
- RF-36: EL SISTEMA incluirá las tareas sin proyecto con fecha de fin en el cronograma por persona de Mi equipo (spec 021), con su propio color en la leyenda.

### Pasar a un proyecto
- RF-37: EL SISTEMA permitirá mover una tarea personal o «Una persona la toma» a un proyecto activo, a quien pueda crear tareas en ese proyecto y editar la tarea.
- RF-38: CUANDO una tarea pase a un proyecto, EL SISTEMA le dará la clave del proyecto, conservará sus comentarios, su historial y sus asignados, y dejará de mostrarla como sin proyecto.
- RF-39: SI se intenta mover a un proyecto una tarea «Cada persona la hace» o una de sus tareas de miembro, ENTONCES EL SISTEMA lo rechazará.

### Fuera de los proyectos
- RF-40: EL SISTEMA no mostrará las tareas sin proyecto en Proyectos, Portafolio, Seguimiento ni Objetivos, ni las contará en sus cifras.

## Requisitos no funcionales
- **Rendimiento** (entorno local): crear una tarea «Cada persona la hace» para un equipo de 200 personas responde en < 1 s; Mi panel sigue cargando en el mismo tiempo que hoy con 200 tareas sin proyecto.
- **Permisos:** los permisos nuevos se añaden a la matriz única y a sus tests.
- **Idioma:** todos los textos en el catálogo de traducciones.

## Casos límite
- **Tarea para uno mismo:** es una tarea personal; el creador es también el asignado.
- **Persona que deja de ser visible para el líder** (cambia de equipo): conserva sus tareas personales; el líder deja de verlas salvo que las haya creado él.
- **Equipo archivado:** sus tareas siguen abiertas y visibles para quien ya las veía; no se pueden crear tareas nuevas para él.
- **Usuario desactivado:** sus tareas personales y tomadas siguen abiertas; el líder puede reasignarlas.
- **La única persona del equipo toma una tarea y la devuelve:** vuelve a «Para tomar».
- **«Cada persona la hace» con el líder como miembro del equipo:** el líder también recibe la suya.
- **Tarea de equipo con fecha pasada y sin tomar:** sale en «Para tomar» como vencida, en rojo.

## Fuera de alcance
- Notificaciones de tareas nuevas para tomar o de tareas asignadas (la spec 017 no notifica asignaciones).
- Tareas recurrentes (cada semana, cada mes).
- Subtareas, dependencias e hitos en tareas sin proyecto.
- Sacar de un proyecto una tarea que ya está en uno.
- Tareas sin proyecto para un grupo libre de personas que no es un equipo.
- Plantillas de tareas sin proyecto.

## Criterios de finalización
- Todos los RF con tests en verde: API (permisos en el test de matriz, toma simultánea, reparto y altas en el equipo, paso a un proyecto, que no aparezcan en Proyectos ni en Portafolio) y frontend (crear, tomar, devolver, filtro en Mi panel, pestaña de Mi equipo).
- Demo manual:
  1. Como miembro, crearse una tarea sin proyecto y cerrarla: gana sus puntos.
  2. Como líder, asignar una tarea a una persona de su equipo y verla en su Mi panel.
  3. Como líder, publicar una tarea «Una persona la toma»; como miembro, tomarla y devolverla.
  4. Como líder, crear una «Cada persona la hace» y ver el avance subir al cerrar un miembro la suya.
  5. Mover una tarea personal a un proyecto y comprobar que conserva sus comentarios.

## Dudas abiertas
- Ninguna. Decidido con el usuario el 2026-11-09 («Dale»):
  - asignan el líder (a su gente) y el administrador (a cualquiera); cada persona puede crearse las suyas;
  - quien entra en el equipo recibe su tarea de «Cada persona la hace»; quien sale conserva la suya abierta;
  - una tarea sin proyecto se puede pasar a un proyecto.
