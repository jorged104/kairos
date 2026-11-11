# Spec 024 — Comparar líneas base

**Estado:** Aprobada (2026-11-11) («Dale»).

## Contexto y objetivo
La spec 009 permite fijar la línea base de un proyecto y volver a fijarla con un motivo, y
guarda cada versión. Pero solo enseña la lista de versiones: para saber qué cambió de una a
otra hay que revisar tarea por tarea. Tampoco hay una forma rápida de ver cuánto se ha
desviado el plan actual desde la última línea base.

Esta spec permite comparar dos versiones de la línea base, o una versión con las fechas
actuales de las tareas. Muestra qué tareas entraron, cuáles salieron, cuáles se movieron y
cuántos días, y cuánto se desplaza el fin del proyecto. Además, desde ahora la línea base
conserva las tareas que se borran después, para que la comparación las muestre como quitadas.

## Usuarios / actores
- **Cualquier usuario que ve el proyecto** (spec 023): compara sus líneas base.
- **Quien gestiona el proyecto** (spec 004): fija la línea base, como hoy (spec 009).

## Conceptos
- **Lado:** cada una de las dos cosas que se comparan. Puede ser una versión de la línea base o **Hoy**: las fechas actuales de las tareas que están ahora en el proyecto.
- **Antes y después:** de los dos lados, «antes» es la versión más antigua y «después» la más reciente. Hoy siempre es «después».
- **Tipos de cambio de una tarea:**
  - **Añadida:** está en el lado «después» y no en el «antes».
  - **Quitada:** está en el lado «antes» y no en el «después». Puede ser porque se borró o porque se movió a otro proyecto.
  - **Movida:** está en los dos lados y cambia su inicio, su fin o los dos.
  - **Sin cambios:** está en los dos lados con las mismas fechas.
- **Diferencia en días:** la fecha del lado «después» menos la del lado «antes». Es positiva si la fecha se retrasa y negativa si se adelanta.
- **Fin del proyecto en un lado:** el fin más tardío de las tareas de ese lado.

## Historias de usuario
- H1: Como responsable de un proyecto quiero ver qué cambió entre dos líneas base para explicar la replanificación.
- H2: Como responsable quiero comparar la línea base vigente con las fechas de hoy para ver cuánto se ha desviado el plan antes de volver a fijarla.
- H3: Como miembro de un proyecto quiero ver qué tareas se retrasaron y cuántos días para entender por qué se movió el fin del proyecto.

## Requisitos funcionales (criterios de aceptación en EARS)

### Elegir qué se compara
- RF-1: MIENTRAS el proyecto tenga al menos una línea base, EL SISTEMA ofrecerá en el panel de línea base del Resumen (spec 009, RF-5) una acción «Comparar».
- RF-2: CUANDO el usuario abra la comparación, EL SISTEMA le dejará elegir cada lado entre las versiones de la línea base del proyecto y Hoy.
- RF-3: CUANDO el proyecto tenga dos o más versiones y el usuario abra la comparación sin elegir, EL SISTEMA comparará la versión anterior a la vigente con la vigente.
- RF-4: CUANDO el proyecto tenga una sola versión y el usuario abra la comparación sin elegir, EL SISTEMA comparará esa versión con Hoy.
- RF-5: EL SISTEMA tratará como lado «antes» la versión más antigua de las dos elegidas, sea cual sea el orden en que se eligieron.
- RF-6: SI el usuario elige lo mismo en los dos lados, ENTONCES EL SISTEMA no mostrará la comparación y le pedirá que elija dos lados distintos.
- RF-7: EL SISTEMA mostrará de cada lado su versión, su fecha, quién la fijó y su motivo, o «Hoy».
- RF-8: EL SISTEMA guardará los dos lados elegidos en la dirección de la página, de forma que al copiarla y abrirla se vea la misma comparación.

### Resumen
- RF-9: EL SISTEMA mostrará el número de tareas añadidas, quitadas, movidas y sin cambios.
- RF-10: EL SISTEMA mostrará el fin del proyecto en cada lado y la diferencia en días entre ambos.
- RF-11: SI uno de los dos lados no tiene ninguna tarea con fin, ENTONCES EL SISTEMA mostrará el fin del proyecto de ese lado como «Sin fecha» y no mostrará la diferencia.

### Tabla de tareas
- RF-12: EL SISTEMA mostrará una fila por tarea con su clave, su título, su tipo de cambio, su inicio y su fin en cada lado, y la diferencia en días del inicio y del fin.
- RF-13: EL SISTEMA dejará filtrar la tabla por uno o varios tipos de cambio.
- RF-14: CUANDO se abra la comparación, EL SISTEMA mostrará las tareas añadidas, quitadas y movidas, y ocultará las que no cambian.
- RF-15: EL SISTEMA ordenará las filas por la diferencia en días del fin, de mayor retraso a mayor adelanto, y las que no tienen diferencia al final, por clave.
- RF-16: SI una tarea no tiene inicio o fin en un lado, ENTONCES EL SISTEMA mostrará «Sin fecha» en ese lado y no mostrará la diferencia de esa fecha.
- RF-17: CUANDO una fecha pase de no existir a existir, o al revés, EL SISTEMA contará la tarea como movida.
- RF-18: EL SISTEMA enlazará cada fila con la ficha de su tarea.
- RF-19: SI la tarea se borró, ENTONCES EL SISTEMA mostrará la clave y el título guardados en la línea base, la marcará «Borrada» y no la enlazará.
- RF-20: SI la tarea se movió a otro proyecto, ENTONCES EL SISTEMA la marcará «En otro proyecto».
- RF-21: SI la tarea está ahora en un proyecto que el usuario no ve (spec 023), ENTONCES EL SISTEMA mostrará la clave y el título guardados en la línea base y no la enlazará.
- RF-22: SI la tarea está ahora en un proyecto que el usuario no ve y la línea base no guardó su clave ni su título, ENTONCES EL SISTEMA la mostrará como «Tarea de otro proyecto», sin clave, sin título y sin enlace.
- RF-23: EL SISTEMA mostrará para cada tarea que sigue existiendo y que el usuario puede ver su título actual.

### Conservar las tareas borradas
- RF-24: CUANDO se fije una línea base, EL SISTEMA guardará además la clave y el título de cada tarea.
- RF-25: CUANDO se borre una tarea, EL SISTEMA la conservará, con su clave, su título y sus fechas, en las líneas base que la guardaron conforme a RF-24.
- RF-26: EL SISTEMA no contará una tarea borrada para el plan del proyecto aunque siga en su línea base (spec 009, casos límite).

### Permisos
- RF-27: EL SISTEMA dejará comparar las líneas base de un proyecto a cualquier usuario que vea el proyecto (spec 023).
- RF-28: SI el usuario no ve el proyecto, ENTONCES EL SISTEMA responderá como si el proyecto no existiera.

## Requisitos no funcionales
- Rendimiento: la comparación de un proyecto de 2.000 tareas responde en menos de 500 ms en el servidor de previsualización.
- Comparar no cambia nada ni se audita: es solo lectura.
- Idioma: todos los textos visibles en español, desde el catálogo de traducciones.
- Sin términos de un sector concreto en textos ni en ejemplos.

## Casos límite
- **Tareas borradas antes de esta spec:** ya no están en ninguna línea base y no aparecen en la comparación. No se pueden recuperar.
- **Líneas base fijadas antes de esta spec:** no guardaron clave ni título; sus tareas muestran la clave y el título actuales (RF-23), salvo en los casos de RF-22.
- **Tarea que vuelve a su proyecto:** si una tarea se movió a otro proyecto y volvió, cuenta como si nunca se hubiera ido: está en los dos lados.
- **Tarea creada y borrada entre dos versiones:** no aparece, porque no está en ninguno de los dos lados.
- **Dos versiones con las mismas fechas:** todas las tareas salen «Sin cambios» y la tabla aparece vacía con el filtro por defecto, con un mensaje que lo dice.
- **Proyecto sin línea base:** no hay acción «Comparar» (RF-1).
- **Lado Hoy de un proyecto cerrado o archivado:** se compara igual, con las fechas actuales de sus tareas.
- **Cambio de clave del proyecto:** la clave de las tareas que siguen existiendo es la actual; las borradas muestran la guardada.

## Fuera de alcance
- Comparar peso, horas, hito, responsable, estado u otros datos de las tareas: solo fechas y presencia.
- Cronograma con las barras de los dos lados.
- Comparar la curva del plan o el plan a una fecha entre dos versiones.
- Herramienta en el MCP y en el agente de informes con IA.
- Exportar la comparación.
- Comparar líneas base de proyectos distintos o de un programa con sus proyectos.
- Recuperar las tareas borradas antes de esta spec.
- Hitos u objetivos como filas propias de la comparación.

## Criterios de finalización
- Todos los RF con test en verde.
- Demo manual:
  1. Fijar la línea base de un proyecto con tres tareas.
  2. Retrasar el fin de una tarea cinco días, crear una tarea nueva y borrar otra.
  3. Abrir «Comparar» y ver la versión 1 frente a Hoy: una añadida, una quitada «Borrada» con su título, una movida con +5 días y el fin del proyecto desplazado.
  4. Volver a fijar la línea base con un motivo y ver que la comparación por defecto es v1 frente a v2.
  5. Copiar la dirección, abrirla como otro usuario que ve el proyecto y ver la misma comparación.
  6. Abrirla como un usuario que no ve el proyecto y ver que no existe.

## Decisiones tomadas con el usuario (2026-11-11)
- **Qué se compara:** presencia de las tareas y sus fechas de inicio y fin.
- **Lados:** dos versiones cualesquiera, o una versión con Hoy.
- **Vista:** resumen y tabla filtrable por tipo de cambio, sin cronograma.
- **Tareas borradas:** las líneas base guardan desde ahora la clave y el título, y conservan las tareas borradas.
- **MCP e informes con IA:** fuera, solo en la pantalla.

## Dudas abiertas
- Ninguna.
