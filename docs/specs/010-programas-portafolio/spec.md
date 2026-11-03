# Spec 010 — Programas y portafolio

**Estado:** Aprobada (2026-11-03) («Si»).

## Contexto y objetivo
Muchas organizaciones agrupan varios proyectos bajo una misma iniciativa y necesitan saber, de
un vistazo, si cada grupo va según lo planificado y qué proyectos lo frenan. Kairos ya permite
anidar proyectos, y desde la spec 009 calcula el plan, el real, la brecha y el semáforo de cada
proyecto. Lo que falta es tratar un proyecto padre como un **programa** con un avance agregado,
y una pantalla de **portafolio** que muestre todos los programas y sus proyectos. Todo se
calcula en tiempo real, a la fecha de hoy. Los conceptos sirven a cualquier industria.

## Usuarios / actores
- **Administrador del sistema:** marca o desmarca un proyecto como programa y fija el peso de los proyectos.
- **Cualquier usuario con lectura de trabajo:** consulta el portafolio. Hoy esa lectura da acceso a todos los proyectos que no son plantillas (spec 004), así que todos ven el portafolio completo.

## Conceptos
- **Programa:** un proyecto marcado como tal. Agrupa a todos sus descendientes. Un programa puede estar dentro de otro programa.
- **Miembros de un programa:** el propio programa, por sus tareas propias, y todos sus proyectos descendientes, a cualquier nivel, que no estén archivados ni sean plantillas.
- **Esfuerzo de un proyecto:** la suma de los pesos de sus tareas propias (spec 004).
- **Peso en el programa:** el peso fijado a mano para un proyecto; si no tiene, su esfuerzo.
- **Plan, real, brecha y semáforo de un proyecto:** los de la spec 009.
- **Bajo plan:** un proyecto cuyo semáforo es «En riesgo» o «Atrasado».

## Historias de usuario
- H1: Como responsable de varios proyectos quiero ver el avance agregado de un programa para saber si la iniciativa va según lo planificado.
- H2: Como responsable quiero ver qué proyectos de un programa están bajo plan para actuar primero en ellos.
- H3: Como administrador quiero marcar qué proyectos son programas y cuánto pesa cada proyecto para que el agregado refleje la importancia real.
- H4: Como cualquier miembro quiero filtrar el portafolio por semáforo y ordenar los proyectos para encontrar rápido los que necesitan atención.
- H5: Como cualquier miembro quiero ver también los proyectos que no pertenecen a ningún programa para no perder ninguno de vista.

## Requisitos funcionales (criterios de aceptación en EARS)

### Marcar programas y pesos
- RF-1: EL SISTEMA permitirá al administrador del sistema marcar un proyecto como programa.
- RF-2: EL SISTEMA permitirá al administrador del sistema desmarcar un programa.
- RF-3: SI quien intenta marcar o desmarcar un programa no es administrador del sistema, ENTONCES EL SISTEMA lo rechazará con un error de permisos.
- RF-4: SI el proyecto es una plantilla, ENTONCES EL SISTEMA no permitirá marcarlo como programa.
- RF-5: EL SISTEMA permitirá al administrador del sistema fijar el peso de un proyecto, un entero de 0 a 100 000.
- RF-6: EL SISTEMA permitirá al administrador del sistema quitar el peso fijado, con lo que el proyecto vuelve a pesar su esfuerzo.
- RF-7: SI quien intenta fijar o quitar un peso no es administrador del sistema, ENTONCES EL SISTEMA lo rechazará con un error de permisos.
- RF-8: EL SISTEMA mostrará en los datos del proyecto si es un programa y su peso, indicando si es el fijado o su esfuerzo.
- RF-9: EL SISTEMA auditará cada vez que se marque o desmarque un programa.
- RF-10: EL SISTEMA auditará cada cambio de peso con el valor anterior y el nuevo.

### Avance de un programa
- RF-11: EL SISTEMA calculará el real de un programa como la media de los reales de sus miembros ponderada por su peso en el programa, en porcentaje con un decimal.
- RF-12: EL SISTEMA calculará el plan de un programa como la media de los planes de sus miembros con plan, ponderada por su peso en el programa.
- RF-13: EL SISTEMA contará cada proyecto descendiente una sola vez en el avance de cada programa que lo contenga, aunque haya programas anidados entre medias.
- RF-14: EL SISTEMA contará como miembro las tareas propias del programa, con su peso en el programa (su esfuerzo si no tiene uno fijado), igual que cualquier otro proyecto.
- RF-15: EL SISTEMA contará un proyecto cerrado con su plan y su real a la fecha de hoy.
- RF-16: EL SISTEMA no contará los proyectos archivados ni las plantillas.
- RF-17: SI todos los miembros pesan 0, ENTONCES EL SISTEMA calculará el plan y el real como la media simple de sus miembros.
- RF-18: SI ningún miembro tiene plan, ENTONCES EL SISTEMA mostrará «Sin plazo» en lugar del plan, la brecha y el semáforo del programa.
- RF-19: EL SISTEMA calculará la brecha y el semáforo del programa con las reglas y los umbrales de la spec 009.
- RF-20: MIENTRAS el programa esté cerrado, EL SISTEMA no mostrará su plan, su brecha ni su semáforo (spec 009, RF-34).

### Pantalla Portafolio
- RF-21: EL SISTEMA ofrecerá una pantalla «Portafolio» en la navegación principal a todo usuario con lectura de trabajo.
- RF-22: EL SISTEMA mostrará en el portafolio una tarjeta por cada programa activo o cerrado, ordenadas por nombre.
- RF-23: EL SISTEMA mostrará en cada tarjeta el nombre del programa, su real, su plan, su brecha y su semáforo.
- RF-24: EL SISTEMA mostrará en cada tarjeta el número de proyectos descendientes y cuántos están bajo plan.
- RF-25: EL SISTEMA mostrará en cada tarjeta una tabla con una fila por cada hijo directo del programa que no esté archivado ni sea plantilla.
- RF-26: EL SISTEMA mostrará en cada fila el nombre, una barra del real con la marca del plan, el plan, el real, la brecha y el semáforo del propio proyecto, sin sus subproyectos.
- RF-27: DONDE una fila sea un programa anidado, EL SISTEMA mostrará en ella su avance agregado y lo indicará como programa.
- RF-28: EL SISTEMA mostrará una tarjeta «Sin programa» con los proyectos activos o cerrados que no son programas ni tienen ningún programa entre sus ancestros.
- RF-29: EL SISTEMA mostrará la fecha de hoy como fecha de los datos del portafolio.
- RF-30: CUANDO el usuario pulse una fila, EL SISTEMA abrirá el Resumen de ese proyecto.
- RF-31: CUANDO el usuario pulse el nombre de un programa, EL SISTEMA abrirá el Resumen del programa.

### Filtrar y ordenar
- RF-32: EL SISTEMA permitirá filtrar el portafolio por uno o varios niveles del semáforo, más «Sin plazo».
- RF-33: MIENTRAS haya un filtro de semáforo, EL SISTEMA ocultará las filas cuyo nivel no esté entre los elegidos.
- RF-34: MIENTRAS haya un filtro de semáforo, EL SISTEMA ocultará las tarjetas que se queden sin filas visibles.
- RF-35: EL SISTEMA permitirá ordenar las filas de todas las tarjetas por nombre, plan, real o brecha, ascendente o descendente.
- RF-36: EL SISTEMA ordenará las filas por brecha ascendente mientras el usuario no elija otro orden, de modo que las más atrasadas salgan primero.
- RF-37: EL SISTEMA recordará en el navegador del usuario el filtro y el orden elegidos.
- RF-38: SI ningún programa ni proyecto cumple el filtro, ENTONCES EL SISTEMA mostrará un mensaje que lo indique y un botón para quitar el filtro.

### Resumen del programa
- RF-39: DONDE el proyecto sea un programa, EL SISTEMA mostrará en su Resumen su avance agregado junto al de sus tareas propias.
- RF-40: DONDE el proyecto sea un programa, EL SISTEMA mostrará en su Resumen la misma tabla de hijos directos que el portafolio.

## Requisitos no funcionales
- Tiempo real: todas las cifras se calculan en cada consulta, a la fecha de hoy; no se guardan fotos.
- Rendimiento: el portafolio con 50 proyectos de 2 000 tareas cada uno responde en menos de 1 s en el servidor de previsualización.
- Idioma: todos los textos visibles en español, desde el catálogo de traducciones.
- Sin términos de un sector concreto en textos, valores por defecto ni ejemplos.
- La pantalla se usa a partir de 360 px de ancho sin desplazamiento horizontal de la página; la tabla puede desplazarse dentro de su tarjeta.

## Casos límite
- Programa sin descendientes ni tareas: su tarjeta muestra 0 proyectos y «Sin plazo».
- Proyecto sin tareas: su esfuerzo es 0 y no aporta salvo que tenga un peso fijado.
- Proyecto que se mueve fuera de un programa: deja de contar en él desde la siguiente consulta.
- Desmarcar un programa con programas anidados: los anidados siguen siendo programas y su tarjeta sigue apareciendo.
- Programa archivado: no aparece en el portafolio, y sus descendientes tampoco cuentan por él, aunque pueden aparecer en otra tarjeta si tienen otro programa por encima o en «Sin programa».
- Programa cerrado con descendientes activos: aparece con su real y sin plan (RF-20).
- Peso fijado a 0: el proyecto aparece en la tabla pero no aporta al agregado.

## Fuera de alcance
- Cortes guardados y consultar el portafolio a otra fecha.
- Pesos distintos para un mismo proyecto según el programa.
- Crear o mover proyectos desde el portafolio.
- Restringir el portafolio a una parte de la organización.
- Exportar a PDF o Excel.
- Seguimiento de varios proyectos con Gantt de hitos (spec 011) y mapa estratégico (spec 012).

## Criterios de finalización
- Todos los RF con test en verde, incluido el test de matriz para marcar programas y fijar pesos.
- Demo manual:
  1. Como administrador, marcar un proyecto como programa con tres proyectos hijos de esfuerzos distintos y comprobar a mano el plan y el real agregados.
  2. Fijar a mano el peso de un hijo y ver cómo cambia el agregado.
  3. Anidar un programa dentro de otro y ver que sus proyectos cuentan una sola vez en el de arriba.
  4. Archivar un hijo y ver que deja de contar.
  5. Filtrar por «Atrasado», ordenar por brecha y abrir el Resumen de un proyecto desde su fila.
  6. Ver la tarjeta «Sin programa».
  7. Intentar marcar un programa como PM y ver el error de permisos.

## Decisiones tomadas con el usuario (2026-11-03)
- **Peso por defecto:** el esfuerzo del proyecto; el peso fijado a mano manda.
- **Anidamiento:** cuenta todo el subárbol, las tareas propias del programa incluidas; un programa puede contener otros programas.
- **Acceso:** según el acceso de cada usuario, que hoy es todo el portafolio para quien tiene lectura de trabajo; no hace falta un permiso nuevo para verlo.
- **Cerrados y archivados:** los cerrados cuentan con su avance; los archivados no.
- **Permiso:** solo el administrador del sistema marca programas y fija pesos.
- **Pantalla:** filtrar por semáforo, ordenar, ver los proyectos sin programa e ir al Resumen de cada uno.
- **Bajo plan:** semáforo «En riesgo» o «Atrasado» (RF-24).

## Dudas abiertas
Ninguna.
