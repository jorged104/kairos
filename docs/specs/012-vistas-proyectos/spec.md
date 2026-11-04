# Spec 012 — Vistas de proyectos

**Estado:** Aprobada (2026-11-04) («Dale demole»).

## Contexto y objetivo
La lista de proyectos es hoy una tabla con cuatro filtros fijos. Las tareas, en cambio, tienen tabla,
kanban y cronograma, filtros dinámicos y vistas guardadas (spec 008). Quien coordina muchos
proyectos necesita lo mismo un nivel más arriba:
- un **Kanban** como vista principal, para ver en qué etapa está cada proyecto y moverlo de etapa;
- un **Cronograma**, para ver las fechas de todos frente a hoy y ajustarlas;
- la **tabla** de siempre;
- en los tres modos, filtros que se guardan como vistas con nombre.

Las etapas de un proyecto no existen todavía: hoy un proyecto solo tiene ciclo de vida (activo,
cerrado o archivado) y salud. Esta spec añade un **catálogo editable de estados de proyecto**, con
valores neutros que sirven a cualquier industria. Las cifras de avance, plan y semáforo son las de
las specs 009 y 010, calculadas en tiempo real a la fecha de hoy.

El diseño de referencia es el lienzo «Vistas de proyectos» (Kanban, Cronograma, y Filtros y guardar
vista).

## Usuarios / actores
- **Cualquier usuario con lectura de trabajo:**
  - consulta los tres modos;
  - filtra;
  - guarda vistas personales.
- **Quien gestiona un proyecto** (su responsable, el líder de su equipo o el administrador):
  - cambia el estado del proyecto;
  - cambia las fechas del proyecto desde el Cronograma.
- **Quien puede crear proyectos y el administrador del sistema:** comparten vistas con toda la
  organización.
- **Administrador del catálogo:** mantiene el catálogo de estados de proyecto.

## Vocabulario
- **Estado de proyecto:** la etapa de un proyecto, elegida de un catálogo editable.
  - Cada estado tiene nombre, color, orden y una categoría: **abierto** o **cerrado**.
  - Ejemplos neutros por defecto: Propuesta, Planificación, En ejecución, En pausa (abiertos) y
    Cerrado (cerrado).
- **Modo:** la forma de mostrar los proyectos: Kanban, Cronograma o Tabla.
- **Vista de proyectos:** un nombre más una combinación de modo, filtros, orden y agrupación.
  - Puede ser personal o compartida con toda la organización.

## Historias de usuario
- H1: Como responsable de varios proyectos quiero verlos en un Kanban por etapa para saber de un vistazo cuántos hay en cada una y cuáles van atrasados.
- H2: Como responsable de un proyecto quiero arrastrarlo a otra columna para cambiar su etapa sin abrir su ficha.
- H3: Como coordinador quiero ver las fechas de todos los proyectos en un cronograma, agrupados por programa, para detectar solapes y retrasos.
- H4: Como responsable de un proyecto quiero estirar o mover su barra en el cronograma para ajustar sus fechas.
- H5: Como cualquier usuario quiero filtrar los proyectos por cualquier propiedad y guardar esa combinación como vista para volver a ella con un clic.
- H6: Como PM quiero compartir una vista con toda la organización para que todos revisen los mismos proyectos.
- H7: Como administrador quiero definir las etapas de un proyecto con los nombres de mi organización.

## Requisitos funcionales (criterios de aceptación en EARS)

### Catálogo de estados de proyecto
- RF-1: EL SISTEMA permitirá a quien administra el catálogo crear estados de proyecto con nombre, color y categoría (abierto o cerrado).
- RF-2: EL SISTEMA permitirá a quien administra el catálogo renombrar un estado, cambiar su color y cambiar su orden.
- RF-3: EL SISTEMA permitirá a quien administra el catálogo retirar un estado.
- RF-4: EL SISTEMA permitirá a quien administra el catálogo marcar un estado abierto como el estado inicial de los proyectos nuevos.
- RF-5: EL SISTEMA permitirá a quien administra el catálogo marcar un estado cerrado como el estado de cierre por defecto.
- RF-6: SI un cambio dejara el catálogo sin ningún estado abierto activo o sin ningún estado cerrado activo, ENTONCES EL SISTEMA lo rechazará con un error que lo indique.
- RF-7: SI se retira el estado inicial o el de cierre por defecto, ENTONCES EL SISTEMA lo rechazará hasta que otro estado ocupe ese papel.
- RF-8: SI el nombre de un estado coincide con otro activo (ignorando mayúsculas y espacios exteriores), ENTONCES EL SISTEMA rechazará el cambio con un código de conflicto.
- RF-9: CUANDO se retire un estado que tienen proyectos, EL SISTEMA mantendrá ese estado en esos proyectos y lo mostrará como retirado.
- RF-10: SI un usuario intenta elegir un estado retirado para un proyecto, ENTONCES EL SISTEMA lo rechazará.
- RF-11: EL SISTEMA auditará cada alta, cambio y retirada de un estado de proyecto.
- RF-12: CUANDO el sistema se actualice a esta versión, EL SISTEMA creará los cinco estados neutros por defecto: Propuesta (inicial), Planificación, En ejecución, En pausa y Cerrado (cierre por defecto).

### Estado de un proyecto
- RF-13: EL SISTEMA dará a cada proyecto que no sea plantilla exactamente un estado de proyecto.
- RF-14: CUANDO se cree un proyecto, EL SISTEMA le asignará el estado inicial.
- RF-15: CUANDO el sistema se actualice a esta versión, EL SISTEMA asignará «En ejecución» a los proyectos activos y archivados.
- RF-16: CUANDO el sistema se actualice a esta versión, EL SISTEMA asignará «Cerrado» a los proyectos cerrados.
- RF-17: EL SISTEMA permitirá a quien gestiona el proyecto cambiar su estado a cualquier estado activo del catálogo, sin reglas de flujo entre estados.
- RF-18: SI quien intenta cambiar el estado no puede gestionar el proyecto, ENTONCES EL SISTEMA lo rechazará con un error de permisos.
- RF-19: CUANDO un proyecto activo pase a un estado de categoría cerrado, EL SISTEMA cerrará el proyecto con las mismas reglas y efectos que la acción de cerrar.
- RF-20: CUANDO un proyecto cerrado pase a un estado de categoría abierto, EL SISTEMA reabrirá el proyecto con las mismas reglas y efectos que la acción de reabrir.
- RF-21: CUANDO un proyecto se cierre desde su ficha, EL SISTEMA le asignará el estado de cierre por defecto si su estado era abierto.
- RF-22: CUANDO un proyecto se reabra desde su ficha, EL SISTEMA le asignará el estado inicial si su estado era cerrado.
- RF-23: SI el proyecto está archivado, ENTONCES EL SISTEMA no permitirá cambiar su estado.
- RF-24: SI el cambio de estado llega con una versión que no es la actual del proyecto, ENTONCES EL SISTEMA lo rechazará con un conflicto de versión.
- RF-25: EL SISTEMA auditará cada cambio de estado con el estado anterior y el nuevo.
- RF-26: EL SISTEMA mostrará el estado del proyecto en su ficha y permitirá cambiarlo allí a quien lo gestiona.

### Pantalla y modos
- RF-27: EL SISTEMA ofrecerá en la pantalla Proyectos tres modos: Kanban, Cronograma y Tabla.
- RF-28: EL SISTEMA abrirá la pantalla en modo Kanban mientras el usuario no haya usado otro modo o abierto una vista.
- RF-29: EL SISTEMA mostrará en los tres modos solo los proyectos no archivados que no son plantilla y que el usuario puede ver.
- RF-30: EL SISTEMA guardará en la dirección de la página el modo, la vista y los filtros activos, de modo que el enlace abra lo mismo.
- RF-31: EL SISTEMA mantendrá los filtros, el orden y la agrupación al cambiar de modo.
- RF-32: EL SISTEMA mantendrá en la pantalla las acciones de crear proyecto que ya existen.

### Kanban
- RF-33: EL SISTEMA mostrará en el Kanban una columna por cada estado activo del catálogo, en el orden del catálogo, con el número de proyectos de cada una.
- RF-34: DONDE haya proyectos en un estado retirado, EL SISTEMA mostrará una columna adicional para ese estado, marcada como retirada.
- RF-35: EL SISTEMA mostrará en cada tarjeta el identificador, el nombre, el programa al que pertenece, el responsable y la fecha fin.
- RF-36: EL SISTEMA mostrará en cada tarjeta una barra del real con la marca del plan de hoy, y su semáforo (spec 009).
- RF-37: SI el proyecto no tiene plan, ENTONCES EL SISTEMA mostrará «Sin plazo» en lugar del semáforo.
- RF-38: CUANDO quien gestiona el proyecto suelte su tarjeta en otra columna, EL SISTEMA cambiará el estado del proyecto al de esa columna.
- RF-39: SI el cambio de estado al soltar la tarjeta falla, ENTONCES EL SISTEMA devolverá la tarjeta a su columna y mostrará el error traducido.
- RF-40: MIENTRAS el usuario no pueda gestionar un proyecto, EL SISTEMA no le dejará arrastrar su tarjeta.
- RF-41: EL SISTEMA ofrecerá, para cada tarjeta que el usuario pueda mover, una alternativa sin arrastre para elegir el nuevo estado (teclado o menú).
- RF-42: CUANDO el usuario pulse una tarjeta, EL SISTEMA abrirá el Resumen de ese proyecto.
- RF-43: EL SISTEMA permitirá elegir qué forma las columnas: estado del proyecto (por defecto), semáforo, salud o programa.
- RF-44: MIENTRAS las columnas no sean por estado del proyecto, EL SISTEMA no permitirá arrastrar tarjetas.
- RF-45: SI una columna tiene más de 100 proyectos, ENTONCES EL SISTEMA mostrará los 100 primeros según el orden elegido y un aviso con cuántos faltan.

### Cronograma
- RF-46: EL SISTEMA mostrará en el Cronograma una fila por proyecto con una barra de su fecha de inicio a su fecha fin.
- RF-47: EL SISTEMA rellenará cada barra en proporción a su real y la coloreará con el color de su semáforo.
- RF-48: EL SISTEMA marcará en cada barra el plan de hoy y mostrará una línea vertical en la fecha de hoy.
- RF-49: DONDE el proyecto tenga línea base (spec 009), EL SISTEMA mostrará bajo su barra las fechas de la línea base, salvo que el usuario la oculte.
- RF-50: EL SISTEMA mostrará los hitos de cada proyecto en su fecha límite, distinguiendo los completados, salvo que el usuario los oculte.
- RF-51: EL SISTEMA permitirá agrupar las filas por programa (por defecto), estado del proyecto o responsable, o no agruparlas.
- RF-52: DONDE las filas estén agrupadas por programa, EL SISTEMA mostrará en cada grupo su real, su plan y su semáforo agregados con las reglas de la spec 010.
- RF-53: EL SISTEMA permitirá elegir la escala del cronograma: semanas, meses o trimestres.
- RF-54: SI un proyecto no tiene fecha de inicio o de fin, ENTONCES EL SISTEMA lo mostrará en su fila sin barra y con el texto «Sin fechas».
- RF-55: CUANDO quien gestiona el proyecto arrastre un extremo de su barra, EL SISTEMA cambiará la fecha de inicio o la de fin del proyecto al día en que lo suelte.
- RF-56: CUANDO quien gestiona el proyecto arrastre la barra entera, EL SISTEMA desplazará el inicio y el fin el mismo número de días.
- RF-57: SI el cambio de fechas dejara el fin antes del inicio, o falla por cualquier otra causa, ENTONCES EL SISTEMA devolverá la barra a su sitio y mostrará el error traducido.
- RF-58: MIENTRAS el usuario no pueda gestionar un proyecto, EL SISTEMA no le dejará arrastrar su barra.
- RF-59: CUANDO el usuario pulse el nombre de un proyecto en el Cronograma, EL SISTEMA abrirá su Resumen.

### Tabla
- RF-60: EL SISTEMA mostrará en modo Tabla la tabla de proyectos actual, con su edición en línea y su fila de alta rápida, más las columnas de estado, programa, real, plan y semáforo.
- RF-61: EL SISTEMA permitirá ordenar la tabla por cualquiera de sus columnas.

### Filtros
- RF-62: EL SISTEMA permitirá filtrar los proyectos por estado del proyecto, semáforo (más «Sin plazo»), salud, programa (más «Sin programa»), equipo, responsable, fecha fin y texto en nombre o identificador.
- RF-63: EL SISTEMA ofrecerá en el filtro de responsable el valor «Yo», resuelto como el usuario que mira la vista.
- RF-64: EL SISTEMA ofrecerá en el filtro de fecha fin los valores vencida, este mes, este trimestre, sin fecha y un rango de fechas.
- RF-65: EL SISTEMA combinará los valores de un mismo campo con «o» y los campos distintos con «y».
- RF-66: EL SISTEMA mostrará cada filtro activo como una pastilla que se puede quitar.
- RF-67: DONDE se filtre por programa, EL SISTEMA incluirá todos los proyectos descendientes de ese programa, no solo sus hijos directos.
- RF-68: EL SISTEMA permitirá ordenar los proyectos por brecha (por defecto, los más atrasados primero), fecha fin, nombre o real.

### Vistas guardadas
- RF-69: EL SISTEMA ofrecerá la vista «Todos», sin filtros, que no se puede modificar ni borrar.
- RF-70: EL SISTEMA permitirá a cualquier usuario con lectura de trabajo guardar el modo, los filtros, el orden y la agrupación actuales como una vista personal con nombre.
- RF-71: DONDE el usuario pueda crear proyectos o sea administrador del sistema, EL SISTEMA le permitirá marcar una vista como compartida con toda la organización.
- RF-72: EL SISTEMA mostrará a cada usuario la vista «Todos», las vistas compartidas y sus vistas personales, con el número de proyectos de cada una.
- RF-73: CUANDO el usuario abra una vista, EL SISTEMA aplicará su modo, sus filtros, su orden y su agrupación.
- RF-74: CUANDO los filtros, el orden, la agrupación o el modo actuales difieran de los de la vista activa, EL SISTEMA mostrará «Cambios sin guardar» con las opciones de actualizar la vista o guardarla como nueva.
- RF-75: EL SISTEMA solo dejará actualizar, renombrar o borrar una vista personal a su autor, y una compartida a quien puede compartir vistas.
- RF-76: SI el nombre de una vista coincide con otra visible para el mismo usuario (ignorando mayúsculas y espacios exteriores), ENTONCES EL SISTEMA rechazará el cambio con un código de conflicto.
- RF-77: SI un usuario intenta tener más de 50 vistas personales de proyectos, o la organización más de 30 compartidas, ENTONCES EL SISTEMA rechazará la nueva vista.
- RF-78: SI una vista filtra por un valor que ya no existe o está retirado (estado, programa, equipo, persona), ENTONCES EL SISTEMA aplicará el resto de filtros y marcará esa pastilla como «no disponible».
- RF-79: EL SISTEMA recordará en el navegador la última vista y el último modo usados por el usuario, y los abrirá la próxima vez.
- RF-80: EL SISTEMA auditará el alta, el cambio y el borrado de las vistas compartidas.

## Requisitos no funcionales
- **Tiempo real:** las cifras de avance, plan y semáforo se calculan en cada consulta a la fecha de hoy; no se guardan fotos.
- **Rendimiento:** con 300 proyectos no archivados de 2 000 tareas en total cada uno, en el servidor de previsualización:
  - el Kanban y el Cronograma se pintan en menos de 1,5 s;
  - aplicar un filtro o cambiar de vista responde en menos de 500 ms.
- **Idioma:** todos los textos visibles en español, desde el catálogo de traducciones; los errores se traducen por su código.
- **Sin términos de un sector concreto** en textos, valores por defecto ni ejemplos.
- **Ancho mínimo:** la pantalla se usa a partir de 360 px sin desplazamiento horizontal de la página; el Kanban, el Cronograma y la tabla se desplazan dentro de su caja.
- **Accesibilidad:** todo lo que se hace arrastrando tiene una alternativa con teclado.

## Casos límite
- **Catálogo con un único estado abierto y uno cerrado:** el Kanban tiene dos columnas, y ninguno de los dos se puede retirar (RF-6).
- **Mover a «Cerrado» un proyecto con tareas abiertas:** se aplican las mismas reglas que al cerrar desde la ficha; si cerrar falla, la tarjeta vuelve a su columna (RF-39).
- **Mover entre dos estados abiertos:** cambia solo el estado; el ciclo de vida sigue activo.
- **Mover entre dos estados cerrados:** cambia solo el estado; el proyecto sigue cerrado.
- **Archivar un proyecto:** conserva su estado y desaparece de los tres modos. Al desarchivarlo vuelve a la columna de su estado.
- **Dos personas mueven el mismo proyecto a la vez:** la segunda recibe un conflicto de versión y su tarjeta vuelve a la columna real.
- **Columnas por programa con programas anidados:** cada proyecto aparece en la columna de su programa más cercano. Los que no tienen programa van a «Sin programa».
- **Arrastrar la barra de un proyecto sin línea base:** cambia sus fechas vigentes. Con línea base, la línea base no cambia y la barra se separa de su sombra.
- **Vista compartida con «Yo»:** cada usuario ve sus propios proyectos.
- **Vista guardada en Cronograma que se abre en un móvil:** abre en Cronograma con desplazamiento dentro de su caja.

## Fuera de alcance
- Reglas de flujo o transiciones permitidas entre estados de proyecto.
- Límites de proyectos por columna (WIP) en el Kanban.
- Dependencias entre proyectos en el Cronograma.
- Mover tareas o hitos desde el Cronograma de proyectos.
- Estados de proyecto distintos por equipo, programa o metodología.
- Exportar las vistas a Excel, PDF o imágenes.
- Notificaciones por cambios de estado o de fechas.
- Mapa estratégico (pasa a ser la spec 013).

## Criterios de finalización
- Todos los RF con test en verde:
  - API: catálogo, cambio de estado con cierre y reapertura, vistas, permisos en el test de matriz, migración de los proyectos existentes;
  - frontend: cada modo, filtros, vistas, arrastre y sus alternativas con teclado.
- Demo manual:
  1. Como administrador, renombrar «En pausa», añadir un estado nuevo y ver las columnas del Kanban.
  2. Como PM, arrastrar un proyecto a «Cerrado» y comprobar que queda cerrado; arrastrarlo de vuelta y comprobar que se reabre.
  3. Intentar mover un proyecto ajeno como usuario sin gestión y ver que no se puede.
  4. Filtrar por «Yo» y «Atrasado», guardar como vista, compartirla y abrirla con otro usuario.
  5. Pasar a Cronograma con la misma vista, agrupar por programa y estirar la barra de un proyecto.
  6. Copiar el enlace y abrirlo en otra ventana con el mismo modo y los mismos filtros.

## Decisiones tomadas con el usuario (2026-11-04)
- **Columnas del Kanban:** un catálogo editable de estados de proyecto; además se puede agrupar por semáforo, salud o programa, en solo lectura.
- **Relación con el ciclo de vida:** cada estado tiene categoría abierto o cerrado. Mover a un estado cerrado cierra el proyecto, y sacarlo de ahí lo reabre. Los archivados no salen.
- **Quién mueve:** quien gestiona el proyecto, a cualquier estado y sin flujo.
- **Vistas:** cualquiera guarda vistas personales. Comparten con toda la organización quienes pueden crear proyectos y el administrador.
- **Cronograma:** se pueden cambiar las fechas arrastrando, solo quien gestiona el proyecto.
- **Proyectos existentes:**
  - los activos y archivados pasan a «En ejecución»;
  - los cerrados pasan a «Cerrado»;
  - los nuevos nacen en el estado inicial.

## Dudas abiertas
Ninguna.
