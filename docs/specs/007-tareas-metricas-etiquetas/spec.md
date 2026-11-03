# Spec 007 — Tareas: nombre, métricas, etiquetas e historial

**Estado:** Aprobada (2026-11-02) con los valores propuestos en las dudas («Spec aprobado dale»).

## Contexto y objetivo
«Work item» es un término técnico que el equipo no usa al hablar: dice «tarea». Además, de una
tarea solo se ve su avance y sus horas, pero no si va al ritmo de sus fechas, cuánto pesa en el
proyecto, cuánto tardó, si se pasó de fecha o cuántas veces se reabrió o estuvo bloqueada. Sin
esos datos no se puede detectar a tiempo una tarea que se atasca ni aprender de las cerradas.
Esta spec cambia el nombre visible a **tarea**, añade un bloque de **métricas** en la ficha
(avance frente al tiempo, aporte al padre y al proyecto, tiempos de ciclo, vencimiento,
reaperturas y bloqueos), **etiquetas** globales para clasificar y filtrar, y el **historial de
actividad** de cada tarea.

## Usuarios / actores
- **Cualquier usuario con acceso a la tarea:** ve su nombre nuevo, sus métricas, sus etiquetas y su historial.
- **Participante con permiso de escritura sobre la tarea:** pone y quita etiquetas.
- **Administrador del sistema:** gestiona el catálogo de etiquetas.

## Conceptos
- **Tarea:** el nombre visible de lo que hasta ahora se llamaba «work item». Los tipos (Épica, Actividad, Subactividad…) no cambian.
- **Avance esperado:** el porcentaje del plazo de la tarea (de su fecha de inicio a su fecha fin) que ya ha pasado.
- **Desvío de avance:** el avance de la tarea (el derivado si tiene hijas) menos su avance esperado, en puntos.
- **Peso del subárbol:** el peso de la tarea más el de todas sus descendientes.
- **Aporte:** la parte del avance de su padre o del proyecto que depende de la tarea y sus descendientes.
- **Lead time:** tiempo desde que se creó la tarea hasta su último cierre.
- **Cycle time:** tiempo desde que la tarea salió por primera vez de su estado inicial hasta su último cierre.
- **Bloqueada:** una tarea de la que otra tarea abierta es bloqueante (dependencia `blocks` de la spec 004).
- **Etiqueta:** una marca del catálogo con nombre y color que clasifica tareas, aparte de su tipo.

## Historias de usuario
- H1: Como miembro quiero ver «tarea» en lugar de «work item» para entender la aplicación sin jerga.
- H2: Como responsable quiero ver si una tarea va adelantada o retrasada respecto a sus fechas para actuar antes de que venza.
- H3: Como responsable quiero saber cuánto pesa una tarea en su padre y en el proyecto para decidir dónde poner el esfuerzo.
- H4: Como miembro quiero ver cuánto tardó una tarea, si se cerró a tiempo y si se reabrió o se bloqueó, para mejorar las estimaciones.
- H5: Como administrador quiero definir etiquetas comunes para que todos los proyectos clasifiquen igual.
- H6: Como miembro quiero etiquetar tareas y filtrar el árbol por etiqueta para encontrar las de un tema.
- H7: Como miembro quiero ver quién cambió qué en una tarea y cuándo para entender cómo llegó a su estado actual.

## Requisitos funcionales (criterios de aceptación en EARS)

### Nombre visible
- RF-1: EL SISTEMA mostrará «tarea» (y «tareas» en plural) en todos los textos visibles donde hoy aparece «work item», incluidos títulos, botones, columnas, mensajes vacíos y errores.
- RF-2: EL SISTEMA concordará en femenino los textos que acompañan a «tarea» (por ejemplo, «Nueva tarea», «tarea eliminada»).
- RF-3: EL SISTEMA mostrará el bloque del editor que hoy se llama «Lista de tareas» como «Lista de comprobación», para no confundirlo con las tareas.
- RF-4: EL SISTEMA mantendrá los nombres de los tipos del catálogo sin cambios.

### Métricas: bloque en la ficha
- RF-5: EL SISTEMA mostrará en la ficha de la tarea una sección «Métricas» entre las propiedades y los comentarios.
- RF-6: EL SISTEMA mostrará la sección «Métricas» plegada al abrir la ficha.
- RF-7: CUANDO el usuario despliegue o pliegue la sección «Métricas», EL SISTEMA recordará esa elección en su navegador para las siguientes fichas.
- RF-8: CUANDO cambie una propiedad de la tarea desde la ficha, EL SISTEMA mostrará las métricas actualizadas sin recargar la página.

### Métricas: avance frente al tiempo
- RF-9: MIENTRAS una tarea abierta tenga fecha de inicio y fecha fin, EL SISTEMA mostrará su avance, su avance esperado y su desvío de avance.
- RF-10 (sustituido por la spec 009, RF-43): EL SISTEMA calculará el avance esperado como los días naturales transcurridos desde la fecha de inicio entre los días naturales del plazo (ambas fechas incluidas), en porcentaje entero.
- RF-11 (sustituido por la spec 009, RF-43): MIENTRAS la fecha actual sea anterior a la fecha de inicio, EL SISTEMA mostrará un avance esperado de 0 %.
- RF-12 (sustituido por la spec 009, RF-43): MIENTRAS la fecha actual sea posterior a la fecha fin, EL SISTEMA mostrará un avance esperado de 100 %.
- RF-13 (sustituido por la spec 009, RF-43): EL SISTEMA clasificará la tarea como «adelantada» si su desvío es mayor que +10 puntos, «retrasada» si es menor que −10 puntos y «al día» en otro caso.
- RF-14 (sustituido por la spec 009, RF-43): SI la tarea no tiene fecha de inicio o fecha fin, ENTONCES EL SISTEMA mostrará «Sin plazo» en lugar del avance esperado, el desvío y la clasificación.
- RF-15: MIENTRAS la tarea esté cerrada, EL SISTEMA no mostrará el avance esperado, el desvío ni la clasificación.

### Métricas: aporte al padre y al proyecto
- RF-16: DONDE la tarea tenga padre, EL SISTEMA mostrará el peso de su subárbol como porcentaje del peso del subárbol del padre.
- RF-17: EL SISTEMA mostrará el peso del subárbol de la tarea como porcentaje de la suma de los pesos de todas las tareas del proyecto.
- RF-18: EL SISTEMA mostrará, junto a cada porcentaje de peso, los puntos de avance que la tarea y sus descendientes aportan ya a su padre y al proyecto (su porcentaje de peso por su avance derivado).
- RF-19: SI el peso del subárbol de la tarea es 0, ENTONCES EL SISTEMA mostrará un aporte de 0 % con la indicación «seguimiento».
- RF-20: SI la suma de pesos del padre o del proyecto es 0, ENTONCES EL SISTEMA mostrará «Sin peso» en lugar de ese aporte.

### Métricas: tiempos de ciclo
- RF-21: MIENTRAS la tarea esté cerrada, EL SISTEMA mostrará su lead time y su cycle time en días con un decimal.
- RF-22: MIENTRAS la tarea esté abierta, EL SISTEMA mostrará los días transcurridos desde su creación y, si ya salió de su estado inicial, los días transcurridos desde esa primera salida.
- RF-23: EL SISTEMA mostrará el tiempo que la tarea lleva en su estado actual, en días con un decimal.
- RF-24: SI una tarea se cerró sin pasar por ningún estado abierto distinto del inicial, ENTONCES EL SISTEMA mostrará su cycle time como «—».
- RF-25: CUANDO se reabra una tarea y se vuelva a cerrar, EL SISTEMA calculará el lead time y el cycle time hasta el último cierre.

### Métricas: vencimiento
- RF-26: MIENTRAS una tarea abierta tenga fecha fin igual o posterior a hoy, EL SISTEMA mostrará los días naturales que faltan para ella.
- RF-27: MIENTRAS una tarea abierta tenga fecha fin anterior a hoy, EL SISTEMA mostrará «Vencida» y los días naturales de retraso.
- RF-28: MIENTRAS una tarea cerrada tenga fecha fin, EL SISTEMA mostrará «A tiempo» si su último cierre fue en la fecha fin o antes, o los días naturales de retraso en otro caso.
- RF-29: SI la tarea no tiene fecha fin, ENTONCES EL SISTEMA mostrará «Sin fecha fin» en el vencimiento.

### Métricas: reaperturas y bloqueos
- RF-30: EL SISTEMA mostrará cuántas veces se ha reabierto la tarea (pasos de un estado cerrado a uno abierto).
- RF-31: MIENTRAS la tarea esté bloqueada, EL SISTEMA mostrará «Bloqueada» y las tareas abiertas que la bloquean.
- RF-32: SI el usuario no tiene acceso a una tarea que bloquea, ENTONCES EL SISTEMA la contará como bloqueante sin mostrar su título ni su identificador.
- RF-33: EL SISTEMA mostrará el tiempo total, en días con un decimal, que la tarea ha estado bloqueada desde su creación.

### Métricas: datos anteriores
- RF-34: EL SISTEMA calculará las métricas de las tareas existentes antes de esta spec a partir de su historial de cambios.
- RF-35: SI falta en el historial un dato que una métrica necesita, ENTONCES EL SISTEMA mostrará «Sin datos» en esa métrica en lugar de un valor aproximado.

### Etiquetas: catálogo
- RF-36: EL SISTEMA permitirá a un administrador crear, renombrar, cambiar el color, reordenar y retirar etiquetas en el catálogo.
- RF-37: SI el nombre de una etiqueta coincide con el de otra activa (ignorando mayúsculas y espacios exteriores), ENTONCES EL SISTEMA rechazará el cambio con un código de conflicto.
- RF-38: EL SISTEMA compartirá las etiquetas del catálogo entre todos los proyectos.
- RF-39: SI una etiqueta está retirada, ENTONCES EL SISTEMA no la ofrecerá para añadirla y la seguirá mostrando en las tareas que ya la tienen.

### Etiquetas: en las tareas
- RF-40: EL SISTEMA permitirá a quien puede modificar una tarea añadirle y quitarle etiquetas del catálogo.
- RF-41: SI se intenta poner a una tarea más de 10 etiquetas, ENTONCES EL SISTEMA rechazará el cambio.
- RF-42: EL SISTEMA mostrará las etiquetas de la tarea entre las propiedades principales de la ficha.
- RF-43: EL SISTEMA mostrará en el árbol de tareas una columna de etiquetas, editable con las mismas reglas que la ficha.
- RF-44: CUANDO el usuario filtre el árbol por una o varias etiquetas, EL SISTEMA mostrará las tareas que tengan al menos una de ellas.
- RF-45: CUANDO se copie una tarea desde una plantilla, EL SISTEMA copiará sus etiquetas activas.
- RF-46: EL SISTEMA registrará en la auditoría cada etiqueta añadida o quitada de una tarea.

### Historial de actividad
- RF-47: EL SISTEMA mostrará en la ficha de la tarea una sección «Historial» después de los comentarios, plegada al abrir la ficha.
- RF-48: EL SISTEMA mostrará en el historial cada cambio de la tarea con su autor, su fecha y hora, la propiedad cambiada y los valores anterior y nuevo.
- RF-49: EL SISTEMA mostrará los valores del historial con su nombre visible (estado, persona, etiqueta, talla…) y no con su identificador interno.
- RF-50: CUANDO cambie la descripción de la tarea, EL SISTEMA mostrará en el historial solo «editó la descripción», sin el contenido.
- RF-51: EL SISTEMA agrupará en una sola entrada del historial los cambios de la descripción de una misma persona separados por menos de 10 minutos.
- RF-52: EL SISTEMA ordenará el historial del cambio más reciente al más antiguo y mostrará 50 entradas cada vez, con la opción de cargar más.
- RF-53: EL SISTEMA mostrará el historial a cualquier usuario con acceso a la tarea.
- RF-54: SI el autor de un cambio está desactivado o dado de baja, ENTONCES EL SISTEMA mostrará su nombre igualmente.

## Requisitos no funcionales
- **Rendimiento** (entorno local): las métricas de una tarea con 200 cambios en su historial se muestran en < 300 ms. La primera página del historial, en < 300 ms. El árbol de 2 000 tareas con la columna de etiquetas sigue dentro del límite de la spec 004.
- **Compatibilidad:** el cambio de nombre es solo de textos visibles. La API, las direcciones de las páginas y los códigos de error no cambian, y la versión anterior de la API sigue funcionando durante el despliegue.
- **Seguridad:** las métricas, las etiquetas y el historial respetan los permisos de acceso a la tarea y comprueban permisos, nunca nombres de rol.
- **Idioma:** español por defecto; todos los textos nuevos salen del catálogo de i18n.

## Casos límite
- **Fecha de inicio y fin iguales:** el plazo es de un día; el avance esperado es 0 % antes, 100 % ese día y después.
- **Fecha fin anterior a la de inicio:** la spec 004 ya lo impide; si existiera, se trata como «Sin plazo».
- **Tarea con hijas:** el desvío usa el avance derivado y el aporte usa el peso de todo su subárbol.
- **Tarea de peso 0 con hijas de peso mayor:** su subárbol pesa lo de las hijas; el aporte no es 0.
- **Tarea creada directamente en un estado cerrado:** lead time 0, cycle time «—» (RF-24).
- **Bloqueante borrado o cerrado:** deja de bloquear desde ese momento; el tiempo bloqueado acumulado se conserva.
- **Dependencia `blocks` entre proyectos:** cuenta igual, con la restricción de visibilidad de RF-32.
- **Tarea movida a otro padre:** el aporte al padre se calcula con el padre actual.
- **Etiqueta retirada en una plantilla:** no se copia (RF-45).
- **Dos personas cambian las etiquetas a la vez:** el segundo cambio responde `version_conflict`, como el resto de propiedades.

## Fuera de alcance
- Métricas en el árbol de tareas, en los hitos o en un panel del proyecto, y filtrar u ordenar por métricas.
- Gráficos (burndown, burnup, distribución de tiempos) y exportación de métricas.
- Comparar horas estimadas con las consumidas y registrar horas trabajadas.
- Checklists dentro de la tarea como propiedad (el bloque «Lista de comprobación» de la página ya existe).
- Etiquetas por proyecto o creadas por cualquier usuario.
- Avisos o notificaciones por retraso, vencimiento o bloqueo.
- Cambiar «work item» en la API, en las direcciones de las páginas, en el código o en la documentación técnica.
- Renombrar los tipos del catálogo.
- Deshacer cambios desde el historial.

## Criterios de finalización
- Todos los RF con tests en verde: API (métricas con fechas, pesos, reaperturas y bloqueos; etiquetas con permisos en el test de matriz; historial) y frontend (textos, sección de métricas, etiquetas en ficha y árbol, historial).
- Ningún texto visible contiene «work item» (comprobación automática sobre el catálogo de i18n).
- Pruebas de rendimiento dentro de los umbrales.
- Demo manual:
  1. Recorrer proyecto, árbol y ficha y no ver «work item» en ninguna parte.
  2. Poner a una tarea fechas del 1 al 10 de un mes, avance 20 % el día 6, y ver «retrasada».
  3. Ver el aporte de una subtarea a su padre y al proyecto, y ponerle peso 0 para ver «seguimiento».
  4. Cerrar, reabrir y volver a cerrar una tarea, y ver lead time, cycle time, una reapertura y si se cerró a tiempo.
  5. Bloquear una tarea con otra, ver «Bloqueada», cerrar la bloqueante y ver el tiempo bloqueado.
  6. Crear etiquetas como administrador, ponerlas en tareas desde la ficha y el árbol y filtrar por ellas.
  7. Abrir el historial y ver los cambios anteriores con nombres legibles.

## Decisiones tomadas con el usuario (2026-11-02)
- **Nombre:** «tarea», solo en los textos visibles (RF-1 a RF-4).
- **Avance relativo:** las dos lecturas, frente al tiempo (RF-9 a RF-15) y aporte al padre y al proyecto (RF-16 a RF-20).
- **Métricas:** tiempos de ciclo, vencimiento y retraso, reaperturas y bloqueos. Horas estimadas frente a reales, fuera.
- **Dónde:** solo en la ficha de la tarea.
- **Extras:** etiquetas e historial de actividad. Checklist y registro de horas, fuera.
- **Etiquetas:** globales, gestionadas por el administrador (RF-36 a RF-39).

- **Dudas cerradas con lo propuesto:** margen de ±10 puntos (RF-13), cycle time desde la primera salida del estado inicial (RF-24), máximo de 10 etiquetas (RF-41) y orden propiedades → métricas → comentarios → página → historial (RF-5, RF-47).

## Dudas abiertas
Ninguna.
