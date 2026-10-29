# Spec 005 — Páginas tipo Notion para work items, proyectos e hitos

**Estado:** Aprobada (2026-10-29).

**Cambio:** 2026-10-29. Ficha del work item: propiedades editables donde se ven, borrar con la papelera, sin hitos ni dependencias, y panel lateral a media pantalla y redimensionable. RF-39 se sustituye por RF-60 a RF-67.

**Cambio:** 2026-10-29. Propiedades principales a la vista y el resto plegadas tras «Más propiedades»: RF-68 y RF-69.

## Contexto y objetivo
Hoy la descripción de un work item, un proyecto o un hito es un campo de texto enriquecido
dentro de su ficha (spec 004, RF-70 y RF-71). Tiene formato básico, se guarda con un botón y no
admite imágenes, adjuntos ni referencias a otros elementos. El equipo quiere que esa descripción
sea una **página**, como en Notion: un documento hecho de bloques que se insertan con `/`, se
arrastran para reordenarlos y admiten listas de tareas, bloques plegables, imágenes, archivos y
menciones. Se guarda solo mientras se escribe. Así el detalle de una tarea (criterios, capturas,
notas de reunión, enlaces a otros work items) vive junto a ella, en lugar de repartirse en
documentos externos.

## Usuarios / actores
- **Participante del proyecto** con permiso de escritura sobre el elemento: edita la página.
- **Usuario con acceso de lectura** al proyecto: lee la página, ve imágenes y descarga adjuntos.
- **Administrador del sistema:** configura dónde se guardan los archivos.

## Conceptos
- **Página:** la descripción de un work item, un proyecto o un hito, compuesta por bloques. Sustituye a la descripción actual y conserva su contenido.
- **Bloque:** una unidad de la página, como un párrafo, un título, una lista, una tabla o una imagen.
- **Almacenamiento de archivos:** dónde se guardan las imágenes y los adjuntos. Puede ser un almacenamiento de objetos compatible con S3 (por ejemplo MinIO) o la base de datos.

## Historias de usuario
- H1: Como participante quiero escribir `/` e insertar el bloque que necesito para dar formato sin buscar botones.
- H2: Como participante quiero arrastrar un bloque para reordenar la página sin cortar y pegar.
- H3: Como participante quiero pegar o subir capturas y adjuntar documentos para dejar el contexto junto a la tarea.
- H4: Como participante quiero listas de tareas y bloques plegables para organizar criterios y notas largas.
- H5: Como participante quiero mencionar a un compañero o a otro work item, hito o proyecto para enlazarlos.
- H6: Como participante quiero que la página se guarde sola para no perder lo escrito.
- H7: Como participante quiero abrir un work item en un panel junto al árbol, o como página completa, para leerlo sin perder el contexto o con todo el espacio.
- H8: Como administrador quiero guardar los archivos en S3 o MinIO cuando lo tenga, y en la base de datos cuando no.

## Requisitos funcionales (criterios de aceptación en EARS)

### Bloques
- RF-1: EL SISTEMA admitirá en una página los bloques que ya admite la descripción: párrafo, títulos, listas con viñetas y numeradas, código, tabla y enlaces. También las marcas de negrita, cursiva y código en línea.
- RF-2: EL SISTEMA admitirá además listas de tareas con casilla, bloques plegables, citas, avisos destacados (callout) y separadores.
- RF-3: CUANDO el usuario marque o desmarque la casilla de una lista de tareas, EL SISTEMA guardará el cambio como cualquier otra edición de la página.
- RF-4: CUANDO el usuario pliegue o despliegue un bloque plegable, EL SISTEMA mostrará u ocultará su contenido sin guardar el cambio. Al abrir la página, todos aparecerán plegados.
- RF-5: EL SISTEMA admitirá bloques de imagen y de archivo adjunto (RF-14 a RF-24 y RF-49 a RF-55).
- RF-6: EL SISTEMA admitirá menciones de usuarios, work items, hitos y proyectos (RF-25 a RF-30).

### Edición por bloques
- RF-7: CUANDO el usuario escriba `/` al principio de un bloque vacío o tras un espacio, EL SISTEMA abrirá un menú con los bloques que se pueden insertar.
- RF-8: MIENTRAS el menú de `/` esté abierto, EL SISTEMA filtrará sus opciones por el texto escrito tras `/` y permitirá elegirlas con las flechas y Enter.
- RF-9: CUANDO el usuario pulse Escape con el menú de `/` abierto, EL SISTEMA cerrará el menú y dejará el texto escrito.
- RF-10: CUANDO el usuario pase el puntero sobre un bloque, EL SISTEMA mostrará junto a él un asa para arrastrarlo.
- RF-11: CUANDO el usuario arrastre un bloque por su asa y lo suelte en otra posición, EL SISTEMA lo moverá ahí con todo su contenido, incluidos los bloques anidados.
- RF-12: CUANDO el usuario abra el menú del asa, EL SISTEMA ofrecerá convertir el bloque en otro tipo compatible, duplicarlo y borrarlo.
- RF-56: CUANDO el usuario escriba al principio de un bloque `# `, `## ` o `### `, `- `, `1. `, `[] `, `> `, `---` o tres acentos graves, EL SISTEMA lo convertirá en título del nivel correspondiente, lista con viñetas, lista numerada, lista de tareas, cita, separador o bloque de código.
- RF-57: EL SISTEMA admitirá bloques plegables dentro de otros plegables, y los bloques de RF-2 y RF-5 dentro de celdas de tabla y de plegables, hasta el límite de 20 niveles de anidamiento de la descripción actual.
- RF-58: SI un cambio superaría los 20 niveles de anidamiento, ENTONCES EL SISTEMA no lo aplicará.
- RF-13: MIENTRAS el usuario no tenga permiso de escritura sobre el elemento, EL SISTEMA mostrará la página en modo lectura, sin menú `/` ni asas, y con las casillas de las listas de tareas deshabilitadas.

### Imágenes y archivos
- RF-14: CUANDO el usuario suba, pegue o arrastre una imagen PNG, JPG, GIF o WebP sobre la página, EL SISTEMA la guardará y la insertará como bloque de imagen visible en la página.
- RF-15: CUANDO el usuario suba o arrastre un archivo PDF, de Office (Word, Excel o PowerPoint), TXT, CSV o ZIP, EL SISTEMA lo guardará y lo insertará como bloque de adjunto con su nombre, tamaño y un enlace de descarga.
- RF-16: SI el archivo supera 10 MB, ENTONCES EL SISTEMA lo rechazará e indicará el límite.
- RF-17: SI el tipo de archivo no es uno de RF-14 o RF-15, ENTONCES EL SISTEMA lo rechazará e indicará los tipos admitidos. Esto incluye SVG, HTML y ejecutables.
- RF-18: EL SISTEMA determinará el tipo del archivo por su contenido, no solo por su extensión ni por lo que declare el navegador.
- RF-19: MIENTRAS se sube un archivo, EL SISTEMA mostrará su progreso en el bloque.
- RF-20: SI la subida falla, ENTONCES EL SISTEMA mostrará el error en el bloque y permitirá reintentarla o quitar el bloque.
- RF-21: DONDE esté configurado un almacenamiento de objetos compatible con S3, EL SISTEMA guardará ahí los archivos.
- RF-22: DONDE no esté configurado ese almacenamiento, EL SISTEMA guardará los archivos en la base de datos.
- RF-23: EL SISTEMA solo servirá una imagen o un adjunto a usuarios con acceso de lectura al elemento cuya página lo contiene.
- RF-24: EL SISTEMA servirá los adjuntos como descarga y nunca para que el navegador los ejecute en la página de Kairos.
- RF-49: CUANDO se guarde una página que ya no contiene un archivo que tenía, EL SISTEMA marcará ese archivo como borrado y dejará de servirlo.
- RF-50: CUANDO se guarde una página que vuelve a contener un archivo suyo marcado como borrado (por ejemplo, al deshacer), EL SISTEMA quitará la marca y volverá a servirlo.
- RF-51: CUANDO se borre un work item, un proyecto o un hito, EL SISTEMA marcará como borrados los archivos de su página.
- RF-52: EL SISTEMA no eliminará del almacenamiento los archivos marcados como borrados. Los eliminará el mantenimiento periódico de una spec posterior.
- RF-53: CUANDO se cree un proyecto desde una plantilla, EL SISTEMA duplicará los archivos de las páginas copiadas, de modo que marcar como borrado un archivo de la copia no afecte a la plantilla.
- RF-54: CUANDO se configure el almacenamiento S3 habiendo archivos en la base de datos, EL SISTEMA seguirá sirviendo esos archivos desde la base de datos y guardará en S3 solo los nuevos.
- RF-55: EL SISTEMA no limitará el número de archivos de una página, solo el tamaño de cada uno (RF-16).

### Menciones
- RF-25: CUANDO el usuario escriba `@`, EL SISTEMA abrirá un buscador de usuarios activos por nombre o email.
- RF-26: CUANDO el usuario escriba `#`, EL SISTEMA abrirá un buscador de work items (por clave o título), hitos y proyectos a los que tenga acceso de lectura.
- RF-27: CUANDO el usuario elija un resultado, EL SISTEMA insertará la mención como un enlace que muestra el nombre actual del usuario, o la clave y el título actuales del elemento.
- RF-28: SI quien lee la página no tiene acceso al elemento mencionado, ENTONCES EL SISTEMA mostrará la mención como «sin acceso», sin revelar su título.
- RF-29: SI el usuario o el elemento mencionado ya no existe, ENTONCES EL SISTEMA mostrará la mención como «eliminado».
- RF-30: EL SISTEMA no enviará avisos a los usuarios mencionados.

### Guardado
- RF-31: CUANDO el usuario deje de escribir durante 2 segundos, EL SISTEMA guardará la página sin que el usuario pulse ningún botón.
- RF-32: CUANDO el usuario salga de la página o cierre el panel con cambios sin guardar, EL SISTEMA los guardará antes de cerrar.
- RF-33: EL SISTEMA mostrará en la página si hay cambios sin guardar, si se está guardando o si ya está guardada.
- RF-34: SI otra persona cambió la página desde que el usuario la abrió, ENTONCES EL SISTEMA no sobrescribirá su cambio, avisará al usuario y conservará en pantalla el texto del usuario para que pueda copiarlo antes de recargar.
- RF-59: CUANDO otra persona cambie otra propiedad del elemento (estado, asignado, fechas…) mientras el usuario edita la página, EL SISTEMA guardará la página sin tratarlo como conflicto y sin deshacer ese cambio.
- RF-35: SI el guardado falla por un error de red, ENTONCES EL SISTEMA conservará los cambios en pantalla, mostrará el error y reintentará el guardado.
- RF-36: SI la página supera el límite de tamaño de la descripción (50 000 caracteres, spec 004 RF-70), ENTONCES EL SISTEMA no la guardará e indicará el límite.

### Vistas
- RF-37: CUANDO el usuario haga clic en un work item del árbol de un proyecto, EL SISTEMA abrirá su página en un panel lateral junto al árbol, sin salir de él.
- RF-38: CUANDO el usuario pulse «Abrir como página» en el panel, EL SISTEMA abrirá la página completa del work item en su URL propia.
- ~~RF-39: CUANDO el usuario abra la URL de un work item, EL SISTEMA mostrará la página completa: título editable, propiedades del work item (estado, tipo, prioridad, asignado, fechas, horas, avance, peso, padre, campos personalizados, hitos y dependencias) y, debajo, el cuerpo de la página.~~ Sustituida por RF-60.
- RF-60: CUANDO el usuario abra la URL de un work item, EL SISTEMA mostrará la página completa: título, propiedades del work item (estado, tipo, prioridad, asignado, padre, autor, fechas, horas, avance, peso y campos personalizados) y, debajo, el cuerpo de la página, sin las secciones de hitos ni de dependencias.
- RF-61: MIENTRAS el usuario tenga permiso de escritura sobre el work item, EL SISTEMA le dejará cambiar el título y cada propiedad, salvo el autor, en la propia página, sin un botón ni un modo de edición.
- RF-62: CUANDO el usuario elija un valor en una lista, o pulse Enter o salga de un campo cuyo valor cambió, EL SISTEMA guardará solo esa propiedad.
- RF-63: CUANDO el usuario pulse Escape en un campo de propiedad, EL SISTEMA devolverá el valor guardado y no cerrará el panel.
- RF-64: DONDE el usuario pueda borrar el work item, EL SISTEMA mostrará en la esquina superior izquierda de su página un botón con solo el icono de la papelera, y borrará el work item cuando se pulse por segunda vez en menos de 3 s.
- RF-65: CUANDO se abra el panel lateral, EL SISTEMA le dará la mitad del ancho de la ventana.
- RF-66: CUANDO el usuario arrastre el borde izquierdo del panel, o use las flechas del teclado sobre él, EL SISTEMA cambiará su ancho entre 360 px y el ancho de la ventana menos 120 px.
- RF-67: CUANDO se cierre el panel o el usuario haga doble clic en su borde, EL SISTEMA volverá a la mitad del ancho; el ancho elegido no se recuerda.
- RF-68: EL SISTEMA mostrará siempre en la página del work item su estado, asignado, prioridad, fecha de inicio y fecha de fin, y plegará el resto de propiedades tras un control «Más propiedades» que indica cuántas hay.
- RF-69: CUANDO el usuario pulse «Más propiedades», EL SISTEMA mostrará el resto de propiedades; al pulsarlo de nuevo, las volverá a plegar. Al abrir otra página empiezan plegadas.
- RF-40: CUANDO el usuario pulse «Ancho completo», EL SISTEMA ocultará la navegación lateral de la aplicación y extenderá la página a todo el ancho de la ventana. Al pulsarlo de nuevo, volverá a la vista normal.
- RF-41: EL SISTEMA recordará en el navegador del usuario si eligió el ancho completo.
- RF-42: CUANDO el usuario pulse Escape en el panel lateral sin ningún menú abierto, EL SISTEMA cerrará el panel.
- RF-43: EL SISTEMA mostrará la página de un proyecto en su ficha y la de un hito en la suya, con las mismas capacidades de edición que la de un work item.

### Compatibilidad
- RF-44: EL SISTEMA mostrará y permitirá editar las descripciones existentes como páginas sin perder contenido.
- RF-45: EL SISTEMA seguirá rechazando en una página HTML, scripts ejecutables y enlaces que no sean `http`, `https` o `mailto` (spec 004 RF-71).
- RF-46: EL SISTEMA seguirá buscando por el texto de la página, incluido el de los bloques nuevos y el nombre de los adjuntos, como hoy busca por la descripción.
- RF-47: EL SISTEMA seguirá auditando solo que la página cambió, sin guardar su contenido (spec 004 RF-135).
- RF-48: EL SISTEMA obtendrá todos los textos visibles de los menús, bloques, avisos y errores de esta spec del catálogo de traducciones.

## Requisitos no funcionales
- **Seguridad:** las imágenes y adjuntos se sirven tras comprobar la sesión y el permiso sobre el elemento (RF-23). Las URL de los archivos no dan acceso sin sesión. Los adjuntos se sirven con un tipo que el navegador no ejecuta (RF-24).
- **Rendimiento** (entorno local): abrir una página de 50 000 caracteres con 20 imágenes se pinta en < 1 s sin contar la descarga de las imágenes. Escribir no muestra retraso visible con esa página. Un autoguardado responde en < 300 ms.
- **Compatibilidad de despliegue:** la versión anterior de la API sigue leyendo y guardando descripciones mientras conviven las dos versiones (regla de migraciones de `AGENTS.md`).
- **Plataformas:** navegadores de escritorio actuales. En móvil, la página se lee y se edita con teclado, sin arrastrar bloques.
- **Idioma:** español por defecto (RF-48).

## Casos límite
- **Pegar contenido de otra web o de Notion:** solo se conserva lo admitido por RF-1, RF-2 y RF-45. Lo demás se convierte en texto.
- **Pegar una imagen que supera 10 MB o de un tipo no admitido:** se rechaza como en RF-16 y RF-17, y el resto de lo pegado se conserva.
- **Imagen quitada de la página:** se marca como borrada y deja de servirse (RF-49). Si se deshace y se vuelve a guardar, se recupera (RF-50).
- **Work item, proyecto o hito borrado:** sus archivos se marcan como borrados (RF-51). Ninguno se elimina del almacenamiento en esta spec (RF-52).
- **Copia desde plantilla (spec 004):** la copia tiene sus propios archivos duplicados (RF-53).
- **Cambio de almacenamiento:** al activar S3, los archivos ya guardados en la base de datos se quedan ahí (RF-54). Si después se quita la configuración de S3, los archivos guardados en S3 dejan de poder servirse hasta que vuelva.
- **Mención dentro de una página copiada de plantilla:** se copia tal cual, apuntando al mismo elemento.
- **S3 configurado pero caído:** la subida falla con error (RF-20), no se guarda en la base de datos como alternativa. Los archivos ya subidos no se pueden ver mientras dure.
- **Dos pestañas del mismo usuario:** se tratan como dos personas (RF-34).
- **Un cambio de estado, asignado u otra propiedad mientras alguien edita la página:** no es conflicto (RF-59).
- **Atajo de Markdown dentro de un bloque de código:** se queda como texto, sin convertir.
- **Mención a un work item de otro proyecto:** se admite si quien escribe tiene acceso. Quien lee sin acceso ve «sin acceso» (RF-28).
- **Página vacía:** se muestra un texto de ayuda que invita a escribir o pulsar `/`, solo a quien puede editar.
- **Bloques plegables anidados y bloques nuevos dentro de tablas:** se admiten hasta 20 niveles (RF-57, RF-58).
- **Arrastrar un bloque dentro de sí mismo o más allá de 20 niveles:** no se aplica (RF-58).

## Fuera de alcance
- Colaboración en tiempo real: cursores de otros usuarios y edición simultánea con fusión de cambios.
- Historial de versiones de la página y restauración.
- Subpáginas o páginas sueltas fuera de un work item, proyecto o hito.
- Página tipo Notion para objetivos y resultados clave.
- Embeds de vídeos, Figma, Google Docs u otras webs.
- Avisos o notificaciones por menciones (RF-30).
- Comentarios en bloques.
- Bases de datos o vistas de Notion dentro de la página.
- Exportar o importar páginas (PDF, Markdown, Notion).
- Editar imágenes: recortar o redimensionar más allá del ancho de la página.
- Arrastrar bloques en pantallas táctiles.
- Previsualizar PDF u Office dentro de la página: se descargan.
- Cuotas de almacenamiento por proyecto o usuario.
- Eliminar físicamente del almacenamiento los archivos marcados como borrados (RF-52), y migrar archivos entre la base de datos y S3.

## Criterios de finalización
- Todos los RF cubiertos por tests automáticos en verde: backend para guardado, conflictos, archivos (tipos, tamaño, permisos y los dos almacenamientos) y menciones sin acceso; frontend para el menú `/`, las asas, el autoguardado y las vistas.
- Demo manual:
  1. Abrir un work item desde el árbol en el panel, escribir `/`, insertar un checklist y un plegable, y arrastrar un bloque.
  2. Pegar una captura y adjuntar un PDF, con y sin S3 configurado.
  3. Mencionar a un usuario y a otro work item.
  4. Abrir la página completa y el ancho completo.
  5. Provocar un conflicto desde otra pestaña.
  6. Entrar con un usuario de solo lectura y comprobar el modo lectura y la mención «sin acceso».

## Decisiones tomadas con el usuario (2026-10-29)
- **Bloques nuevos:** listas de tareas, plegables, citas, avisos destacados y separadores; imágenes y archivos; menciones. Sin embeds.
- **Archivos:** en un almacenamiento compatible con S3 (por ejemplo MinIO) si está configurado y, si no, en la base de datos. Máximo 10 MB. Imágenes PNG, JPG, GIF y WebP visibles en la página; PDF, Office, TXT, CSV y ZIP como adjuntos. Sin SVG ni ejecutables.
- **Vistas:** panel lateral desde el árbol, página completa con URL propia y opción de ancho completo. El diseño visual se propone aparte con `/design`.
- **Guardado:** autoguardado con aviso de conflicto, sin colaboración en tiempo real.
- **Edición:** menú `/`, asas para arrastrar y menú de bloque.
- **Alcance:** work items, proyectos e hitos. Objetivos fuera.
- **Menciones:** solo enlace, sin avisos.
- **Autoguardado:** a los 2 s sin escribir (RF-31).
- **Archivos quitados o de elementos borrados:** solo se marcan como borrados. Su eliminación física la hará un mantenimiento periódico de otra spec (RF-49 a RF-52).
- **Plantillas:** la copia duplica los archivos (RF-53).
- **Activar S3:** los archivos existentes se quedan en la base de datos (RF-54).
- **Conflicto:** solo cuentan los cambios de la página (RF-34, RF-59).
- **Atajos de Markdown:** incluidos (RF-56).
- **Anidamiento:** permitido hasta 20 niveles (RF-57, RF-58).
- **Archivos por página:** sin límite de número (RF-55).
- **Componente de edición:** se busca uno ya hecho en lugar de construir el editor desde cero. Se sigue con TipTap 3, que ya usa el proyecto, y se añaden sus extensiones oficiales: asa de arrastre para Vue 3 (con sus dependencias obligatorias de colaboración, que no se usa), sugerencias para el menú `/` y los buscadores, menciones, plegables e imagen. En el backend se añade un cliente S3 compatible con MinIO. El detalle y la justificación de cada una están en el plan.

## Dudas abiertas
Ninguna.
