# Spec 018 — Reuniones

**Estado:** Aprobada (2026-11-05) («comienza a trabajar»), sin Teams.

## Contexto y objetivo
Los equipos se reúnen para planificar, seguir el avance y tomar acuerdos. Hoy esas notas viven fuera de Kairos, o en la descripción de alguna tarea, y los acuerdos se pierden.

Esta spec añade la **reunión** como un elemento nuevo de Kairos:
- tiene título, fecha y hora, y un lugar o enlace;
- sus notas se escriben en una página con el mismo editor que las tareas (spec 005);
- se puede crear a partir de una plantilla;
- lleva comentarios y menciones;
- desde sus notas se crean tareas.

Por ahora una reunión pertenece a un proyecto o a un programa (spec 010). Se ve en una pestaña del proyecto y en el portafolio. La reunión se diseña para poder pertenecer más adelante a otros elementos, sin rehacer las que ya existan.

Es neutra respecto al sector: vale para cualquier organización.

## Usuarios / actores
- **Quien puede escribir tareas en el proyecto** (spec 004): crea, edita y borra las reuniones de ese proyecto, y crea tareas desde ellas.
- **Quien puede gestionar el proyecto** (responsable, PM o líder): además, gestiona las plantillas propias del proyecto.
- **Quien administra el catálogo:** gestiona las plantillas globales.
- **Quien puede comentar en el proyecto:** comenta en sus reuniones.
- **Cualquier usuario con lectura de trabajo:** consulta las reuniones.

## Vocabulario
- **Reunión:** un elemento con título, fecha y hora, lugar o enlace opcional, y una página de notas. Pertenece a un elemento.
- **Elemento al que pertenece:** hoy, un proyecto, que puede ser un programa.
- **Plantilla de reunión:** un modelo con nombre, título propuesto, lugar o enlace propuesto y página inicial, por ejemplo «Seguimiento semanal» con su orden del día.
  - Una **plantilla global** sirve para todos los proyectos.
  - Una **plantilla del proyecto** solo sirve para ese proyecto.
- **Tarea de la reunión:** una tarea creada desde las notas de una reunión, que queda enlazada a ella.

## Historias de usuario
- H1: Como PM quiero crear la reunión de seguimiento semanal desde una plantilla para no escribir cada vez el orden del día.
- H2: Como persona del equipo quiero tomar las notas de la reunión con el mismo editor que uso en las tareas.
- H3: Como persona del equipo quiero convertir un acuerdo de la reunión en una tarea sin salir de las notas.
- H4: Como persona del equipo quiero ver las próximas reuniones del proyecto y consultar las pasadas.
- H5: Como responsable de un programa quiero ver en el portafolio cuándo es la próxima reunión de cada programa.
- H6: Como administrador quiero ofrecer a todos los proyectos unas plantillas comunes, y que cada proyecto pueda tener las suyas.

## Requisitos funcionales (criterios de aceptación en EARS)

### La reunión
- RF-1: EL SISTEMA guardará de cada reunión un título, una fecha y hora, un lugar o enlace opcional, y una página de notas.
- RF-2: EL SISTEMA guardará de cada reunión el elemento al que pertenece; hoy, un proyecto o un programa.
- RF-3: SI el título está vacío o supera 255 caracteres, ENTONCES EL SISTEMA rechazará la reunión e indicará el campo.
- RF-4: SI falta la fecha y hora, ENTONCES EL SISTEMA rechazará la reunión e indicará el campo.
- RF-5: SI el lugar o enlace supera 500 caracteres, ENTONCES EL SISTEMA rechazará la reunión e indicará el campo.
- RF-6: DONDE el lugar o enlace sea una dirección `http` o `https`, EL SISTEMA lo mostrará como enlace, que se abre en otra pestaña.
- RF-7: EL SISTEMA mostrará la fecha y hora de la reunión en la zona horaria de quien la consulta.
- RF-8: EL SISTEMA permitirá crear, editar y borrar reuniones de un proyecto a quien pueda escribir tareas en ese proyecto.
- RF-9: SI alguien sin ese permiso intenta crear, editar o borrar una reunión, ENTONCES EL SISTEMA lo rechazará.
- RF-10: MIENTRAS el proyecto no esté activo (cerrado o archivado), EL SISTEMA mostrará sus reuniones en modo lectura y rechazará cualquier cambio en ellas.
- RF-11: CUANDO alguien vaya a borrar una reunión, EL SISTEMA pedirá confirmación antes.
- RF-12: CUANDO se borre una reunión, EL SISTEMA borrará sus comentarios y marcará como borrados sus archivos, como hace con los de las tareas.
- RF-13: CUANDO se borre una reunión, EL SISTEMA conservará las tareas creadas desde ella, sin el enlace a la reunión.
- RF-14: EL SISTEMA auditará la creación, la edición y el borrado de cada reunión, y registrará solo que cambiaron sus notas, nunca su contenido.
- RF-15: SI la reunión cambió desde que se abrió (conflicto de versión), ENTONCES EL SISTEMA rechazará la edición y pedirá recargarla.

### Notas
- RF-16: EL SISTEMA ofrecerá en las notas de la reunión el mismo editor de páginas que en las tareas, con sus bloques, imágenes y archivos, menciones y guardado automático (spec 005).
- RF-17: EL SISTEMA permitirá mencionar en las notas a personas con `@` y a tareas, hitos y reuniones con `#`.
- RF-18: EL SISTEMA permitirá mencionar una reunión con `#` desde cualquier otra página o comentario.
- RF-19: CUANDO se mencione a una persona en las notas o en un comentario de una reunión, EL SISTEMA se lo notificará con las reglas de las menciones de la spec 017.

### Comentarios
- RF-20: EL SISTEMA ofrecerá en cada reunión los mismos comentarios que en las tareas: hilos, respuestas, resolver y reabrir, y comentarios anclados a un texto de las notas (spec 006).
- RF-21: EL SISTEMA permitirá comentar en una reunión a quien pueda comentar en su proyecto.
- RF-22: CUANDO alguien responda en un hilo de una reunión, EL SISTEMA notificará a los participantes del hilo, como en las tareas (spec 017, RF-9).

### Tareas desde la reunión
- RF-23: CUANDO quien puede escribir tareas seleccione texto en las notas de una reunión, EL SISTEMA le ofrecerá «Crear tarea» junto a la selección.
- RF-24: CUANDO la persona confirme «Crear tarea», EL SISTEMA creará una tarea en el proyecto de la reunión, con el texto seleccionado como título (hasta 255 caracteres) y el tipo que la persona elija.
- RF-25: CUANDO se cree una tarea desde la reunión, EL SISTEMA insertará en las notas, tras el texto seleccionado, una mención a la nueva tarea.
- RF-26: EL SISTEMA enlazará cada tarea creada desde una reunión con esa reunión.
- RF-27: EL SISTEMA mostrará en la reunión la lista de sus tareas, con su clave, su título y su estado.
- RF-28: EL SISTEMA mostrará en la ficha de la tarea la reunión de la que salió, con un enlace a ella.

### Plantillas
- RF-29: EL SISTEMA permitirá a quien administra el catálogo crear, editar, reordenar y retirar plantillas globales.
- RF-30: EL SISTEMA permitirá a quien gestiona un proyecto crear, editar y borrar plantillas de ese proyecto.
- RF-31: EL SISTEMA guardará de cada plantilla un nombre, un título propuesto, un lugar o enlace propuesto y una página inicial.
- RF-32: EL SISTEMA admitirá en la página de una plantilla los mismos bloques que en las notas, salvo imágenes y archivos.
- RF-33: SI el nombre de una plantilla ya existe en su ámbito (comparación sin mayúsculas ni espacios exteriores), ENTONCES EL SISTEMA no la creará e informará del conflicto.
- RF-34: CUANDO se cree una reunión, EL SISTEMA ofrecerá elegir entre «En blanco», las plantillas globales activas y las plantillas del proyecto.
- RF-35: CUANDO se cree una reunión desde una plantilla, EL SISTEMA copiará en ella el título, el lugar o enlace y la página de la plantilla, y la persona podrá cambiarlos.
- RF-36: CUANDO se edite o se retire una plantilla, EL SISTEMA no cambiará las reuniones creadas antes desde ella.
- RF-37: EL SISTEMA auditará los cambios de las plantillas.

### Dónde se ven
- RF-38: EL SISTEMA mostrará en la ficha de cada proyecto y programa una pestaña «Reuniones», con las próximas reuniones de la más cercana a la más lejana y las pasadas de la más reciente a la más antigua.
- RF-39: EL SISTEMA mostrará de cada reunión de la lista su título, su fecha y hora, su lugar o enlace, y cuántas tareas salieron de ella.
- RF-40: EL SISTEMA mostrará las listas de reuniones por páginas de 50.
- RF-41: CUANDO la persona pulse una reunión de la lista, EL SISTEMA la abrirá en el panel lateral, como las tareas, con la opción de abrirla en su propia página.
- RF-42: EL SISTEMA dará a cada reunión una dirección propia que se puede compartir.
- RF-43: EL SISTEMA mostrará en el portafolio, para cada programa, sus próximas 3 reuniones, con un enlace a la pestaña «Reuniones» del programa.
- RF-44: EL SISTEMA mostrará en un programa solo sus propias reuniones, sin las de sus proyectos hijos.

### Desde un cliente de IA (MCP, spec 016)
- RF-45: EL SISTEMA ofrecerá por el MCP herramientas de consulta para:
  - listar las reuniones de un proyecto o programa, próximas o pasadas;
  - ver una reunión con sus notas en Markdown, sus comentarios y sus tareas;
  - listar las plantillas disponibles en un proyecto.
- RF-46: EL SISTEMA ofrecerá por el MCP una herramienta para crear una reunión en un proyecto o programa, con título, fecha y hora, lugar o enlace, notas en Markdown y, opcionalmente, una plantilla.
- RF-47: CUANDO se cree una reunión por el MCP con plantilla y con notas, EL SISTEMA pondrá la página de la plantilla y, a continuación, las notas recibidas.
- RF-48: EL SISTEMA ofrecerá por el MCP una herramienta para cambiar el título, la fecha y hora o el lugar de una reunión.
- RF-49: EL SISTEMA ofrecerá por el MCP una herramienta para añadir texto a las notas de una reunión o reemplazarlas, como con las demás páginas (spec 016).
- RF-50: EL SISTEMA ofrecerá por el MCP una herramienta para comentar en una reunión.
- RF-51: EL SISTEMA ofrecerá por el MCP una herramienta para crear una tarea desde una reunión, enlazada a ella y mencionada al final de sus notas (RF-25, RF-26).
- RF-52: SI la fecha y hora que llega por el MCP no indica su zona horaria, ENTONCES EL SISTEMA la rechazará con un error que pida indicarla.
- RF-53: EL SISTEMA no ofrecerá por el MCP borrar reuniones ni gestionar plantillas (spec 016, RF-31).
- RF-54: EL SISTEMA aplicará a las herramientas de reuniones los mismos permisos que en la interfaz, y marcará lo que cambian como «vía IA» (spec 016).

## Requisitos no funcionales
- **Ampliable:** el elemento al que pertenece una reunión se guarda de forma que una spec futura pueda permitir otros elementos (por ejemplo hitos o equipos), sin cambiar las reuniones existentes ni su dirección.
- **Mismo editor:** las notas, las plantillas y los comentarios usan el mismo editor, la misma lista blanca de bloques (ADR 0003) y los mismos límites de tamaño y de archivos que las tareas.
- **Rendimiento:** la pestaña «Reuniones» responde en menos de 1 s con 1.000 reuniones en el proyecto.
- **Idioma:** todos los textos salen de `es.json`.

## Casos límite
- **Reunión en un proyecto que pasa a ser programa:** sigue siendo suya y se ve igual.
- **Reunión sin hora relevante:** la hora es obligatoria; la persona pone la que quiera.
- **Plantilla del proyecto con el mismo nombre que una global:** se permite, porque el ámbito es distinto. Al elegir, se distinguen por su grupo.
- **Plantilla retirada:** no se ofrece para reuniones nuevas, pero las reuniones creadas con ella no cambian (RF-36).
- **Crear tarea con una selección de más de 255 caracteres:** el título se corta a 255 y la persona puede editarlo antes de confirmar.
- **Crear tarea sin permiso de escribir tareas:** no se ofrece la opción (RF-23); quien solo comenta puede comentar.
- **Mención a una reunión que la persona no puede ver:** se muestra como «sin acceso», como las demás menciones (spec 005).
- **Copiar un proyecto desde una plantilla (spec 004):** no copia sus reuniones.
- **Borrar una reunión con tareas creadas desde ella:** las tareas se quedan, sin el enlace (RF-13).

## Fuera de alcance
- Asistentes, convocatorias, confirmación de asistencia y estado de la reunión (programada, realizada o cancelada).
- Duración, hora de fin y reuniones recurrentes.
- Calendario, exportar a iCal e integración con calendarios externos o con herramientas de videollamada.
- Reuniones del portafolio entero, sin proyecto, o asociadas a varios proyectos a la vez.
- Reuniones de los proyectos hijos dentro de un programa (RF-44).
- Reuniones en Mi panel.
- Notificaciones por crear o cambiar una reunión, y seguir reuniones.
- Borrar reuniones y gestionar plantillas desde el MCP (RF-53).
- Integración con Microsoft Teams u otras herramientas de videollamada: queda para una spec propia.
- Copiar reuniones al crear un proyecto desde una plantilla.

## Criterios de finalización
- Todos los RF con test en verde:
  - backend: apitest de reuniones, plantillas, permisos, tareas desde la reunión y menciones, más filas en la matriz;
  - MCP: crear una reunión con plantilla, completar sus notas y crear una tarea desde ella;
  - frontend: pestaña, ficha, plantillas, «Crear tarea» y portafolio.
- Demo manual:
  - crear una plantilla global y una del proyecto;
  - crear una reunión desde una de ellas, tomar notas, crear dos tareas desde las notas y comentar;
  - ver la reunión en la pestaña y en el portafolio.

## Decisiones tomadas con el usuario (2026-11-05)
1. Una reunión pertenece a un proyecto o a un programa. En el portafolio se ven las de los programas.
2. Datos: título, fecha y hora, lugar o enlace, y las notas.
3. Plantillas globales del catálogo y plantillas propias de cada proyecto.
4. Crea y edita reuniones quien puede escribir tareas en el proyecto.
5. Relaciones: crear tareas desde las notas, menciones y comentarios.
6. Se ven en una pestaña del proyecto o programa y en el portafolio.
7. Las reuniones se pueden crear, consultar y completar desde un cliente de IA por el MCP (RF-45 a RF-54).

Propuestas mías que van en el borrador, para confirmar:
- borrar una reunión pide confirmación y conserva sus tareas, sin el enlace (RF-11, RF-13);
- las plantillas del proyecto las gestiona quien gestiona el proyecto, y las globales quien administra el catálogo (RF-29, RF-30);
- las plantillas no admiten imágenes ni archivos (RF-32);
- la reunión se abre en el panel lateral como las tareas, y tiene su propia página (RF-41, RF-42);
- en el portafolio se ven las 3 próximas reuniones de cada programa (RF-43);
- un programa no muestra las reuniones de sus proyectos hijos (RF-44);
- notificaciones solo por menciones y respuestas en hilos, como en la spec 017 (RF-19, RF-22).

## Dudas abiertas
- Ninguna.
