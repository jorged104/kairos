# Spec 006 — Tallas, varios asignados y comentarios en work items

**Estado:** Aprobada (2026-10-29). Las dudas se cerraron con las respuestas propuestas («Dale, implementa»).

## Contexto y objetivo
Hoy un work item tiene un peso (spec 004, RF-149 a RF-156), pero no hay una forma rápida y
común de estimarlo: cada persona escribe un número. Además solo admite un asignado, aunque
muchas tareas las hacen varias personas a la vez. Y la conversación sobre una tarea vive fuera
de Kairos (chat, correo), separada del work item y de su página, así que se pierde el porqué de
las decisiones. Esta spec añade **tallas** (XS a XL) que fijan el peso, **varios asignados** por
work item, todos con el mismo papel, y **comentarios**: hilos con respuestas que se pueden
resolver, en la ficha justo debajo de las propiedades, como en Notion, y también anclados a un
trozo de texto de la página.

## Usuarios / actores
- **Participante del proyecto** con permiso de escritura sobre el work item: cambia la talla, el peso y los asignados.
- **Cualquier usuario con acceso al work item salvo el rol `viewer`**: comenta, responde, resuelve hilos y comenta trozos de la página, aunque no pueda editar el work item.
- **Usuario `viewer`:** lee los comentarios y ve los trozos comentados.
- **Administrador del sistema:** configura las tallas del catálogo.

## Conceptos
- **Talla:** una estimación de tamaño del catálogo (XS, S, M, L, XL…), con un orden y un peso asociado.
- **Asignados:** la lista de personas que trabajan en un work item. Ninguna es principal.
- **Hilo:** un comentario raíz y sus respuestas. Puede estar abierto o resuelto.
- **Comentario anclado:** un hilo asociado a un trozo de texto de la página del work item, que se ve resaltado en ella.

## Historias de usuario
- H1: Como participante quiero elegir una talla para estimar sin discutir un número, y que el peso se rellene solo.
- H2: Como administrador quiero definir las tallas y su peso para que todo el equipo estime con la misma escala.
- H3: Como participante quiero asignar un work item a varias personas para reflejar que lo hacen juntas.
- H4: Como participante quiero encontrar los work items en los que estoy asignado aunque haya más personas.
- H5: Como colaborador quiero comentar un work item justo debajo de sus propiedades para dejar la conversación junto a la tarea.
- H6: Como colaborador quiero responder a un comentario y resolver el hilo cuando se cierra la cuestión, para que la ficha no se llene de conversaciones acabadas.
- H7: Como colaborador quiero seleccionar un trozo de la descripción y comentarlo para señalar exactamente a qué me refiero.

## Requisitos funcionales (criterios de aceptación en EARS)

### Tallas
- RF-1: EL SISTEMA permitirá a un administrador crear, renombrar, reordenar y retirar tallas en el catálogo, cada una con un nombre único (ignorando mayúsculas y espacios exteriores) y un peso entero de 0 a 100.
- RF-2: EL SISTEMA creará el catálogo con las tallas XS, S, M, L y XL, de pesos 1, 2, 3, 5 y 8.
- RF-3: SI el peso de una talla no es un entero de 0 a 100, ENTONCES EL SISTEMA rechazará el cambio.
- RF-4: EL SISTEMA guardará de cada work item una talla opcional del catálogo.
- RF-5: CUANDO el usuario elija una talla para un work item, EL SISTEMA pondrá como peso del work item el peso de esa talla.
- RF-6: CUANDO el usuario cambie a mano el peso de un work item con talla, EL SISTEMA conservará la talla y el peso nuevo.
- RF-7: CUANDO el usuario quite la talla de un work item, EL SISTEMA conservará su peso.
- RF-8: CUANDO cambie el peso de una talla en el catálogo, EL SISTEMA no cambiará el peso de los work items existentes con esa talla.
- RF-9: SI una talla está retirada, ENTONCES EL SISTEMA no la ofrecerá para work items nuevos ni cambios, y la seguirá mostrando en los work items que ya la tienen.
- RF-10: EL SISTEMA mostrará la talla y el peso entre las propiedades principales de la ficha del work item, siempre a la vista (cambia RF-68 de la spec 005).
- RF-56: EL SISTEMA mostrará la talla en una columna del árbol, editable en la celda como las demás (RF-139 de la spec 004).
- RF-11: CUANDO se copie un work item desde una plantilla, EL SISTEMA copiará su talla.

### Varios asignados
- RF-12: EL SISTEMA guardará de cada work item una lista de cero o más asignados distintos, todos con el mismo papel. Sustituye al asignado único de la spec 004 (RF-61).
- RF-13: SI un cambio deja más de 20 asignados en un work item, ENTONCES EL SISTEMA lo rechazará.
- RF-14: SI alguno de los asignados que se añaden está inactivo, dado de baja o solo tiene el rol `viewer`, ENTONCES EL SISTEMA rechazará el cambio entero (extiende RF-68 de la spec 004 a cada asignado).
- RF-15: CUANDO un asignado se desactive o se dé de baja, EL SISTEMA lo conservará en las listas de asignados existentes (como RF-69 de la spec 004).
- RF-16: CUANDO se actualice la aplicación, EL SISTEMA convertirá el asignado único de cada work item existente en una lista con esa persona.
- RF-17: EL SISTEMA permitirá añadir y quitar asignados en la ficha del work item y en la celda de asignados del árbol, y guardará cada cambio sin botón de guardar.
- RF-18: EL SISTEMA mostrará en el árbol las iniciales o el avatar de hasta 3 asignados y, si hay más, un indicador «+N» con el nombre de todos al pasar el ratón.
- RF-19: CUANDO se filtre el listado de work items por asignado, EL SISTEMA devolverá los work items en los que esa persona esté entre los asignados (cambia RF-129 de la spec 004).
- RF-20: EL SISTEMA incluirá la lista completa de asignados en el momento del evento en cada evento de work item, también en `work_item.closed` (cambia RF-122 de la spec 004).
- RF-21: EL SISTEMA auditará cada cambio de la lista de asignados con las personas añadidas y quitadas.

### Comentarios en la ficha
- RF-22: EL SISTEMA mostrará los comentarios del work item en su ficha, justo debajo de las propiedades y encima de la página, tanto en el panel lateral como en la página completa.
- RF-23: DONDE el usuario tenga acceso al work item y un rol distinto de `viewer`, EL SISTEMA le permitirá crear un hilo, responder a un hilo y resolver o reabrir un hilo.
- RF-24: MIENTRAS el usuario solo tenga el rol `viewer`, EL SISTEMA le mostrará los comentarios sin el cuadro para escribir ni los botones de responder, resolver, editar o borrar.
- RF-25: SI el usuario intenta comentar sin permiso, ENTONCES EL SISTEMA rechazará la operación con un código de prohibido sin guardar nada.
- RF-26: EL SISTEMA admitirá en un comentario el mismo texto enriquecido básico que las descripciones, menciones con `@` y `#` (solo enlace, como RF-25 a RF-30 de la spec 005) e imágenes y adjuntos con los límites de la spec 005 (RF-14 a RF-24).
- RF-27: SI un comentario está vacío o supera los 10 000 caracteres de texto, ENTONCES EL SISTEMA no lo guardará e indicará el motivo.
- RF-28: EL SISTEMA admitirá respuestas a un comentario raíz, en un solo nivel: responder a una respuesta añade otra respuesta al mismo hilo.
- RF-29: EL SISTEMA mostrará los hilos abiertos ordenados del más antiguo al más reciente, y las respuestas de cada hilo en el mismo orden.
- RF-30: EL SISTEMA mostrará en cada comentario su autor, la fecha y hora, y la marca «editado» si se cambió después de publicarlo.
- RF-31: CUANDO el autor edite su comentario, EL SISTEMA guardará el texto nuevo y lo marcará como editado.
- RF-32: CUANDO el autor borre un comentario sin respuestas, EL SISTEMA lo quitará del hilo.
- RF-33: CUANDO el autor borre el comentario raíz de un hilo con respuestas, EL SISTEMA mostrará «Comentario eliminado» en su lugar y conservará las respuestas.
- RF-34: DONDE el usuario pueda gestionar el proyecto, EL SISTEMA le permitirá borrar comentarios de otras personas.
- RF-35: SI el usuario intenta editar un comentario de otra persona, ENTONCES EL SISTEMA lo rechazará.
- RF-36: CUANDO el usuario resuelva un hilo, EL SISTEMA lo plegará bajo «N hilos resueltos» con quién y cuándo lo resolvió.
- RF-37: CUANDO el usuario despliegue los hilos resueltos y reabra uno, EL SISTEMA lo volverá a mostrar entre los abiertos.
- RF-38: CUANDO alguien responda a un hilo resuelto, EL SISTEMA lo reabrirá.
- RF-39: MIENTRAS el proyecto no esté activo (cerrado o archivado), EL SISTEMA mostrará los comentarios en modo lectura y rechazará cualquier cambio en ellos.
- RF-40: CUANDO se borre un work item, EL SISTEMA borrará sus comentarios y marcará sus archivos como borrados, como hace con los de su página (RF-49 de la spec 005).
- RF-41: CUANDO se copie un work item desde una plantilla, EL SISTEMA no copiará sus comentarios.
- RF-42: EL SISTEMA auditará la creación, la edición, el borrado, la resolución y la reapertura de cada comentario o hilo, sin guardar el texto en la auditoría.
- RF-43: CUANDO el usuario publique, edite, borre, resuelva o reabra un comentario, EL SISTEMA lo reflejará en la ficha sin recargar la página.

### Comentarios en la página
- RF-44: CUANDO un usuario que puede comentar seleccione texto de la página del work item, EL SISTEMA le ofrecerá «Comentar» junto a la selección.
- RF-45: CUANDO el usuario publique ese comentario, EL SISTEMA creará un hilo anclado a ese trozo de texto y lo resaltará en la página.
- RF-46: EL SISTEMA permitirá anclar comentarios a quien puede comentar aunque no pueda editar la página, sin dejarle cambiar el resto de la página.
- RF-47: CUANDO el usuario haga clic en un texto resaltado, EL SISTEMA le mostrará su hilo junto al texto.
- RF-48: EL SISTEMA mostrará los hilos anclados también en la lista de comentarios de la ficha, con la cita del texto comentado, y al pulsar la cita llevará al texto en la página.
- RF-49: MIENTRAS un hilo anclado esté resuelto, EL SISTEMA no resaltará su texto.
- RF-50: CUANDO se edite el texto comentado sin borrarlo entero, EL SISTEMA mantendrá el anclaje en lo que quede de él.
- RF-51: SI se borra entero el texto comentado, ENTONCES EL SISTEMA conservará el hilo en la lista de la ficha, con la cita original y la marca «texto eliminado», sin resaltar nada.
- RF-52: SI se deshace el borrado del texto antes de guardar la página, ENTONCES EL SISTEMA recuperará el anclaje.
- RF-53: CUANDO se copie o duplique texto comentado, EL SISTEMA no duplicará el anclaje: el hilo sigue unido solo al texto original.
- RF-54: SI dos trozos comentados se solapan, ENTONCES EL SISTEMA resaltará los dos y, al hacer clic en el solape, ofrecerá elegir el hilo.
- RF-55: MIENTRAS el usuario solo tenga el rol `viewer`, EL SISTEMA le mostrará el resaltado y el hilo sin opción de comentar.

## Requisitos no funcionales
- **Rendimiento** (entorno local): la ficha de un work item con 200 comentarios en 50 hilos carga sus comentarios en < 300 ms. Publicar un comentario responde en < 200 ms. La página con 50 trozos comentados se pinta dentro del límite de 1 s de la spec 005.
- **Seguridad:** los comentarios se sanean en el servidor con la misma lista blanca que las descripciones y nunca se guarda ni se pinta HTML (ADR 0003). Los archivos de los comentarios se sirven con las mismas comprobaciones que los de las páginas (spec 005, RF-23 y RF-24). Las operaciones comprueban permisos, nunca nombres de rol.
- **Compatibilidad de despliegue:** la versión anterior de la API sigue leyendo y guardando el asignado de los work items mientras conviven las dos versiones (regla de migraciones de `AGENTS.md`).
- **Plataformas:** navegadores de escritorio actuales. En móvil se leen y escriben comentarios de la ficha; comentar un trozo de la página es solo de escritorio.
- **Idioma:** español por defecto; todos los textos y errores nuevos salen del catálogo de i18n.

## Casos límite
- **Talla elegida y luego peso a mano:** la talla queda y el peso es el manual (RF-6). Para volver al peso de la talla se elige otra y luego la misma.
- **Talla retirada:** se ve en los work items que la tienen y no se ofrece (RF-9).
- **Work item sin asignados:** válido; en el árbol se ve «Sin asignar».
- **Misma persona dos veces:** la lista no admite duplicados; añadirla de nuevo no cambia nada.
- **Asignado desactivado:** se queda en la lista (RF-15) y no se puede añadir de nuevo (RF-14).
- **Dos personas cambian los asignados a la vez:** el segundo cambio responde `version_conflict`, como el resto de propiedades.
- **Comentario mientras otro edita la página:** comentar en la ficha no afecta a la página. Anclar un comentario no es un conflicto para quien está escribiendo.
- **Autor desactivado o dado de baja:** sus comentarios se quedan con su nombre.
- **Mención a algo sin acceso o borrado:** se muestra como en las páginas, «sin acceso» o «eliminado» (spec 005).
- **Hilo resuelto con texto borrado:** sigue entre los resueltos con la marca «texto eliminado».
- **Plantillas:** se copia la talla y no se copian ni los asignados (RF-37 de la spec 004) ni los comentarios (RF-41).

## Fuera de alcance
- Notificaciones de cualquier tipo: menciones, respuestas, asignaciones o hilos resueltos.
- Comentarios en proyectos, hitos u objetivos.
- Ver comentarios nuevos sin recargar mientras otra persona escribe (tiempo real).
- Reacciones con emoji, comentarios privados y vigilantes del work item.
- Anclar comentarios a imágenes, adjuntos o bloques enteros: solo a trozos de texto.
- Repartir horas, avance o XP entre los asignados.
- Filtrar o agrupar el árbol por talla.

## Criterios de finalización
- Todos los RF con tests en verde: API (permisos por rol en el test de matriz, conversión del asignado único, eventos con todos los asignados, comentarios y anclajes) y frontend (ficha, árbol y página).
- Prueba de rendimiento con 200 comentarios en 50 hilos dentro de los umbrales.
- Demo manual:
  1. Configurar las tallas, poner M a un work item y ver su peso.
  2. Asignar a tres personas y filtrar por una de ellas.
  3. Comentar, responder, resolver y reabrir un hilo.
  4. Seleccionar texto de la página, comentarlo, borrar el texto y ver «texto eliminado».
  5. Entrar como `viewer` y comprobar que solo lee.

## Decisiones tomadas con el usuario (2026-10-29)
- **Tallas:** fijan el peso al elegirlas; el peso se puede retocar después (RF-5, RF-6). Pesos iniciales 1, 2, 3, 5 y 8 (RF-2). Columna editable en el árbol (RF-56).
- **Asignados:** todos iguales, sin principal, con un máximo de 20 (RF-12, RF-13).
- **Comentarios:** hilos con un nivel de respuestas que se resuelven y se reabren (RF-28, RF-36 a RF-38). Comentan todos los roles menos `viewer` (RF-23). Quien gestiona el proyecto puede borrar los de otros (RF-34). En proyectos cerrados o archivados, solo lectura (RF-39).
- **Comentarios en la página:** anclados a un trozo de texto (RF-44 a RF-55).
- **Fuera:** notificaciones, comentarios en proyectos e hitos, y tiempo real.

## Dudas abiertas
Ninguna.
