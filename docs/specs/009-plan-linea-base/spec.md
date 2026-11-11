# Spec 009 — Plan, línea base y semáforo

**Estado:** Aprobada (2026-11-03) («dale»).

## Contexto y objetivo
Hoy el avance esperado de un proyecto o de una tarea es lineal: el porcentaje de días que han
pasado entre su inicio y su fin (spec 007, RF-10; spec 008, RF-6 y RF-13). Ese cálculo no tiene
en cuenta cómo está repartido el trabajo: un proyecto que concentra casi todo su esfuerzo al
final aparece «retrasado» desde el principio. Además, si alguien mueve las fechas de las tareas,
el plan se mueve con ellas y el retraso desaparece sin que nadie lo vea.

Esta spec introduce un **plan** que sale de las fechas y los pesos de las tareas, una **línea
base** que congela esas fechas para medir contra lo comprometido, y un **semáforo** común de
cuatro niveles con umbrales configurables. Todo se calcula en tiempo real, a la fecha de hoy.
Es la base de las siguientes fases de la hoja de ruta: portafolio de programas, seguimiento de
varios proyectos y mapa estratégico. Los conceptos sirven a cualquier industria.

## Usuarios / actores
- **Quien gestiona el proyecto** (spec 004): fija la línea base y la vuelve a fijar.
- **Cualquier usuario con acceso al proyecto:** ve el plan, el real, la brecha y el semáforo del proyecto, sus hitos y sus tareas.
- **Administrador del sistema:** configura los umbrales, los nombres y los colores del semáforo.

## Conceptos
- **Fechas planificadas de una tarea:** su inicio y su fin según la línea base vigente; si la tarea no está en la línea base o el proyecto no tiene línea base, sus fechas actuales.
- **Línea base:** la foto de las fechas de inicio y fin de todas las tareas de un proyecto en un momento dado, fijada por quien gestiona el proyecto.
- **Fracción planificada de una tarea a una fecha:** la parte de la tarea que debería estar hecha a esa fecha según sus fechas planificadas, de 0 a 1.
- **Plan:** el porcentaje de trabajo que debería estar hecho hoy, ponderado por el peso de las tareas (spec 004).
- **Real:** el avance ponderado que el sistema ya calcula para proyectos, hitos y tareas (specs 004 y 007).
- **Brecha:** real menos plan, en puntos porcentuales (pp).
- **Semáforo:** la clasificación de la brecha en cuatro niveles: Adelantado, En tiempo, En riesgo y Atrasado.

## Historias de usuario
- H1: Como responsable de un proyecto quiero que el plan refleje cómo está repartido el trabajo en el tiempo para no ver alarmas falsas.
- H2: Como responsable quiero congelar las fechas comprometidas para que replanificar no esconda el retraso.
- H3: Como responsable quiero volver a fijar la línea base cuando el plan cambia de verdad, dejando constancia del motivo.
- H4: Como miembro quiero ver de un vistazo, con el mismo color en todas partes, si un proyecto, un hito o una tarea va adelantado, en tiempo, en riesgo o atrasado.
- H5: Como administrador quiero ajustar los umbrales y los nombres del semáforo a la forma de trabajar de mi organización.

## Requisitos funcionales (criterios de aceptación en EARS)

### Línea base
- RF-1: CUANDO quien gestiona el proyecto fije la línea base, EL SISTEMA guardará las fechas de inicio y fin vigentes de todas las tareas del proyecto.
- RF-2: CUANDO quien gestiona el proyecto vuelva a fijar la línea base, EL SISTEMA le exigirá un motivo de entre 1 y 500 caracteres.
- RF-3: CUANDO se fije una línea base, EL SISTEMA la numerará con el siguiente número de versión del proyecto, empezando por 1.
- RF-4: EL SISTEMA medirá el plan contra la versión más reciente de la línea base.
- RF-5: EL SISTEMA mostrará en el proyecto la versión vigente de la línea base, su fecha, quién la fijó y su motivo.
- RF-6: EL SISTEMA mostrará la lista de versiones anteriores de la línea base con su fecha, su autor y su motivo.
- RF-7: SI quien intenta fijar la línea base no gestiona el proyecto, ENTONCES EL SISTEMA lo rechazará con un error de permisos.
- RF-8: SI el proyecto está cerrado o archivado, ENTONCES EL SISTEMA no permitirá fijar la línea base.
- RF-9: MIENTRAS el proyecto no tenga línea base, EL SISTEMA calculará el plan con las fechas actuales de las tareas.
- RF-10: MIENTRAS una tarea no esté en la línea base vigente, EL SISTEMA calculará su parte del plan con sus fechas actuales.
- RF-11: MIENTRAS una tarea no esté en la línea base vigente, EL SISTEMA la marcará como «Fuera de línea base».
- RF-12: MIENTRAS las fechas actuales de una tarea difieran de las de la línea base vigente, EL SISTEMA mostrará en su ficha ambas fechas y la diferencia en días.
- RF-13: EL SISTEMA dibujará en el cronograma (spec 008) la barra de la línea base de cada tarea detrás de su barra actual.

### Plan de una tarea
- RF-14: MIENTRAS la fecha de hoy sea anterior al inicio planificado de la tarea, EL SISTEMA considerará su fracción planificada igual a 0.
- RF-15: MIENTRAS la fecha de hoy sea igual o posterior al fin planificado de la tarea, EL SISTEMA considerará su fracción planificada igual a 1.
- RF-16: MIENTRAS la fecha de hoy esté entre el inicio y el fin planificados, EL SISTEMA calculará la fracción planificada como los días naturales transcurridos desde el inicio entre los días naturales del plazo, ambas fechas incluidas.
- RF-17: DONDE la tarea solo tenga fin planificado, EL SISTEMA considerará su fracción planificada 0 antes del fin y 1 desde el día del fin.
- RF-18: SI la tarea no tiene fin planificado, ENTONCES EL SISTEMA la dejará fuera del plan.
- RF-19: DONDE la tarea tenga subtareas, EL SISTEMA calculará su plan como el de sus subtareas ponderado por peso, igual que su avance derivado (spec 004).

### Plan de un proyecto y de un hito
- RF-20: EL SISTEMA calculará el plan del proyecto como la suma del peso por la fracción planificada de sus tareas dividida entre la suma de los pesos de las tareas que cuentan para el plan, en porcentaje con un decimal.
- RF-21: EL SISTEMA calculará el plan de un hito igual que el del proyecto, sobre sus tareas efectivas (spec 004).
- RF-22: EL SISTEMA no mostrará nunca un plan mayor que 100 % ni menor que 0 %.
- RF-23: SI ninguna tarea del proyecto cuenta para el plan, ENTONCES EL SISTEMA calculará el plan del proyecto con el cálculo lineal entre sus fechas de inicio y fin (spec 008, RF-6).
- RF-24: SI ninguna tarea del proyecto cuenta para el plan y el proyecto no tiene fechas de inicio y fin, ENTONCES EL SISTEMA mostrará «Sin plazo» en lugar del plan, la brecha y el semáforo.
- RF-25: SI ninguna tarea de un hito cuenta para el plan, ENTONCES EL SISTEMA mostrará «Sin plazo» en lugar del plan, la brecha y el semáforo del hito.
- RF-26: EL SISTEMA contará una tarea en cualquier estado cerrado con un real del 100 % y con su plan según sus fechas, igual que hoy (las tareas no tienen estado «cancelada»).

### Brecha y semáforo
- RF-27: EL SISTEMA calculará la brecha como el real menos el plan, en puntos porcentuales con un decimal.
- RF-28: EL SISTEMA clasificará la brecha como «Adelantado» si es mayor o igual que el umbral superior.
- RF-29: EL SISTEMA clasificará la brecha como «En tiempo» si es menor que el umbral superior y mayor o igual que el umbral de riesgo.
- RF-30: EL SISTEMA clasificará la brecha como «En riesgo» si es menor que el umbral de riesgo y mayor o igual que el umbral de atraso.
- RF-31: EL SISTEMA clasificará la brecha como «Atrasado» si es menor que el umbral de atraso.
- RF-32: EL SISTEMA usará como valores por defecto +5 pp para el umbral superior, −5 pp para el de riesgo y −15 pp para el de atraso.
- RF-33: EL SISTEMA aplicará el mismo semáforo y los mismos umbrales a proyectos, hitos y tareas.
- RF-34: MIENTRAS el proyecto, el hito o la tarea esté cerrado o cumplido, EL SISTEMA no mostrará plan, brecha ni semáforo.
- RF-35: EL SISTEMA mostrará junto a cada semáforo el plan, el real y la brecha.

### Configuración del semáforo
- RF-36: EL SISTEMA permitirá al administrador del sistema cambiar los tres umbrales.
- RF-37: SI los umbrales no cumplen superior > riesgo > atraso, ENTONCES EL SISTEMA rechazará el cambio e indicará el umbral que falla.
- RF-38: SI algún umbral queda fuera del rango de −100 a +100 pp, ENTONCES EL SISTEMA rechazará el cambio.
- RF-39: EL SISTEMA permitirá al administrador cambiar el nombre visible de cada nivel, de 1 a 30 caracteres.
- RF-40: EL SISTEMA permitirá al administrador elegir el color de cada nivel de una paleta cerrada que cumpla el contraste de accesibilidad en tema claro y oscuro.
- RF-41: CUANDO el administrador cambie la configuración del semáforo, EL SISTEMA aplicará los nuevos valores en todas las pantallas a partir de la siguiente consulta.
- RF-42: SI quien intenta cambiar la configuración no es administrador del sistema, ENTONCES EL SISTEMA lo rechazará con un error de permisos.

### Sustitución del esperado lineal (specs 007 y 008)
- RF-43: EL SISTEMA mostrará en la ficha de la tarea su plan, su brecha y su semáforo en lugar del avance esperado, el desvío y la clasificación de la spec 007 (RF-9 a RF-14).
- RF-44: EL SISTEMA mostrará en el Resumen del proyecto su plan, su brecha y su semáforo en lugar del avance esperado y la diferencia de la spec 008 (RF-6 y RF-7).
- RF-45: EL SISTEMA mostrará en el Resumen el plan y la brecha de cada hito de la lista de hitos (spec 008, RF-11).
- RF-46: EL SISTEMA dibujará la línea de plan de la gráfica del Resumen (spec 008, RF-13) con el plan de cada día calculado con la línea base vigente.
- RF-47: EL SISTEMA considerará «retrasada», a efectos de las tareas en riesgo del Resumen (spec 008, RF-15), una tarea con semáforo «En riesgo» o «Atrasado».
- RF-48: EL SISTEMA marcará en el cronograma (spec 008, RF-71) cada barra de tarea abierta con el color de su semáforo cuando sea «En riesgo» o «Atrasado».

### Auditoría
- RF-49: EL SISTEMA auditará cada vez que se fije una línea base, con su versión y su motivo.
- RF-50: EL SISTEMA auditará cada cambio de la configuración del semáforo con los valores anteriores y nuevos.

## Requisitos no funcionales
- Tiempo real: plan, real, brecha y semáforo se calculan en cada consulta, a la fecha de hoy; no se guardan fotos.
- Rendimiento: el Resumen de un proyecto de 2.000 tareas responde en menos de 500 ms en el servidor de previsualización.
- Idioma: todos los textos visibles en español, desde el catálogo de traducciones.
- Sin términos de un sector concreto en textos, valores por defecto ni ejemplos.

## Casos límite
- Tarea con fin anterior al inicio: no puede existir (spec 004); la línea base nunca la contiene.
- Tarea de peso 0: no aporta al plan ni al real, igual que hoy.
- Tarea que estaba en la línea base y se borra: deja de contar para el plan y para el real. Desde la spec 024 sigue guardada en la línea base para mostrarla como quitada al comparar.
- Tarea rechazada o cerrada sin hacer: cuenta como cerrada (RF-26); quien no quiera que cuente la borra o le pone peso 0.
- Tarea que cambia de proyecto: deja de contar en la línea base del proyecto de origen y entra en el nuevo como fuera de línea base.
- Fijar la línea base dos veces el mismo día: crea dos versiones; manda la más reciente.
- Proyecto sin tareas y sin fechas: «Sin plazo» (RF-24).
- Plan del 100 % y real del 100 %: brecha 0, «En tiempo» hasta que se cierre el proyecto.
- Cambio de umbrales mientras alguien mira una pantalla: verá los nuevos al recargar o en la siguiente consulta (RF-41).

## Fuera de alcance
- Cortes guardados o fotos congeladas del plan, del real o de la brecha.
- Consultar el plan o la brecha a una fecha distinta de hoy.
- Umbrales distintos por proyecto o por programa.
- Línea base de hitos u objetivos independiente de la de sus tareas.
- Aprobación de una nueva línea base por un tercero.
- Comparar dos versiones de línea base entre sí (lo cubre la spec 024).
- Programas, portafolio, seguimiento de varios proyectos y mapa estratégico (siguientes specs).

## Criterios de finalización
- Todos los RF con test en verde, incluidos el test de matriz de permisos para fijar la línea base y para configurar el semáforo.
- Demo manual:
  1. Crear un proyecto con tres tareas de pesos y fechas distintas y comprobar el plan de hoy con un cálculo a mano.
  2. Fijar la línea base, mover el fin de una tarea y ver que el plan no cambia, que la tarea muestra la diferencia en días y que el cronograma dibuja la barra de la línea base.
  3. Crear una tarea nueva y verla «Fuera de línea base».
  4. Volver a fijar la línea base con un motivo y ver las dos versiones.
  5. Cambiar los umbrales como administrador y ver que cambian los colores del Resumen, los hitos y las fichas.
  6. Intentar fijar la línea base como miembro y ver el error de permisos.

## Decisiones tomadas con el usuario (2026-11-03)
- **Plan:** a partir de las fechas y los pesos de las tareas, con línea base congelada.
- **Tareas:** usan el mismo semáforo de cuatro niveles; desaparece el margen fijo de ±10 puntos.
- **Tareas nuevas tras fijar la línea base:** cuentan con sus fechas actuales y se marcan «Fuera de línea base».
- **Permiso:** fija la línea base quien gestiona el proyecto.
- **Tiempo real:** sin cortes guardados; todo se calcula a la fecha de hoy.
- **Umbrales:** globales y configurables; por defecto +5, −5 y −15 pp.
- **Tareas en riesgo:** cuentan las de nivel «En riesgo» y «Atrasado», cada una con su color (RF-47, RF-48).

## Dudas abiertas
Ninguna.
