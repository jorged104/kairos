# Spec 016 — Conexión de clientes de IA (MCP)

**Estado:** Aprobada (2026-11-05) («Dale»).

## Contexto y objetivo
Muchas personas ya trabajan con un asistente de IA: Claude, ChatGPT u otro. Hoy, para que ese asistente ayude con el trabajo de Kairos, hay que copiar y pegar a mano tareas, avances y comentarios.

Esta spec abre Kairos a esos asistentes con el protocolo abierto MCP (Model Context Protocol). La persona crea en Kairos una conexión y recibe **su propia URL de conexión**. La pega en su cliente de IA, en cualquiera que acepte un servidor MCP por URL, y no hace falta nada más. Desde ahí el asistente puede:
- consultar su trabajo y el de sus proyectos;
- crear y actualizar tareas, hitos y proyectos;
- comentar.

Todo con exactamente los mismos permisos que tiene la persona en la interfaz.

Lo que el asistente cambia cuenta como hecho por la persona, pero queda marcado «vía IA». El administrador del sistema puede:
- apagar el acceso para todos;
- ver y cortar las conexiones de cualquiera.

Nada de esto depende de un sector concreto ni de un cliente de IA concreto.

## Usuarios / actores
- **Usuario:** cualquier persona con cuenta activa. Crea sus conexiones, las pega en su cliente de IA y las gestiona.
- **Administrador de proyectos:** quien lleva proyectos (líder, PM). Usa el mismo acceso que cualquier usuario. Lo que puede hacer por IA en sus proyectos es lo que le dan sus permisos.
- **Administrador del sistema** (permiso de administrar el sistema, spec 015): activa o desactiva el acceso por MCP y ve y revoca las conexiones de todos.
- **Cliente de IA:** la aplicación que se conecta en nombre de la persona. Por ejemplo, Claude.ai, ChatGPT, Claude Desktop o Claude Code, o cualquier otro compatible con MCP.

## Vocabulario
- **MCP:** protocolo abierto con el que un cliente de IA descubre y usa las **herramientas** que ofrece un servicio.
- **Herramienta:** una operación concreta que el cliente de IA puede invocar, como «buscar tareas» o «cambiar el estado de una tarea».
- **Conexión:** el acceso que una persona crea para un cliente de IA. Tiene nombre, caducidad y su propia URL.
- **URL de conexión:** la dirección única y secreta de una conexión. Quien la tiene actúa como la persona dueña, así que se trata como una contraseña.
- **Vía IA:** la marca que indica que un cambio llegó por una conexión de IA, y por cuál.
- **Acción irreversible:** borrar una tarea, un hito, un proyecto, un comentario o una página, y archivar o cancelar un proyecto.

## Historias de usuario
- H1: Como usuario quiero preguntar a mi asistente «¿qué tengo pendiente esta semana?» para no tener que abrir Kairos.
- H2: Como usuario quiero decirle a mi asistente «marca como terminada la tarea X y comenta que quedó desplegada» para actualizar mi trabajo desde donde estoy.
- H3: Como administrador de proyectos quiero pedir a mi asistente un resumen del avance y las tareas atrasadas de mi proyecto para preparar el seguimiento.
- H4: Como administrador de proyectos quiero que mi asistente cree las tareas de un hito a partir de unas notas de reunión.
- H5: Como usuario quiero conectar cualquier cliente de IA pegando una URL, sin pasos distintos para cada uno.
- H6: Como usuario quiero ver y revocar mis conexiones.
- H7: Como administrador del sistema quiero apagar el acceso por IA y cortar las conexiones de cualquiera.
- H8: Como cualquier persona del equipo quiero saber qué cambios se hicieron vía IA.

## Requisitos funcionales (criterios de aceptación en EARS)

### Activación
- RF-1: EL SISTEMA tendrá el acceso por MCP desactivado en una instalación nueva.
- RF-2: EL SISTEMA mostrará al administrador del sistema, en la pantalla «Sistema», un interruptor para activar o desactivar el acceso por MCP.
- RF-3: MIENTRAS el acceso por MCP esté desactivado, EL SISTEMA rechazará toda petición MCP con un error que diga que el acceso por IA está desactivado.
- RF-4: MIENTRAS el acceso por MCP esté desactivado, EL SISTEMA no dejará crear conexiones.
- RF-5: CUANDO el administrador desactive el acceso por MCP, EL SISTEMA conservará las conexiones existentes.
- RF-6: CUANDO el administrador vuelva a activar el acceso por MCP, las conexiones existentes funcionarán de nuevo con la misma URL.
- RF-7: EL SISTEMA auditará cada activación y desactivación del acceso por MCP.

### Crear una conexión
- RF-8: EL SISTEMA dejará a cada persona crear conexiones en Mi cuenta, con un nombre que ella elige (por ejemplo, «Claude del portátil»).
- RF-9: CUANDO la persona cree una conexión, EL SISTEMA le pedirá la caducidad: 30, 90 o 365 días. Por defecto, 90.
- RF-10: CUANDO se cree una conexión, EL SISTEMA generará para ella una URL de conexión única. La URL tendrá al menos 128 bits aleatorios.
- RF-11: CUANDO se cree una conexión, EL SISTEMA mostrará su URL una sola vez, con un botón para copiarla.
- RF-12: EL SISTEMA no volverá a mostrar nunca la URL de una conexión ya creada.
- RF-13: EL SISTEMA guardará las conexiones de forma que su URL no se pueda recuperar a partir de lo guardado.
- RF-14: EL SISTEMA dejará a cada persona tener como máximo 10 conexiones vigentes.
- RF-15: CUANDO un cliente de IA use una URL de conexión vigente, EL SISTEMA atenderá sus peticiones MCP sin pedir ningún otro dato de acceso.
- RF-16: EL SISTEMA aceptará la URL de conexión en cualquier cliente que admita un servidor MCP remoto por URL, sin pasos propios de un cliente concreto.
- RF-17: CUANDO la persona cree una conexión, EL SISTEMA le mostrará junto a la URL un aviso: quien tenga la URL actúa en su nombre, así que no debe compartirla.
- RF-18: EL SISTEMA mostrará en Mi cuenta unos pasos generales para conectar: crear la conexión, copiar la URL y pegarla en el cliente como servidor MCP o conector personalizado.

### Gestionar las conexiones
- RF-19: EL SISTEMA mostrará a cada persona, en Mi cuenta, sus conexiones vigentes. De cada una indicará:
  - el nombre;
  - la fecha de creación;
  - el último uso;
  - el nombre del cliente que la usó por última vez, si el cliente lo informa;
  - la caducidad.
- RF-20: CUANDO la persona revoque una de sus conexiones, EL SISTEMA rechazará desde ese momento toda petición a su URL.
- RF-21: SI una conexión ha caducado, ENTONCES EL SISTEMA rechazará las peticiones a su URL.
- RF-22: SI llega una petición a una URL de conexión inexistente, caducada o revocada, ENTONCES EL SISTEMA la rechazará con el mismo error en los tres casos. El error dirá que se cree una conexión nueva en Mi cuenta.
- RF-23: EL SISTEMA mostrará al administrador del sistema, en la pantalla «Sistema», todas las conexiones vigentes con los mismos datos que RF-19 y además la persona dueña.
- RF-24: CUANDO el administrador del sistema revoque una conexión de otra persona, EL SISTEMA la cortará como en RF-20.
- RF-25: EL SISTEMA auditará la creación y la revocación de cada conexión, indicando quién la revocó.
- RF-26: CUANDO una persona quede desactivada o eliminada, EL SISTEMA rechazará las peticiones de todas sus conexiones.

### Qué puede hacer el cliente de IA
- RF-27: EL SISTEMA ejecutará cada herramienta como la persona dueña de la conexión, con sus permisos vigentes en el momento de la petición.
- RF-28: SI la persona no tiene permiso para lo que pide una herramienta, ENTONCES EL SISTEMA rechazará la petición con el mismo motivo que daría la interfaz.
- RF-29: EL SISTEMA ofrecerá herramientas de consulta para:
  - mi panel: mis tareas, lo atrasado y lo próximo (spec 013);
  - buscar proyectos y ver la ficha de un proyecto;
  - el resumen, el plan frente a real y los cortes de un proyecto (specs 008 y 009);
  - el portafolio y los programas (spec 010);
  - el seguimiento con sus filtros (spec 011);
  - los hitos de un proyecto y su avance;
  - buscar tareas con filtros y ver una tarea con sus subtareas, dependencias y comentarios;
  - leer una página (spec 005);
  - mis puntos y mi nivel (spec 014);
  - los valores del catálogo que se necesitan para escribir: tipos, estados, prioridades, tallas, etiquetas y personas asignables.
- RF-30: EL SISTEMA ofrecerá herramientas de modificación para:
  - crear una tarea o subtarea;
  - editar los campos de una tarea: título, descripción, fechas, prioridad, talla, etiquetas y campos personalizados;
  - cambiar el estado de una tarea según su flujo;
  - asignar o desasignar personas;
  - añadir y quitar dependencias;
  - comentar en una tarea;
  - crear y editar un hito;
  - editar los campos de un proyecto;
  - editar el contenido de una página.
- RF-31: EL SISTEMA no ofrecerá ninguna herramienta que haga una acción irreversible.
- RF-32: EL SISTEMA no ofrecerá herramientas de usuarios, equipos, atributos, homologaciones, catálogo, gamificación, auditoría ni configuración del sistema.
- RF-33: EL SISTEMA marcará cada herramienta como de solo lectura o de modificación, para que el cliente de IA lo sepa.
- RF-34: CUANDO el cliente de IA escriba una descripción, un comentario o una página, EL SISTEMA aceptará texto con formato básico (Markdown: títulos, listas, negrita, cursiva, enlaces, código y citas).
- RF-35: EL SISTEMA guardará ese texto con la misma lista blanca que las descripciones escritas en la interfaz (ADR 0003).
- RF-36: CUANDO el cliente de IA lea una descripción, un comentario o una página, EL SISTEMA se la devolverá como texto con el mismo formato básico.
- RF-37: EL SISTEMA dejará buscar proyectos, hitos y tareas por texto en su nombre.
- RF-38: EL SISTEMA devolverá en cada resultado el identificador con el que se usa en las demás herramientas, junto con el enlace a su pantalla en Kairos.
- RF-39: EL SISTEMA devolverá las listas por páginas, con 50 elementos como máximo en cada una.
- RF-40: SI la modificación choca con un cambio de otra persona (conflicto de versión), ENTONCES EL SISTEMA rechazará la petición con un error que pida volver a leer el elemento.
- RF-41: SI una petición tiene datos no válidos, ENTONCES EL SISTEMA responderá con el código de error y un mensaje en español que diga qué campo falla.

### Autoría «vía IA»
- RF-42: EL SISTEMA atribuirá cada cambio hecho por una conexión a la persona dueña de esa conexión.
- RF-43: EL SISTEMA registrará en la auditoría de cada cambio vía IA el nombre de la conexión y, si el cliente lo informa, el nombre del cliente.
- RF-44: EL SISTEMA mostrará la marca «vía IA» y el nombre de la conexión en los cambios vía IA, tanto en el historial de la tarea como en sus comentarios.
- RF-45: EL SISTEMA otorgará los puntos de gamificación de los cambios vía IA igual que los de la interfaz (spec 014).
- RF-46: CUANDO un cambio vía IA se guarde, EL SISTEMA lo mostrará en tiempo real a las demás personas, igual que un cambio hecho en la interfaz.

### Límites
- RF-47: SI una conexión supera 120 peticiones por minuto, ENTONCES EL SISTEMA rechazará las siguientes con un error que diga cuándo reintentar.

## Requisitos no funcionales
- **Seguridad:**
  - fuera de desarrollo local, el acceso por MCP solo funciona por HTTPS;
  - la URL de conexión, o su parte secreta, no aparece en logs, auditoría ni respuestas, salvo al crearla (RF-11).
- **Compatibilidad:** basta con pegar la URL para conectar los conectores personalizados de Claude.ai y ChatGPT, Claude Desktop y Claude Code.
- **Idioma:**
  - todo texto visible en Kairos (Mi cuenta, Sistema) sale de `es.json`;
  - las descripciones de las herramientas y los mensajes de error que lee el cliente de IA, en español.
- **Rendimiento:** con 1.000 tareas en el proyecto, una consulta de herramienta responde en menos de 2 s, igual que la pantalla equivalente.

## Casos límite
- **Permisos que cambian:** si a la persona le quitan un permiso o la sacan de un proyecto, la siguiente petición ya lo refleja y la conexión sigue viva (RF-27).
- **Modo de acceso que cambia:** las conexiones no dependen de cómo se entra en Kairos; siguen valiendo aunque el modo pase a «solo SSO».
- **Misma URL en dos clientes:** funciona en los dos y todo se atribuye a la misma conexión. Para distinguirlos, se crea una conexión para cada cliente.
- **URL pegada en un sitio público:** la persona o el administrador la revocan (RF-20, RF-24) y el corte es inmediato.
- **URL perdida:** no se puede recuperar (RF-12); se revoca la conexión y se crea otra.
- **Petición a medias al revocar:** lo que ya se guardó se queda; la siguiente petición se rechaza.
- **Cambio de estado no permitido por el flujo:** se rechaza con el mismo error que en la interfaz (RF-28).
- **El cliente intenta borrar:** no hay herramienta para ello (RF-31); si la persona lo pide, el cliente solo puede decirle que lo haga en Kairos.
- **Texto con HTML o formato no permitido:** se limpia con la lista blanca (RF-35) y nunca se guarda HTML.
- **Elemento inexistente o que la persona no puede ver:** el mismo error de «no encontrado» en los dos casos, para no revelar que existe.
- **Varias tareas con el mismo nombre:** la búsqueda las devuelve todas con su proyecto, para que el cliente pregunte a la persona cuál es.
- **Acceso por MCP desactivado a media sesión:** la siguiente petición se rechaza (RF-3).

## Fuera de alcance
- Conectar iniciando sesión en Kairos desde el cliente con una pantalla de consentimiento (OAuth).
- Acciones irreversibles por IA: borrar, archivar o cancelar.
- Administración por IA: usuarios, equipos, atributos, homologaciones, catálogo, gamificación, auditoría y Sistema.
- Objetivos, que por ahora están ocultos.
- Conexiones de solo lectura o con menos permisos que los de la persona.
- Un permiso para decidir qué roles pueden crear conexiones.
- Que Kairos lleve un asistente de IA propio o llame a modelos de IA.
- Notificaciones o eventos enviados al cliente de IA por iniciativa de Kairos.
- Subir imágenes o archivos desde el cliente de IA.
- Vistas guardadas, Kanban y cronograma como tales: el cliente consulta los datos, no las vistas.
- Límites de uso distintos por persona o por cliente.

## Criterios de finalización
- Todos los RF con test en verde:
  - backend: apitest de las herramientas con un usuario de cada rol, del rechazo por permisos, de crear, caducar y revocar conexiones y del interruptor;
  - fila del permiso de administrar el sistema en la matriz para las conexiones de todos;
  - frontend: Mi cuenta (conexiones e instrucciones) y Sistema.
- Demo manual:
  - pegar la URL en un conector personalizado de Claude.ai, consultar mi panel y cerrar una tarea; el historial muestra «vía IA»;
  - pegar otra URL en ChatGPT o Claude Code y crear una tarea;
  - revocar desde Sistema y comprobar que el cliente deja de funcionar.

## Decisiones tomadas con el usuario (2026-11-05)
1. El cliente de IA puede leer y modificar, con los permisos de la persona.
2. Cada conexión tiene su propia URL, que se pega en cualquier cliente. No hay pasos propios de cada cliente ni pantalla de consentimiento.
3. Las acciones irreversibles quedan fuera del MCP.
4. El administrador del sistema activa o desactiva el acceso y ve y revoca las conexiones de todos.
5. Los cambios cuentan como de la persona, puntos incluidos, con la marca «vía IA».
6. Solo cubre el trabajo de proyectos; la administración queda fuera.

Propuestas mías que van en el borrador, para confirmar:
- acceso desactivado por defecto (RF-1);
- caducidad a 30, 90 o 365 días, 90 por defecto (RF-9);
- máximo 10 conexiones por persona (RF-14);
- 120 peticiones por minuto por conexión (RF-47);
- 50 elementos por página (RF-39).

## Dudas abiertas
- Ninguna.
