# Spec 020 — Informes de proyecto con IA

**Estado:** Aprobada (2026-11-05) («apruebo»), con el agente construido con deepagents.

## Contexto y objetivo
Preparar el informe de un proyecto obliga hoy a recorrer pantallas y a copiar a mano avances, hitos y tareas atrasadas. Si el proyecto es un programa con subproyectos, hay que repetirlo en cada uno.

Esta spec añade a cada proyecto una sección **Informes**. En ella, quien lleva el proyecto pide a un modelo de lenguaje (IA) que escriba un informe a partir de los datos reales del proyecto y de sus subproyectos.

**La IA:**
- consulta por sí misma proyectos, hitos y tareas;
- solo puede leer, nunca cambia nada;
- escribe el informe en Markdown.

**Quien genera el informe:**
- lo ajusta pidiendo cambios a la IA o editándolo a mano;
- lo guarda para que lo lea el resto del proyecto.

**El administrador del sistema:**
- conecta Kairos con cualquier servicio de IA que ofrezca una API compatible con la de OpenAI;
- define las plantillas de informe, cada una con su propio prompt.

Nada depende de un sector ni de un proveedor de IA concreto.

## Usuarios / actores
- **Generador de informes:** cualquier persona que puede gestionar el proyecto según la spec 004, es decir:
  - un administrador;
  - un líder, en los proyectos de su ámbito;
  - el responsable del proyecto (PM).

  Crea, modifica, guarda y borra los informes de ese proyecto.
- **Lector:** cualquier persona que puede ver el proyecto. Lee sus informes guardados.
- **Administrador del sistema** (permiso de administrar el sistema, spec 015):
  - configura la conexión con el servicio de IA;
  - gestiona las plantillas de informe.
- **Servicio de IA:** un servicio externo o propio que ofrece una API compatible con la de OpenAI.

## Vocabulario
- **Servicio de IA:** el modelo de lenguaje al que Kairos pide los informes. Se define con:
  - una dirección base;
  - una clave de API;
  - un modelo.
- **Plantilla de informe:** un tipo de informe que define el administrador, por ejemplo «Estado semanal» o «Cierre de hito». Tiene nombre, descripción y prompt.
- **Prompt:** las instrucciones que recibe la IA para escribir un tipo de informe.
- **Árbol del proyecto:** el proyecto y todos sus subproyectos, a cualquier profundidad.
- **Consulta:** cada lectura de datos que hace la IA mientras escribe.
- **Borrador:** el texto que se está generando o editando y que aún no se ha guardado.
- **Informe:** el texto guardado en el proyecto. Tiene:
  - título;
  - contenido en Markdown;
  - la plantilla con que se creó;
  - autor;
  - quién lo modificó por última vez y cuándo.

## Historias de usuario
- H1: Como administrador del sistema quiero conectar Kairos con el servicio de IA de mi organización, cualquiera que sea compatible con la API de OpenAI, para no depender de un proveedor.
- H2: Como administrador del sistema quiero definir plantillas de informe con su prompt, para que los informes sigan el formato de mi organización.
- H3: Como PM quiero generar un informe de mi proyecto o programa eligiendo una plantilla, para no recopilar los datos a mano.
- H4: Como PM quiero pedir cambios a la IA sobre el informe («añade los riesgos», «resume los subproyectos») y retocarlo a mano, para dejarlo como lo necesito.
- H5: Como PM quiero guardar el informe en el proyecto, para que el equipo lo lea.
- H6: Como miembro del proyecto quiero leer los informes guardados.

## Requisitos funcionales (criterios de aceptación en EARS)

### Servicio de IA (pantalla Sistema)
- RF-1: EL SISTEMA mostrará al administrador del sistema, en la pantalla Sistema, una pestaña «Informes con IA».
- RF-2: EL SISTEMA tendrá la generación de informes desactivada en una instalación nueva.
- RF-3: EL SISTEMA permitirá al administrador activar o desactivar la generación de informes.
- RF-4: EL SISTEMA permitirá al administrador indicar la dirección base de la API.
- RF-5: EL SISTEMA permitirá al administrador indicar la clave de API.
- RF-6: EL SISTEMA permitirá al administrador indicar el nombre del modelo.
- RF-7: EL SISTEMA guardará la clave de API cifrada.
- RF-8: EL SISTEMA no volverá a mostrar la clave de API; el campo quedará vacío e indicará que hay una guardada.
- RF-9: CUANDO el administrador guarde con el campo de la clave vacío, EL SISTEMA conservará la clave anterior.
- RF-10: EL SISTEMA permitirá dejar la clave vacía, para los servicios propios que no la piden.
- RF-11: EL SISTEMA ofrecerá un botón «Probar conexión» que haga una petición mínima al servicio, que incluya una consulta.
- RF-12: CUANDO la prueba funcione, EL SISTEMA lo indicará.
- RF-13: SI la prueba falla, ENTONCES EL SISTEMA indicará el motivo: no se pudo conectar, clave rechazada, modelo inexistente o el modelo no admite consultas.
- RF-14: EL SISTEMA mostrará en la pestaña un aviso de que los datos de los proyectos se envían al servicio de IA configurado.
- RF-15: EL SISTEMA auditará cada cambio de la configuración del servicio de IA, registrando la clave solo como «cambiada».
- RF-16: MIENTRAS la generación esté desactivada o el servicio sin configurar, EL SISTEMA ocultará los botones de generar y de pedir cambios a la IA.
- RF-17: MIENTRAS la generación esté desactivada o el servicio sin configurar, EL SISTEMA seguirá mostrando los informes guardados y dejará editarlos a mano.

### Plantillas de informe
- RF-18: EL SISTEMA permitirá al administrador del sistema crear plantillas de informe, cada una con nombre, descripción y prompt.
- RF-19: EL SISTEMA permitirá al administrador modificar las plantillas de informe.
- RF-20: EL SISTEMA permitirá al administrador archivar las plantillas de informe.
- RF-21: EL SISTEMA exigirá un nombre de 1 a 100 caracteres, único entre las plantillas activas sin distinguir mayúsculas.
- RF-22: EL SISTEMA exigirá un prompt de 1 a 20 000 caracteres.
- RF-23: EL SISTEMA incluirá en una instalación nueva una plantilla «Estado del proyecto» con un prompt neutro: avance frente al plan, hitos, tareas atrasadas, próximas entregas y riesgos.
- RF-24: CUANDO se modifique una plantilla, los informes ya guardados no cambiarán.
- RF-25: CUANDO se archive una plantilla, EL SISTEMA dejará de ofrecerla para informes nuevos.
- RF-26: CUANDO se archive una plantilla, los informes creados con ella conservarán su nombre.
- RF-27: EL SISTEMA auditará la creación, modificación y archivo de plantillas, registrando el prompt solo como «prompt cambiado».

### Sección Informes del proyecto
- RF-28: EL SISTEMA mostrará en la ficha de cada proyecto una pestaña «Informes» a quien pueda ver el proyecto.
- RF-29: EL SISTEMA listará los informes del proyecto, del modificado más recientemente al más antiguo.
- RF-30: EL SISTEMA mostrará de cada informe de la lista su título, su plantilla, su autor y la fecha de la última modificación.
- RF-31: CUANDO alguien abra un informe, EL SISTEMA mostrará su contenido Markdown con formato: títulos, listas, tablas, negrita, cursiva, enlaces y citas.
- RF-32: EL SISTEMA nunca interpretará HTML incluido en el contenido de un informe: lo mostrará como texto.
- RF-33: SI quien abre la pestaña no puede gestionar el proyecto, ENTONCES EL SISTEMA no le mostrará los botones de crear, modificar ni borrar.

### Generar un informe
- RF-34: CUANDO un generador pulse «Nuevo informe», EL SISTEMA le pedirá elegir una plantilla activa.
- RF-35: CUANDO un generador pulse «Nuevo informe», EL SISTEMA le dejará añadir instrucciones adicionales de hasta 2 000 caracteres.
- RF-36: CUANDO el generador confirme, EL SISTEMA pedirá el informe al servicio de IA con:
  - el prompt de la plantilla;
  - las instrucciones adicionales;
  - el nombre del proyecto;
  - la fecha del día.
- RF-37: MIENTRAS se genera, EL SISTEMA mostrará qué consulta está haciendo la IA, por ejemplo «Leyendo hitos de Proyecto X».
- RF-38: MIENTRAS se genera, EL SISTEMA mostrará la lista de pasos que la IA se ha marcado para escribir el informe, indicando cuáles lleva hechos.
- RF-39: MIENTRAS se genera, EL SISTEMA mostrará el texto a medida que llega.
- RF-40: MIENTRAS se genera, EL SISTEMA ofrecerá un botón para cancelar.
- RF-41: CUANDO termine la generación, EL SISTEMA mostrará el resultado como borrador editable.
- RF-42: CUANDO termine la generación, EL SISTEMA propondrá un título editable, formado por el nombre de la plantilla y la fecha.
- RF-43: EL SISTEMA no guardará nada en el proyecto hasta que el generador pulse «Guardar».
- RF-44: SI el generador intenta salir con un borrador sin guardar, ENTONCES EL SISTEMA le pedirá confirmación.

### Lo que la IA puede consultar
- RF-45: EL SISTEMA permitirá a la IA consultar, durante la generación, los datos de los proyectos del árbol del proyecto:
  - nombre, estado, fechas, responsable, avance y plan frente a real;
  - hitos, con su estado, fechas y avance;
  - tareas, filtrando por proyecto, hito, estado, responsable, fechas y si están atrasadas.
- RF-46: EL SISTEMA permitirá a la IA leer la descripción de una tarea, de un hito o de un proyecto del árbol.
- RF-47: EL SISTEMA limitará cada consulta a lo que puede ver la persona que genera el informe.
- RF-48: SI la IA pide datos de un proyecto fuera del árbol del proyecto, ENTONCES EL SISTEMA no se los dará y le responderá que no tiene acceso.
- RF-49: EL SISTEMA no ofrecerá a la IA ninguna operación que cree, modifique o borre datos.
- RF-50: EL SISTEMA no dará a la IA datos personales que no se muestran en el proyecto, como el correo de las personas o los atributos de usuario.
- RF-51: EL SISTEMA devolverá como máximo 200 tareas por consulta e indicará a la IA cuántas quedaron fuera.
- RF-52: SI la IA supera 60 consultas en una misma generación, ENTONCES EL SISTEMA dejará de responderlas y le pedirá que termine con lo que tiene.
- RF-53: EL SISTEMA dará al modelo, por encima del prompt de la plantilla, unas reglas fijas que el administrador no puede editar:
  - escribir solo en Markdown;
  - usar solo datos obtenidos de las consultas;
  - no inventar cifras;
  - escribir en español salvo que el prompt diga otro idioma.

### Modificar un informe
- RF-54: EL SISTEMA permitirá al generador editar a mano el contenido Markdown de un borrador o de un informe guardado.
- RF-55: EL SISTEMA permitirá al generador editar a mano el título.
- RF-56: EL SISTEMA permitirá al generador pedir un cambio a la IA escribiendo una instrucción de hasta 2 000 caracteres, por ejemplo «añade una sección de riesgos».
- RF-57: CUANDO el generador pida un cambio a la IA, EL SISTEMA le enviará el texto actual del borrador y la instrucción.
- RF-58: CUANDO el generador pida un cambio a la IA, la IA podrá hacer nuevas consultas con las mismas reglas (RF-45 a RF-53).
- RF-59: CUANDO la IA termine un cambio, EL SISTEMA sustituirá el borrador por el texto nuevo.
- RF-60: CUANDO la IA termine un cambio, EL SISTEMA permitirá deshacerlo, para volver al texto anterior mientras no se haya guardado.
- RF-61: EL SISTEMA permitirá pedir varios cambios seguidos antes de guardar.

### Guardar y borrar
- RF-62: CUANDO el generador guarde un borrador nuevo, EL SISTEMA creará el informe en el proyecto.
- RF-63: CUANDO el generador guarde un informe existente, EL SISTEMA sustituirá su título y su contenido. No se conservan versiones anteriores.
- RF-64: SI otra persona guardó el informe después de que el generador lo abriera, ENTONCES EL SISTEMA rechazará el guardado con `version_conflict`.
- RF-65: SI otra persona guardó el informe después de que el generador lo abriera, ENTONCES EL SISTEMA dejará al generador copiar su texto antes de recargar.
- RF-66: EL SISTEMA exigirá un título de 1 a 200 caracteres.
- RF-67: EL SISTEMA exigirá un contenido de 1 a 200 000 caracteres.
- RF-68: EL SISTEMA permitirá al generador borrar un informe, previa confirmación.
- RF-69: EL SISTEMA auditará la creación, modificación y borrado de cada informe, registrando el contenido solo como «contenido cambiado».
- RF-70: EL SISTEMA marcará en la auditoría si el contenido guardado salió de la IA o se editó a mano después.

### Errores del servicio de IA
- RF-71: SI el servicio de IA no responde en 30 segundos, ENTONCES EL SISTEMA cancelará la generación e informará de que el servicio no respondió.
- RF-72: SI una generación dura más de 10 minutos, ENTONCES EL SISTEMA la cancelará e informará de que superó el tiempo máximo.
- RF-73: SI el servicio de IA rechaza la petición (clave inválida, cuota agotada, modelo inexistente o texto demasiado largo), ENTONCES EL SISTEMA mostrará el motivo traducido.
- RF-74: SI una generación falla o se cancela, ENTONCES EL SISTEMA conservará el texto recibido hasta ese momento como borrador.
- RF-75: SI una petición de cambio falla o se cancela, ENTONCES EL SISTEMA conservará el texto anterior al cambio.
- RF-76: SI el servicio de IA devuelve un texto vacío, ENTONCES EL SISTEMA informará de que no se generó contenido y no creará borrador.
- RF-77: EL SISTEMA permitirá a cada persona como máximo 2 generaciones o cambios en curso a la vez.

## Requisitos no funcionales
- **Seguridad:**
  - la clave de API nunca sale del servidor, nunca se escribe en logs y nunca se devuelve en la API;
  - las peticiones al servicio de IA salen solo del servidor, nunca del navegador.
- **Privacidad:** solo se envía al servicio de IA lo que la IA consulta y la persona puede ver (RF-47 y RF-50), más el prompt y el texto del borrador.
- **Compatibilidad:** sirve cualquier servicio que implemente la API de chat de OpenAI con llamadas a herramientas, sea un proveedor externo o un modelo propio. Si el modelo no admite consultas, la prueba de conexión lo dice (RF-13).
- **Idioma:**
  - todos los textos de la interfaz salen de `es.json`;
  - los errores se muestran por su código.

## Casos límite
- **Proyecto sin hitos ni tareas:** la IA recibe listas vacías y el informe lo dice; no es un error.
- **Programa con muchos subproyectos:** la IA trabaja dentro de los límites de RF-51 y RF-52. Si se alcanzan, el informe se basa en lo leído.
- **Subproyectos que la persona no puede ver:** no existen para la IA (RF-47).
- **Proyecto archivado o cancelado:**
  - se pueden leer sus informes;
  - generar sigue el mismo permiso que gestionar ese proyecto.
- **Pérdida del permiso de gestionar el proyecto:** los informes que esa persona creó siguen en el proyecto; ya no puede modificarlos.
- **Autor dado de baja:** sus informes se conservan y muestran su nombre.
- **Configuración cambiada durante una generación:** la generación en curso termina con la configuración con la que empezó.
- **Dos generadores editan el mismo informe:** el segundo en guardar recibe `version_conflict` (RF-64).
- **Respuesta con HTML o scripts:** se muestran como texto (RF-32).
- **Respuesta que no es Markdown válido:** se guarda tal cual. Markdown es tolerante y se muestra lo que se pueda dar formato.

## Fuera de alcance
- Historial de versiones de un informe (se sobrescribe, RF-63).
- Exportar informes a PDF, Word o correo, y enviarlos o programarlos de forma periódica.
- Plantillas por proyecto: solo hay plantillas globales.
- Informes de varios proyectos sin relación, de portafolio o del mapa estratégico.
- Una entrada «Informes» en el menú principal con los informes de todos los proyectos.
- Que la IA lea comentarios, páginas, reuniones o datos de otras personas fuera del proyecto.
- Que la IA cree o modifique tareas, hitos o proyectos desde un informe.
- Gráficos generados por la IA: solo texto, tablas y listas en Markdown.
- Varios servicios de IA a la vez, o elegir modelo al generar.
- Control de gasto o cuotas de uso por persona, más allá de RF-77.
- Herramientas MCP (spec 016) para leer o generar informes.
- Repartir el análisis entre varios agentes, por ejemplo uno por subproyecto: un solo agente hace todas las consultas.
- Comentarios sobre los informes.

## Criterios de finalización
- Todos los RF con test en verde: apitest del backend con un servicio de IA simulado (incluidos el aislamiento del árbol, los límites y los errores) y Vitest en el frontend.
- Filas nuevas en el test de la matriz de permisos.
- Demo manual:
  1. configurar un servicio real en Sistema y probar la conexión;
  2. generar un informe «Estado del proyecto» de un programa con subproyectos;
  3. pedir un cambio y retocarlo a mano;
  4. guardar y comprobar que lo lee un miembro sin permiso de gestión.

## Decisiones de la revisión
- **Sección Informes:** es solo la pestaña de cada proyecto. Un menú «Informes» con los de todos los proyectos queda fuera de esta versión.
- **Agente:** por petición expresa del usuario, el agente que escribe los informes se construye con **deepagents** (LangChain), en Python. La lista de pasos de RF-38 es la que el agente mantiene mientras trabaja. Los detalles técnicos están en el plan y en el ADR 0004.

## Dudas abiertas
Ninguna.
