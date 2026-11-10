# Spec 011 — Seguimiento de proyectos

**Estado:** Aprobada (2026-11-03) («DAle»).

## Contexto y objetivo
El Resumen de la spec 008 muestra un proyecto, y el portafolio de la spec 010 muestra cifras
agregadas por programa. Falta una vista de **seguimiento** que reúna varios proyectos: sus
indicadores, el calendario de sus hitos frente a hoy y las tareas que piden atención (atrasadas,
en proceso, terminadas hace poco y próximas). Así, quien coordina varios proyectos los revisa
en una sola pantalla, sin abrirlos uno a uno. La vista es de consulta, todo se calcula en tiempo
real a la fecha de hoy y los conceptos sirven a cualquier industria.

## Usuarios / actores
- **Administrador, Dirección y líder** (cambio de la spec 023, RF-17a): abren la vista, filtran y siguen los enlaces, sobre los proyectos que ven.

## Conceptos
- **Proyectos seguidos:** los proyectos activos que cumplen los filtros, hasta un máximo de 50.
- **Mis proyectos:** los proyectos activos de los que el usuario es responsable o tiene un papel (spec 008).
- **Inicio y fin planificados:** los de la spec 009 (de la línea base si la hay).
- **Barra de un hito:** va del inicio planificado más temprano de sus tareas efectivas al fin planificado más tardío. Si una tarea no tiene inicio, cuenta su fin.
- **Fecha objetivo de un hito:** su fecha límite.
- **Tarea hoja:** una tarea sin subtareas.

## Historias de usuario
- H1: Como coordinador de varios proyectos quiero ver sus indicadores juntos para saber de un vistazo cómo va el conjunto.
- H2: Como coordinador quiero ver los hitos de todos los proyectos en un calendario frente a hoy y su fecha objetivo para anticipar retrasos.
- H3: Como coordinador quiero ver las tareas atrasadas, en proceso, terminadas hace poco y próximas para saber qué está pasando y qué viene.
- H4: Como miembro quiero que la vista empiece con mis proyectos para no tener que filtrar cada vez.

## Requisitos funcionales (criterios de aceptación en EARS)

### Acceso y filtros
- RF-1 (sustituido por la spec 023, RF-17a): EL SISTEMA ofrecerá una pantalla «Seguimiento» en la navegación principal al administrador, a Dirección y a los líderes.
- RF-2: EL SISTEMA permitirá filtrar por un programa (spec 010), con lo que se siguen sus proyectos descendientes activos y el propio programa.
- RF-3: EL SISTEMA permitirá filtrar por uno o varios proyectos, elegidos de una lista con búsqueda por nombre o identificador.
- RF-4: EL SISTEMA permitirá filtrar por uno o varios responsables de proyecto.
- RF-5: EL SISTEMA combinará los filtros, de modo que un proyecto se sigue si cumple todos los elegidos.
- RF-6 (concretado por la spec 023, RF-21): MIENTRAS no haya ningún filtro elegido, EL SISTEMA seguirá los proyectos del usuario: de los que es responsable, en los que tiene un papel, de sus equipos (no de los de encima) o en los que tiene tareas.
- RF-7 (sustituido por la spec 023, RF-22): SI no hay filtros y el usuario no tiene proyectos, ENTONCES EL SISTEMA seguirá todos los proyectos activos que ve.
- RF-8: EL SISTEMA reflejará los filtros en la dirección de la página, para que un enlace copiado abra la misma selección.
- RF-9: SI los filtros eligen más de 50 proyectos, ENTONCES EL SISTEMA seguirá los 50 primeros por nombre y avisará de cuántos han quedado fuera.
- RF-10: SI los filtros no eligen ningún proyecto, ENTONCES EL SISTEMA mostrará un mensaje que lo indique y un botón para quitar los filtros.

### Indicadores
- RF-11: EL SISTEMA mostrará el número de proyectos seguidos.
- RF-12: EL SISTEMA mostrará el número de hitos no cancelados de los proyectos seguidos, y cuántos están cumplidos.
- RF-13: EL SISTEMA mostrará el real, el plan, la brecha y el semáforo del conjunto, como media de los proyectos seguidos ponderada por su peso en el programa (spec 010).
- RF-14: EL SISTEMA mostrará la fecha de hoy como fecha de los datos.

### Calendario de hitos
- RF-15: EL SISTEMA mostrará los hitos no cancelados de los proyectos seguidos, agrupados por proyecto y ordenados por fecha objetivo dentro de cada uno.
- RF-16: EL SISTEMA dibujará cada hito como su barra sobre un eje de meses.
- RF-17: EL SISTEMA marcará en la barra de cada hito su fecha objetivo.
- RF-18: EL SISTEMA dibujará una línea vertical en la fecha de hoy.
- RF-19: EL SISTEMA mostrará en cada hito su real, su plan y su brecha (spec 009).
- RF-20: EL SISTEMA clasificará el hito como «Finalizado» si está cumplido.
- RF-21: EL SISTEMA clasificará un hito abierto como «Atrasado» si su fecha objetivo ya pasó o su semáforo es «Atrasado».
- RF-22: EL SISTEMA clasificará un hito abierto como «En riesgo» si su semáforo es «En riesgo».
- RF-23: EL SISTEMA clasificará un hito abierto como «Futuro» si el inicio de su barra es posterior a hoy.
- RF-24: EL SISTEMA clasificará un hito abierto como «Sin datos» si ninguna de sus tareas efectivas tiene fin planificado.
- RF-25: EL SISTEMA clasificará como «En ejecución» cualquier otro hito abierto.
- RF-26: EL SISTEMA aplicará las clasificaciones en el orden de RF-20 a RF-25 y se quedará con la primera que se cumpla.
- RF-27: EL SISTEMA mostrará una leyenda con cada clasificación y su color.
- RF-28: DONDE un hito no tenga barra, EL SISTEMA mostrará solo la marca de su fecha objetivo, o «Sin datos» si tampoco la tiene.
- RF-29: CUANDO el usuario pulse un hito, EL SISTEMA abrirá su ficha.

### Tareas
- RF-30: EL SISTEMA mostrará cuatro columnas de tareas hoja de los proyectos seguidos: «Atrasadas», «En proceso», «Terminadas» y «Próximas».
- RF-31: EL SISTEMA pondrá en «Atrasadas» las tareas abiertas cuya fecha fin ya pasó.
- RF-32: EL SISTEMA pondrá en «En proceso» las tareas abiertas no atrasadas que ya empezaron, es decir, con fecha de inicio igual o anterior a hoy o en un estado distinto del estado inicial.
- RF-33: EL SISTEMA pondrá en «Terminadas» las tareas cerradas en los últimos 30 días.
- RF-34: EL SISTEMA pondrá en «Próximas» las tareas abiertas que aún no empezaron y cuya fecha de inicio cae en los próximos 14 días.
- RF-35: EL SISTEMA mostrará en la cabecera de cada columna el número total de tareas que contiene.
- RF-36: EL SISTEMA mostrará en cada tarea su clave, su título, su proyecto y su tarea padre, si la tiene.
- RF-37: EL SISTEMA mostrará en «Atrasadas» el avance, la fecha fin y los días de atraso de cada tarea, de más a menos días de atraso.
- RF-38: EL SISTEMA mostrará en «En proceso» el avance y la fecha fin de cada tarea, de la fecha fin más cercana a la más lejana.
- RF-39: EL SISTEMA mostrará en «Terminadas» la fecha de cierre de cada tarea, de la más reciente a la más antigua.
- RF-40: EL SISTEMA mostrará en «Próximas» la fecha de inicio y la fecha fin de cada tarea, de la que empieza antes a la que empieza después.
- RF-41: EL SISTEMA mostrará como máximo 100 tareas por columna e indicará cuántas quedan sin mostrar.
- RF-42: CUANDO el usuario pulse una tarea, EL SISTEMA abrirá su ficha.

### Solo consulta
- RF-43: EL SISTEMA no permitirá modificar proyectos, hitos ni tareas desde esta vista.

## Requisitos no funcionales
- Tiempo real: todo se calcula en cada consulta, a la fecha de hoy.
- Rendimiento: la vista con 50 proyectos de 2 000 tareas cada uno responde en menos de 1,5 s en el servidor de previsualización.
- Idioma: todos los textos visibles en español, desde el catálogo de traducciones.
- Sin términos de un sector concreto.
- La pantalla se usa a partir de 360 px de ancho; el calendario y las columnas pueden desplazarse dentro de su sección.

## Casos límite
- Proyecto seguido sin hitos: aparece en el calendario con el texto «Sin hitos».
- Hito sin tareas y con fecha objetivo: solo la marca de la fecha, clasificado «Sin datos» (RF-24, RF-28).
- Tarea sin fechas, abierta y en el estado inicial: no entra en ninguna columna.
- Tarea abierta sin fecha fin que ya empezó: entra en «En proceso».
- Proyecto cerrado: no se sigue, aunque se elija en el filtro.
- Filtro por un programa archivado o por un proyecto que ya no existe: se ignora ese valor.
- Responsable que ya no tiene proyectos activos: el filtro no da resultados (RF-10).

## Fuera de alcance
- Modificar nada desde la vista: estados, fechas o asignados.
- Consultar a otra fecha o guardar cortes.
- Guardar la selección como vista con nombre.
- Exportar a PDF o Excel.
- Filtrar las tareas por asignado o etiqueta.
- Mapa estratégico (spec 012).

## Criterios de finalización
- Todos los RF con test en verde.
- Demo manual:
  1. Abrir «Seguimiento» y ver los proyectos propios.
  2. Filtrar por un programa y después por un responsable, copiar el enlace y abrirlo en otra pestaña.
  3. Ver el calendario con hitos en cada clasificación, la línea de hoy y la marca de la fecha objetivo.
  4. Comprobar a mano una tarea de cada columna.
  5. Pulsar un hito y una tarea y llegar a su ficha.

## Decisiones tomadas con el usuario (2026-11-03)
- **Barra del hito:** se calcula con las tareas; la fecha límite es el objetivo y se marca aparte.
- **Columnas:** «Próximas» cubre los próximos 14 días y «Terminadas» los últimos 30.
- **Nivel de las tareas:** solo las hojas, con su tarea padre como contexto.
- **Por defecto:** mis proyectos y, si no tengo ninguno, todos los activos.
- **Límite:** 50 proyectos, con aviso.
- **Acciones:** solo consulta.

## Dudas abiertas
Ninguna.
