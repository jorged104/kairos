# Spec 017 — Notificaciones

**Estado:** Aprobada (2026-11-05) («dámole»).

## Contexto y objetivo
Hoy Kairos no avisa de nada. Para enterarse de que alguien comentó, de que le mencionaron o de que cambió una tarea suya, la persona tiene que entrar a mirar. Todas las specs anteriores dejaron fuera las notificaciones de forma explícita: la 005 (menciones), la 006 (comentarios), la 007, la 008, la 012 y la 013.

Esta spec añade notificaciones dentro de Kairos para tres sucesos:
- comentarios;
- menciones;
- cambios en las tareas de la persona.

Las notificaciones llegan en tiempo real a quien tiene Kairos abierto, sin recargar la página. Se ven en una campana con contador, en un aviso emergente y en una página con el historial.

Cada persona controla qué tareas sigue. Varios sucesos de una misma tarea se agrupan para no saturar.

Es neutro respecto al sector: vale para cualquier organización.

## Usuarios / actores
- **Cualquier usuario activo:** recibe notificaciones, las lee y sigue o deja de seguir tareas.
- **Autor del suceso:** quien comenta, menciona o cambia algo. Puede hacerlo desde la interfaz o por una conexión de IA (spec 016). Nunca recibe aviso de lo que hace él mismo.

## Vocabulario
- **Notificación:** un aviso para una persona sobre un elemento (una tarea, un hito o un proyecto). Puede estar leída o no leída.
- **Suceso:** lo que provoca un aviso:
  - un comentario nuevo (raíz o respuesta);
  - una mención;
  - un cambio de estado de una tarea;
  - un hilo que se resuelve o se reabre.
- **Seguidor de una tarea:** quien recibe los comentarios y los cambios de esa tarea.
- **Participante de un hilo:** quien escribió el comentario raíz o alguna respuesta de ese hilo.
- **Agrupar:** mientras una persona tenga sin leer una notificación de un elemento, los sucesos nuevos de ese elemento se suman a ella en vez de crear otra.

## Historias de usuario
- H1: Como persona asignada quiero enterarme cuando alguien comenta en mi tarea para responder a tiempo.
- H2: Como participante de un hilo quiero saber cuándo me responden.
- H3: Como cualquier persona quiero saber cuándo me mencionan, esté donde esté la mención.
- H4: Como persona asignada quiero saber cuándo cambia de estado una tarea mía sin que yo la toque.
- H5: Como persona interesada en una tarea que no es mía quiero seguirla, y dejar de seguir las que no me interesan.
- H6: Como cualquier persona quiero ver los avisos al momento, sin recargar, y que varios comentarios seguidos de una tarea no me llenen la lista.

## Requisitos funcionales (criterios de aceptación en EARS)

### Seguir tareas
- RF-1: CUANDO se asigne a una persona a una tarea, EL SISTEMA hará que esa persona siga la tarea.
- RF-2: EL SISTEMA mostrará en la ficha de la tarea si la persona la sigue, con un botón para seguirla o dejar de seguirla.
- RF-3: EL SISTEMA dejará seguir una tarea a cualquier persona que pueda verla.
- RF-4: CUANDO una persona deje de seguir una tarea, EL SISTEMA dejará de enviarle notificaciones de comentarios y de cambios de esa tarea, aunque esté asignada o participe en sus hilos.
- RF-5: CUANDO se asigne de nuevo a una persona que había dejado de seguir la tarea, EL SISTEMA hará que vuelva a seguirla.
- RF-6: CUANDO se quite a una persona de los asignados, EL SISTEMA no cambiará si la sigue o no.

### Qué genera una notificación y para quién
- RF-7: CUANDO se publique un comentario raíz en una tarea, EL SISTEMA notificará a sus seguidores.
- RF-8: CUANDO se publique una respuesta en un hilo, EL SISTEMA notificará a los seguidores de la tarea.
- RF-9: CUANDO se publique una respuesta en un hilo, EL SISTEMA notificará también a los participantes de ese hilo que no hayan dejado de seguir la tarea.
- RF-10: CUANDO un comentario mencione a una persona con `@`, EL SISTEMA le notificará la mención.
- RF-11: CUANDO al guardar la descripción de una tarea, un hito o un proyecto aparezca la mención a una persona que antes no estaba, EL SISTEMA le notificará la mención.
- RF-12: EL SISTEMA notificará las menciones aunque la persona no siga la tarea o la haya dejado de seguir.
- RF-13: CUANDO cambie el estado de una tarea, EL SISTEMA notificará a sus seguidores el estado anterior y el nuevo.
- RF-14: CUANDO se resuelva o se reabra un hilo, EL SISTEMA notificará a sus participantes que no hayan dejado de seguir la tarea.
- RF-15: EL SISTEMA no notificará a nadie de un suceso que ha provocado esa misma persona.
- RF-16: EL SISTEMA no notificará a personas inactivas ni a personas que no puedan ver el elemento.
- RF-17: SI un mismo suceso da a una persona varias razones para recibir aviso, ENTONCES EL SISTEMA le enviará un solo aviso; si una de las razones es una mención, lo contará como mención.
- RF-18: EL SISTEMA notificará igual los sucesos hechos por una conexión de IA, con la marca «vía IA» de la spec 016.
- RF-19: EL SISTEMA no notificará la edición ni el borrado de comentarios.
- RF-20: EL SISTEMA no notificará los cambios de campos distintos del estado.

### Agrupar
- RF-21: MIENTRAS una persona tenga sin leer una notificación de un elemento, EL SISTEMA sumará a esa notificación los sucesos nuevos del mismo elemento en vez de crear otra.
- RF-22: CUANDO se sume un suceso a una notificación, EL SISTEMA la pondrá la primera de la lista con la hora del último suceso.
- RF-23: EL SISTEMA mostrará en cada notificación:
  - el elemento: clave y título de la tarea, o nombre del hito o del proyecto;
  - quién lo hizo: hasta 2 nombres, y «y N más» si hay más personas;
  - cuántos sucesos agrupa;
  - qué fue el último suceso;
  - si incluye una mención;
  - la hora del último suceso.
- RF-24: EL SISTEMA mostrará del último comentario sus primeros 140 caracteres de texto.
- RF-25: CUANDO una persona lea una notificación, EL SISTEMA abrirá una notificación nueva con el siguiente suceso de ese elemento.

### Dónde se ven
- RF-26: EL SISTEMA mostrará en la barra lateral una campana con el número de notificaciones sin leer. Si pasan de 99, mostrará «99+».
- RF-27: CUANDO la persona pulse la campana, EL SISTEMA desplegará sus 20 notificaciones más recientes, primero las no leídas.
- RF-28: EL SISTEMA ofrecerá una página de notificaciones con todo el historial, filtrable entre todas y solo no leídas.
- RF-29: La página de notificaciones mostrará el historial por páginas de 50.
- RF-30: CUANDO llegue una notificación nueva o se sume un suceso a una, EL SISTEMA mostrará un aviso emergente durante 6 s, que se puede cerrar y que lleva al elemento al pulsarlo.
- RF-31: MIENTRAS la persona tenga abierta la ficha de ese mismo elemento, EL SISTEMA no le mostrará el aviso emergente de ese elemento.
- RF-32: SI llegan más de 3 avisos emergentes a la vez, ENTONCES EL SISTEMA mostrará solo uno con el número de notificaciones nuevas.

### Tiempo real
- RF-33: MIENTRAS la persona tenga Kairos abierto, EL SISTEMA le hará llegar cada notificación en menos de 5 s desde el suceso, sin recargar la página.
- RF-34: EL SISTEMA actualizará a la vez el contador y la lista en todas las pestañas abiertas de esa persona.
- RF-35: CUANDO la persona lea notificaciones en una pestaña, EL SISTEMA actualizará el contador en sus demás pestañas.
- RF-36: SI se corta la conexión en tiempo real, ENTONCES EL SISTEMA intentará reconectar.
- RF-37: CUANDO la conexión en tiempo real se recupere, EL SISTEMA actualizará el contador y la lista.
- RF-38: MIENTRAS no haya conexión en tiempo real, EL SISTEMA actualizará el contador al menos cada 60 s.

### Leer
- RF-39: CUANDO la persona pulse una notificación, EL SISTEMA abrirá el elemento.
- RF-40: CUANDO la persona pulse una notificación, EL SISTEMA marcará esa notificación como leída.
- RF-41: CUANDO el último suceso de una notificación sea un comentario, EL SISTEMA llevará a ese comentario dentro del elemento.
- RF-42: EL SISTEMA permitirá marcar una notificación como leída o como no leída sin abrir el elemento.
- RF-43: EL SISTEMA permitirá marcar todas las notificaciones como leídas.
- RF-44: EL SISTEMA borrará las notificaciones cuando pasen 90 días desde su último suceso.
- RF-45: SI la persona ya no puede ver el elemento de una notificación, o el elemento se borró, ENTONCES EL SISTEMA no mostrará esa notificación ni la contará.

## Requisitos no funcionales
- **Sin infraestructura nueva:** se usan la base de datos, la cola de eventos de dominio y la conexión en tiempo real que Kairos ya tiene.
- **Rendimiento:**
  - el contador de no leídas responde en menos de 200 ms con 10.000 notificaciones de la persona;
  - generar las notificaciones no retrasa el guardado del comentario ni del cambio que las provoca.
- **Privacidad:** una notificación solo muestra lo que su destinatario puede ver del elemento (RF-16, RF-45).
- **Idioma:** todos los textos salen de `es.json`.
- **Accesibilidad:**
  - el aviso emergente se anuncia a los lectores de pantalla sin robar el foco;
  - la campana dice cuántas notificaciones hay sin leer.

## Casos límite
- **Quitar y volver a poner una mención en una descripción:** no se avisa otra vez, salvo que la mención desaparezca y vuelva en guardados distintos.
- **Mencionar a alguien que no puede ver la tarea:** no se le avisa (RF-16), ni se le revela que existe.
- **Comentario borrado antes de que lo lean:** la notificación se queda, pero lleva a la tarea en vez de al comentario, y no muestra el texto borrado.
- **Persona asignada y mencionada en el mismo comentario:** recibe un solo aviso, que cuenta como mención (RF-17).
- **Cambio de estado hecho por un hito o por un movimiento en bloque:** cada tarea que cambia avisa a sus seguidores, agrupado por tarea.
- **Mover una tarea en el Kanban y devolverla enseguida:** se avisa de los dos cambios, agrupados en la misma notificación si no se ha leído.
- **Pestaña en segundo plano durante horas:** al volver, la conexión se recupera y el contador se pone al día (RF-37).
- **Persona desactivada:** deja de recibir avisos (RF-16), y su conexión en tiempo real se cierra como ya se hace hoy.
- **Proyecto cerrado o archivado:** no hay sucesos nuevos porque no se puede comentar ni cambiar nada; las notificaciones existentes se siguen viendo.

## Fuera de alcance
- Correo, notificaciones del navegador o del sistema operativo, móvil o chat.
- Notificaciones de asignaciones y de cambios en campos distintos del estado.
- Comentarios en hitos y proyectos, que no existen.
- Seguir hitos o proyectos enteros.
- Preferencias por tipo de notificación, horarios de silencio o resúmenes diarios.
- Notificaciones de vencimientos, retrasos, bloqueos, puntos o niveles.
- Notificaciones a través del MCP (spec 016).
- Avisos de sucesos ocurridos antes de esta funcionalidad.

## Criterios de finalización
- Todos los RF con test en verde:
  - backend: apitest de quién recibe cada suceso, de seguir y dejar de seguir, de agrupar, de leer, de la privacidad y del tiempo real por la conexión existente;
  - frontend: campana, lista, página, aviso emergente y botón Seguir.
- Demo manual con dos personas en dos navegadores:
  - una comenta, menciona y mueve una tarea, y a la otra le llega en menos de 5 s;
  - la otra deja de seguir la tarea y ya no le llegan sus comentarios, pero sí las menciones.

## Decisiones tomadas con el usuario (2026-11-05)
1. Sucesos: comentarios, menciones y cambios en las tareas (estado, hilo resuelto o reabierto). Las asignaciones quedan fuera.
2. Reciben los comentarios los asignados, los participantes del hilo y los seguidores.
3. Se ven en una campana con contador, en un aviso emergente y en una página de historial.
4. El único ajuste es seguir o dejar de seguir cada tarea. Las menciones llegan siempre.
5. Se agrupan por elemento mientras la notificación no se lea.
6. Una notificación se marca leída al abrirla, o a mano, y se borra a los 90 días.

Propuestas mías que van en el borrador, para confirmar:
- quien queda asignado sigue la tarea automáticamente (RF-1, RF-5), y quien deja de estar asignado no deja de seguirla (RF-6);
- el autor de la tarea no la sigue automáticamente: no lo elegiste como destinatario;
- extracto del comentario de 140 caracteres (RF-24);
- aviso emergente de 6 s, que se agrupa si llegan más de 3 a la vez (RF-30, RF-32);
- tiempo real en menos de 5 s, y sin conexión, una actualización cada 60 s (RF-33, RF-38).

## Dudas abiertas
- Ninguna.
