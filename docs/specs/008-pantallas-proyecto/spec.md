# Spec 008 — Pantallas de proyecto: resumen, tabla, kanban, cronograma y equipo

**Estado:** Aprobada (2026-11-02) con los valores propuestos en las dudas («Dale escríbela»).
**Diseño de referencia:** lienzo «Kairos — Pantallas de proyecto» (cinco pantallas: Resumen, Tabla, Kanban, Cronograma y Equipo).

## Contexto y objetivo
Hoy un proyecto se ve como un árbol de tareas editable y una ficha de datos. Quien dirige el
proyecto no tiene una vista de un vistazo de cómo va, quien lo gestiona no tiene dónde repartir el
trabajo según la carga de cada persona, y el equipo solo puede trabajar en forma de lista. Esta
spec convierte la ficha del proyecto en cinco vistas: un **resumen gerencial**, una **tabla** con
filtros dinámicos que se guardan como **vistas**, un **kanban**, un **cronograma** y una pantalla
de **equipo** con papeles por proyecto, capacidad y sugerencias de asignación.

## Usuarios / actores
- **Dirección y cualquier usuario con acceso al proyecto:** consulta el resumen y las vistas.
- **PM del proyecto:** persona con el papel PM en ese proyecto; lo gestiona aunque su rol global sea `member`.
- **Gestor del proyecto** (spec 004): `admin`, `lead` con el proyecto en su ámbito o responsable del proyecto.
- **Miembros del proyecto:** trabajan las tareas, crean y guardan vistas personales.
- **Administrador y líder del equipo de la persona:** fijan su capacidad semanal.

## Conceptos
- **Miembro del proyecto:** una persona con un papel en el proyecto. Lo son el responsable del proyecto, los participantes por equipo (spec 004) y las personas añadidas expresamente.
- **Papel en el proyecto:** PM, líder técnico, miembro u observador. Concede o limita permisos solo en ese proyecto.
- **Capacidad semanal:** puntos de peso que una persona puede llevar abiertos a la vez, la misma en todos los proyectos.
- **Carga:** suma del peso de las tareas abiertas asignadas a una persona, en este proyecto y en total.
- **Avance del proyecto:** media del avance de todas sus tareas ponderada por su peso, con las cerradas al 100 % (las de peso 0 no cuentan).
- **Avance esperado del proyecto:** el porcentaje del plazo del proyecto (de su fecha de inicio a su fecha de fin) que ya ha pasado.
- **Vista guardada:** un nombre más una combinación de filtros, orden y agrupación, utilizable en la tabla, el kanban y el cronograma. Puede ser personal o compartida con el proyecto.
- **Tarea en riesgo:** abierta y vencida, bloqueada o retrasada (spec 007, RF-13).

## Historias de usuario
- H1: Como directivo quiero ver de un vistazo el avance frente al plan, los hitos y los riesgos de un proyecto para decidir dónde intervenir.
- H2: Como PM quiero dar papeles en el proyecto para que cada persona tenga los permisos que le tocan sin cambiar su rol global.
- H3: Como PM quiero ver la carga de cada persona frente a su capacidad y asignar las tareas pendientes a quien tiene hueco.
- H4: Como miembro quiero filtrar las tareas por cualquier propiedad y guardar esa combinación como vista para volver a ella con un clic.
- H5: Como PM quiero compartir vistas con todo el proyecto para que el equipo trabaje sobre los mismos cortes.
- H6: Como miembro quiero mover tarjetas en un kanban para cambiar el estado de mis tareas.
- H7: Como PM quiero ver las tareas en un cronograma y arrastrarlas para cambiar sus fechas.

## Requisitos funcionales (criterios de aceptación en EARS)

### Navegación
- RF-1: EL SISTEMA mostrará en la ficha de un proyecto las pestañas Resumen, Tabla, Kanban, Cronograma y Equipo, en ese orden, seguidas de las pestañas Hitos, Fases y Datos que ya existían (spec 004).
- RF-2: EL SISTEMA abrirá un proyecto en la pestaña Resumen.
- RF-3: EL SISTEMA guardará en la dirección de la página la pestaña, la vista y los filtros activos, de modo que el enlace abra lo mismo.
- RF-4: MIENTRAS el usuario no pueda gestionar el equipo del proyecto (RF-33), EL SISTEMA no le mostrará la pestaña Equipo.
- RF-5: EL SISTEMA mantendrá la apertura de una tarea en el panel lateral (spec 005) desde la tabla, el kanban y el cronograma.

### Resumen
- RF-6 (sustituido por la spec 009, RF-44): EL SISTEMA mostrará el avance del proyecto, su avance esperado hoy y la diferencia en puntos.
- RF-7 (sustituido por la spec 009, RF-44): SI el proyecto no tiene fecha de inicio o de fin, ENTONCES EL SISTEMA mostrará «Sin plazo» en lugar del avance esperado y la diferencia.
- RF-8: EL SISTEMA mostrará el número de tareas por estado y el total de cerradas.
- RF-9: EL SISTEMA mostrará el número de tareas abiertas vencidas y el de bloqueadas, cada uno con un enlace a la tabla filtrada por ese criterio.
- RF-10: EL SISTEMA mostrará el próximo hito no cumplido ni cancelado, con su fecha, los días que faltan y su avance.
- RF-11: EL SISTEMA mostrará la lista de hitos del proyecto ordenados por fecha, con su avance y su estado.
- RF-12: EL SISTEMA registrará una vez al día el avance de cada proyecto activo.
- RF-13 (la línea de plan, por la spec 009, RF-46): EL SISTEMA mostrará una gráfica del avance registrado por días frente a una línea de plan que va de 0 % en la fecha de inicio a 100 % en la fecha de fin.
- RF-14: MIENTRAS un proyecto tenga menos de dos días registrados, EL SISTEMA mostrará en la gráfica solo el plan y el punto de hoy.
- RF-15 («retrasada» según la spec 009, RF-47): EL SISTEMA mostrará hasta 10 tareas en riesgo: primero las vencidas (más días de retraso antes), después las bloqueadas y después las retrasadas.
- RF-16: EL SISTEMA mostrará la carga de las 5 personas del proyecto con más carga, comparada con su capacidad.
- RF-17: EL SISTEMA mostrará la salud del proyecto y su explicación, editables por quien gestiona el proyecto (spec 004).

### Filtros dinámicos
- RF-18: EL SISTEMA permitirá filtrar las tareas del proyecto por estado, abierta o cerrada, tipo, prioridad, asignado, etiqueta, talla, hito efectivo, fecha de fin (antes de, después de, vencida), bloqueada y texto.
- RF-19: EL SISTEMA ofrecerá en el filtro de asignado los valores «Sin asignar» y «Yo», este último resuelto como el usuario que mira la vista.
- RF-20: EL SISTEMA combinará los filtros de propiedades distintas con «y» y los varios valores de una misma propiedad con «o».
- RF-21: EL SISTEMA mostrará cada filtro activo como un chip que se quita con un clic.
- RF-22: EL SISTEMA aplicará los mismos filtros a la tabla, el kanban y el cronograma.

### Vistas guardadas
- RF-23: EL SISTEMA ofrecerá en cada proyecto la vista «Todas», sin filtros, que no se puede modificar ni borrar.
- RF-24: EL SISTEMA permitirá a cualquier usuario con acceso al proyecto guardar los filtros, el orden y la agrupación actuales como una vista personal con nombre.
- RF-25: DONDE el usuario pueda gestionar el equipo del proyecto (RF-33), EL SISTEMA le permitirá marcar una vista como compartida con el proyecto.
- RF-26: EL SISTEMA mostrará a cada usuario la vista «Todas», las vistas compartidas del proyecto y sus vistas personales, con el número de tareas de cada una.
- RF-27: CUANDO los filtros actuales difieran de los de la vista activa, EL SISTEMA mostrará «Cambios sin guardar» con las opciones de actualizar la vista o guardarla como nueva.
- RF-28: EL SISTEMA solo dejará actualizar, renombrar o borrar una vista personal a su autor, y una compartida a quien puede gestionar el equipo del proyecto.
- RF-29: SI el nombre de una vista coincide con otra visible para el mismo usuario en el proyecto (ignorando mayúsculas y espacios exteriores), ENTONCES EL SISTEMA rechazará el cambio con un código de conflicto.
- RF-30: SI un usuario intenta tener más de 50 vistas personales en un proyecto, o un proyecto más de 30 compartidas, ENTONCES EL SISTEMA rechazará la nueva vista.
- RF-31: SI una vista filtra por un valor que ya no existe o está retirado (etiqueta, estado, persona, hito), ENTONCES EL SISTEMA aplicará el resto de filtros y marcará ese chip como «no disponible».
- RF-32: EL SISTEMA recordará en el navegador la última vista usada por el usuario en cada proyecto y la abrirá la próxima vez.

### Equipo y papeles
- RF-33: EL SISTEMA permitirá gestionar el equipo de un proyecto (añadir personas, cambiar papeles, quitar) a sus gestores (spec 004) y a sus PM.
- RF-34: EL SISTEMA mostrará como miembros del proyecto a su responsable, a sus participantes por equipo y a las personas añadidas expresamente.
- RF-35: EL SISTEMA dará al responsable del proyecto el papel PM, que no se le puede quitar mientras sea responsable.
- RF-36: EL SISTEMA dará a los participantes por equipo el papel miembro mientras nadie les asigne otro.
- RF-37: SI se intenta añadir a una persona inactiva, dada de baja o con solo el rol global `viewer` con un papel distinto de observador, ENTONCES EL SISTEMA rechazará el cambio.
- RF-38: SI se intenta quitar a un participante por equipo, ENTONCES EL SISTEMA lo rechazará e indicará que se le puede poner el papel observador.
- RF-39: EL SISTEMA concederá a un PM los mismos permisos que al gestor del proyecto (spec 004), solo en ese proyecto.
- RF-40: EL SISTEMA concederá a un líder técnico crear, editar y borrar cualquier tarea del proyecto.
- RF-80: EL SISTEMA no concederá a un líder técnico por su papel gestionar los hitos, las vistas compartidas, los límites del kanban ni el equipo del proyecto.
- RF-41: EL SISTEMA concederá a un miembro crear y editar tareas del proyecto, y borrar las suyas sin hijas (como al participante de la spec 004).
- RF-42: EL SISTEMA limitará a un observador a leer el proyecto y comentar sus tareas (spec 006), aunque sea participante por equipo.
- RF-43: EL SISTEMA no quitará por un papel de proyecto los permisos que da el rol global (`admin`, o `lead` con el proyecto en su ámbito).
- RF-44: EL SISTEMA aplicará los cambios de papel a las peticiones de esa persona dentro de la ventana de permisos de la spec 003 (RF-44 de 003).
- RF-45: EL SISTEMA registrará en la auditoría cada alta, baja y cambio de papel en un proyecto.

### Capacidad y carga
- RF-46: EL SISTEMA guardará de cada usuario una capacidad semanal opcional, entera de 0 a 1000 puntos de peso.
- RF-47: EL SISTEMA permitirá cambiar la capacidad de una persona a un administrador y al líder de un equipo al que pertenece (spec 003).
- RF-48: EL SISTEMA mostrará en la pantalla Equipo, por cada miembro: papel, capacidad, carga en este proyecto, carga total y número de tareas abiertas en el proyecto.
- RF-49: SI la carga total de una persona supera su capacidad, ENTONCES EL SISTEMA marcará su carga como sobrecargada.
- RF-50: SI una persona no tiene capacidad, ENTONCES EL SISTEMA mostrará su carga sin compararla ni marcarla.

### Asignación
- RF-51: EL SISTEMA mostrará en la pantalla Equipo las tareas abiertas del proyecto sin asignar, con su peso, etiquetas y fecha de fin.
- RF-52: EL SISTEMA sugerirá para cada tarea sin asignar hasta 2 miembros con papel PM, líder técnico o miembro: primero los que comparten alguna etiqueta con la tarea y, entre ellos, los de menor carga total respecto a su capacidad.
- RF-53: CUANDO el gestor pulse una sugerencia, EL SISTEMA asignará la tarea a esa persona y actualizará su carga sin recargar la página.
- RF-54: EL SISTEMA permitirá asignar la tarea a cualquier otro miembro desde la misma tarjeta.

### Tabla
- RF-55: MIENTRAS no haya filtros activos, EL SISTEMA mostrará el árbol de tareas editable de la spec 004.
- RF-56: MIENTRAS haya filtros activos, EL SISTEMA mostrará la lista de tareas que los cumplen, editable igual que el árbol.
- RF-57: EL SISTEMA permitirá ordenar la lista por clave, título, estado, prioridad, fecha de fin, avance y peso.
- RF-58: EL SISTEMA permitirá agrupar la lista por hito, estado, asignado o tipo, con el número de tareas de cada grupo.
- RF-59: CUANDO se cree una tarea con la fila rápida mientras hay filtros activos, EL SISTEMA le pondrá los valores de los filtros que tengan un único valor de estado, prioridad, asignado, etiqueta, talla o hito.

### Kanban
- RF-60: EL SISTEMA mostrará una columna por cada estado activo del catálogo usado en el proyecto, en el orden del catálogo.
- RF-61: EL SISTEMA mostrará en cada tarjeta la clave, el título, la talla, las etiquetas, los asignados, la fecha de fin y, si está bloqueada, la marca «Bloqueada».
- RF-62: CUANDO el usuario arrastre una tarjeta a otra columna, EL SISTEMA cambiará el estado de la tarea.
- RF-63: MIENTRAS se arrastra una tarjeta, EL SISTEMA marcará como no disponibles las columnas a las que el flujo del tipo no permite pasar (spec 004, RF-86).
- RF-64: SI el cambio de estado se rechaza (flujo, bloqueo, conflicto de versión o permisos), ENTONCES EL SISTEMA devolverá la tarjeta a su columna y mostrará el motivo.
- RF-65: EL SISTEMA ofrecerá en cada tarjeta la acción «Mover a», con los mismos estados permitidos, como alternativa al arrastre con teclado o en pantallas táctiles.
- RF-66: DONDE quien gestiona el equipo del proyecto (RF-33) haya fijado un límite de tareas para una columna, EL SISTEMA mostrará el número de tareas frente al límite y lo marcará en rojo al superarlo, sin impedir mover tarjetas.
- RF-67: EL SISTEMA permitirá agrupar el kanban en carriles por asignado, hito o prioridad.
- RF-68: EL SISTEMA permitirá ocultar las tareas cerradas hace más de 14 días.
- RF-69: CUANDO se cree una tarea desde el pie de una columna, EL SISTEMA la creará con el estado de esa columna.

### Cronograma
- RF-70: EL SISTEMA mostrará las tareas con fecha de inicio y de fin como barras sobre una línea de tiempo, agrupadas por hito efectivo, con un grupo «Sin hito».
- RF-71 (colores según la spec 009, RF-48): EL SISTEMA mostrará en cada barra el avance de la tarea y la marcará en rojo si está vencida, bloqueada o retrasada.
- RF-72: EL SISTEMA mostrará cada hito como un rombo en su fecha y una línea vertical en la fecha de hoy.
- ~~RF-73: EL SISTEMA permitirá cambiar la escala entre semanas, meses y trimestres.~~ Sustituido por RF-81 (2026-11-03).
- RF-81: EL SISTEMA permitirá cambiar la escala entre días, semanas, meses y trimestres.
- RF-74: EL SISTEMA listará al final, en «Sin fechas», las tareas a las que les falta la fecha de inicio o la de fin.
- RF-75: CUANDO el usuario arrastre una barra, EL SISTEMA moverá sus fechas de inicio y fin conservando su duración.
- RF-76: CUANDO el usuario arrastre un extremo de una barra, EL SISTEMA cambiará solo la fecha de ese extremo.
- RF-77: SI el arrastre dejaría la fecha de fin antes que la de inicio, ENTONCES EL SISTEMA no permitirá soltar la barra en esa posición.
- RF-78: SI el cambio de fechas se rechaza, ENTONCES EL SISTEMA devolverá la barra a su posición y mostrará el motivo.
- RF-79: DONDE el usuario active «Dependencias», EL SISTEMA dibujará las dependencias `blocks` y `precedes` entre tareas visibles y marcará las `precedes` cuyas fechas se incumplen (spec 004, RF-96).
- RF-82: MIENTRAS el usuario arrastra una barra o uno de sus extremos, EL SISTEMA mostrará junto al puntero la fecha de inicio y la de fin con que quedaría la tarea y su duración en días.
- RF-83: EL SISTEMA mostrará las tareas de cada grupo en su jerarquía, con cada subtarea sangrada bajo su tarea padre.
- RF-84: SI una tarea padre no tiene fechas y alguna de sus subtareas del grupo sí, ENTONCES EL SISTEMA mostrará su fila sin barra arrastrable, con una marca del intervalo que cubren sus subtareas.
- RF-85: EL SISTEMA permitirá plegar y desplegar las subtareas de una tarea padre.
- RF-86: CUANDO el usuario cree una tarea desde el pie de un grupo, EL SISTEMA la creará con inicio hoy y fin seis días después.
- RF-87: CUANDO el usuario cree una tarea desde el pie del grupo de un hito, EL SISTEMA la vinculará a ese hito (ligar es editar la tarea, spec 004, RF-161).
- RF-88: CUANDO el usuario cree una subtarea desde una fila, EL SISTEMA la creará como hija de esa tarea con sus fechas de inicio y fin, o con las de RF-86 si la tarea no las tiene.
- RF-89: CUANDO se cree una tarea en el cronograma mientras hay filtros activos, EL SISTEMA le pondrá los valores de los filtros que tengan un único valor, como en RF-59.

## Requisitos no funcionales
- **Rendimiento** (entorno local, proyecto con 2 000 tareas): el Resumen carga en < 500 ms; aplicar un filtro o cambiar de vista responde en < 300 ms; el kanban y el cronograma se pintan en < 1 s.
- **Accesibilidad:** todo lo que se hace arrastrando tiene una alternativa con teclado («Mover a» en el kanban, campos de fecha en el cronograma).
- **Plataformas:** navegadores de escritorio actuales; en móvil, las pestañas se leen y el arrastre se sustituye por las alternativas anteriores.
- **Seguridad:** todas las operaciones comprueban permisos, nunca nombres de rol ni de papel; los papeles de proyecto se añaden a la matriz única de la spec 003 y a sus tests.
- **Compatibilidad de despliegue:** la versión anterior de la API sigue funcionando con los cambios de esquema (regla de migraciones de `AGENTS.md`).
- **Idioma:** español por defecto; todos los textos nuevos salen del catálogo de i18n.

## Casos límite
- **Proyecto sin tareas:** el resumen muestra 0 %, sin riesgos ni carga, y una invitación a crear la primera tarea.
- **Proyecto cerrado o archivado:** todas las pestañas son de solo lectura; no se arrastra ni se asigna.
- **Responsable que cambia:** el nuevo responsable pasa a PM; el anterior conserva el papel que tenga por equipo o expresamente.
- **PM que deja la organización:** si queda inactivo, conserva el papel pero no tiene permisos (spec 003).
- **Persona en dos proyectos:** su carga total suma ambos; su papel es distinto en cada uno.
- **Vista compartida con «Yo»:** cada usuario ve sus propias tareas.
- **Dos personas mueven la misma tarjeta a la vez:** la segunda recibe `version_conflict` y su tarjeta vuelve (RF-64).
- **Tarea en dos hitos:** en el cronograma aparece en el grupo de cada hito.
- **Subtarea cuyo padre está en otro grupo:** se muestra en su grupo como tarea de primer nivel.
- **Tarea creada en el cronograma que no cumple los filtros activos:** se crea, pero no aparece hasta quitar los filtros.
- **Día sin registro de avance (servidor apagado):** la gráfica une los días registrados; no se inventan valores.

## Fuera de alcance
- Notificaciones de asignación, de sobrecarga o de cambios en vistas.
- Reordenar tarjetas dentro de una columna del kanban.
- Ediciones masivas (varias tareas a la vez).
- Recalcular fechas de forma automática por dependencias (ruta crítica) en el cronograma.
- Calendario de disponibilidad (vacaciones, festivos) y capacidad distinta por proyecto.
- Registro de horas trabajadas.
- Ver en tiempo real los cambios de otras personas sin recargar.
- Exportar las vistas a Excel, PDF o imágenes.
- Reconstruir el avance de días anteriores a la activación de esta spec.
- Papeles personalizados o permisos configurables por papel.
- Crear tareas arrastrando sobre la línea de tiempo vacía, o cambiar el padre de una tarea arrastrándola en el cronograma.

## Criterios de finalización
- Todos los RF con tests en verde: API (papeles en el test de matriz, capacidad, vistas, registro diario, sugerencias) y frontend (cada pestaña, filtros, vistas, arrastre y sus alternativas).
- Pruebas de rendimiento dentro de los umbrales.
- Demo manual:
  1. Abrir un proyecto y ver el resumen con avance, hitos, riesgos y carga.
  2. Como PM con rol global `member`, dar el papel líder técnico a alguien y comprobar que puede borrar tareas de otros.
  3. Poner a un participante como observador y comprobar que no puede editar tareas.
  4. Fijar capacidades, ver a alguien sobrecargado y asignarle una tarea a otra persona desde una sugerencia.
  5. Filtrar la tabla por etiqueta y «Yo», guardarlo como vista, compartirla y abrirla con otro usuario.
  6. Mover una tarjeta en el kanban, intentar moverla a un estado no permitido y usar «Mover a».
  7. Arrastrar una barra del cronograma y uno de sus extremos.

## Decisiones tomadas con el usuario (2026-11-02)
- **Papeles por proyecto:** dan permisos en ese proyecto (RF-39 a RF-43).
- **Capacidad:** global por persona (RF-46).
- **Vistas:** personales y compartidas por quien gestiona el equipo (RF-24, RF-25).
- **Edición directa:** se arrastra en el kanban (estado) y en el cronograma (fechas) (RF-62, RF-75, RF-76).
- **Gráfica del resumen:** avance registrado cada día desde la activación, plan lineal (RF-12 a RF-14).

- **Dudas cerradas con lo propuesto:** la pestaña Equipo solo la ven quienes gestionan el equipo (RF-4); el líder técnico gestiona solo tareas (RF-80); los límites del kanban los fijan el PM y los gestores (RF-66); el resumen muestra 10 riesgos y 5 personas (RF-15, RF-16). Las pestañas Hitos, Fases y Datos se conservan detrás de las nuevas (RF-1).

## Decisiones tomadas con el usuario (2026-11-03)
- **Cronograma:** escala por días (RF-81), fecha bajo el puntero al arrastrar (RF-82), tareas en árbol (RF-83 a RF-85) y alta de tareas y subtareas (RF-86 a RF-89).

## Dudas abiertas
Ninguna.
