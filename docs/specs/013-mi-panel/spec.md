# Spec 013 — Mi panel

**Estado:** Aprobada (2026-11-04) («dale»).

**Cambio:** 2026-11-05, a petición del usuario.
- La tarjeta del Kanban lleva el color del tipo de la tarea (RF-14a).
- La tarea se abre en el panel lateral, como en los proyectos (RF-15).
- Las columnas son más fáciles de desplazar (RF-14b, RF-14c).
- Cada tarea de la respuesta trae su tipo (`type_id`).

## Contexto y objetivo
Hoy, al entrar en Kairos, cada persona ve el estado técnico del sistema. No ve su trabajo: qué
tareas tiene asignadas, cuáles van atrasadas, en qué proyectos participa ni cómo evoluciona.
Para saberlo tiene que recorrer proyecto por proyecto.

Esta spec convierte la página de inicio en **Mi panel**, que reúne en una sola pantalla:
- una tarjeta de perfil con un resumen;
- un Kanban con las tareas asignadas, por estado y con las más atrasadas primero;
- los proyectos en los que la persona aporta;
- tres gráficas de su avance personal.

Todo se calcula en tiempo real a la fecha de hoy y los conceptos sirven a cualquier industria.

## Usuarios / actores
- **Cualquier usuario con lectura de trabajo:** ve su propio panel y mueve sus tareas según los permisos y el flujo que ya existen.
- Nadie ve el panel de otra persona.

## Vocabulario
- **Mis tareas:** las tareas que tienen a la persona entre sus asignados.
  - Solo cuentan las de proyectos activos que no son plantilla.
- **Atrasada:** una tarea abierta cuya fecha fin ya pasó.
- **Cerrada a tiempo:** una tarea cerrada el día de su fecha fin o antes.
  - Una tarea sin fecha fin no cuenta para la puntualidad.
- **Mis proyectos:** los proyectos activos en los que la persona aporta, por cualquiera de estas vías:
  - es su responsable;
  - es miembro del equipo del proyecto (spec 008);
  - tiene alguna tarea asignada en él.

## Historias de usuario
- H1: Como persona del equipo quiero ver al entrar todas mis tareas asignadas, con las más atrasadas primero, para saber qué atender hoy.
- H2: Como persona del equipo quiero mover mis tareas de estado desde el panel para no tener que abrir cada proyecto.
- H3: Como persona del equipo quiero ver en qué proyectos aporto, con mi papel y mis tareas abiertas en cada uno.
- H4: Como persona del equipo quiero ver gráficas de lo que cierro y de mi puntualidad para saber cómo voy.
- H5: Como persona del equipo quiero una tarjeta con mis datos y un resumen de mi carga.

## Requisitos funcionales (criterios de aceptación en EARS)

### Página
- RF-1: CUANDO un usuario con lectura de trabajo entre en la página de inicio, EL SISTEMA le mostrará Mi panel.
- RF-2: EL SISTEMA mostrará el estado técnico del sistema en una ruta propia, accesible desde la navegación.
- RF-3: EL SISTEMA mostrará en Mi panel solo datos del usuario que lo mira.
- RF-4: EL SISTEMA calculará todas las cifras del panel a la fecha de hoy del usuario.

### Tarjeta de perfil
- RF-5: EL SISTEMA mostrará en la tarjeta de perfil las iniciales del usuario, su nombre, sus roles y sus equipos.
- RF-6: EL SISTEMA mostrará en la tarjeta de perfil el número de tareas abiertas, el de atrasadas y el de cerradas en el mes en curso.
- RF-7: CUANDO el usuario pulse el enlace de la tarjeta de perfil, EL SISTEMA abrirá Mi cuenta.

### Kanban de mis tareas
- RF-8: EL SISTEMA mostrará en el Kanban una columna por cada estado activo del catálogo de tareas, en el orden del catálogo, con su número de tareas.
- RF-9: DONDE alguna tarea del usuario esté en un estado retirado, EL SISTEMA mostrará una columna para ese estado, marcada como retirada.
- RF-10: EL SISTEMA mostrará en las columnas de estados abiertos todas las tareas abiertas del usuario.
- RF-11: EL SISTEMA mostrará en las columnas de estados cerrados solo las tareas cerradas en los últimos 14 días.
- RF-12: EL SISTEMA ordenará las tareas de cada columna abierta por días de atraso descendente, después por fecha fin ascendente (las que no tienen fecha, al final) y después por clave.
- RF-13: EL SISTEMA ordenará las tareas de cada columna cerrada por fecha de cierre, de la más reciente a la más antigua.
- RF-14: EL SISTEMA mostrará en cada tarjeta la clave, el título, el proyecto, la fecha fin y, si está atrasada, los días de atraso destacados.
- RF-14a: EL SISTEMA mostrará arriba de cada tarjeta una línea con el color del tipo de la tarea, según el catálogo, y el nombre del tipo al pasar por encima.
- RF-14b: MIENTRAS el puntero esté sobre el Kanban y las columnas no quepan, EL SISTEMA desplazará las columnas en horizontal con la rueda del ratón.
- RF-14c: MIENTRAS las columnas no quepan, EL SISTEMA mostrará botones para desplazarlas una columna a la izquierda o a la derecha.
- RF-15: CUANDO el usuario pulse una tarjeta, EL SISTEMA abrirá la ficha de esa tarea en el panel lateral, sobre Mi panel, con la misma animación que en los proyectos (cambio de 2026-11-05: antes se abría la página completa).
- RF-16: CUANDO el usuario suelte una tarjeta en otra columna, EL SISTEMA cambiará el estado de la tarea al de esa columna.
- RF-17: EL SISTEMA aplicará al mover una tarea desde el panel los mismos permisos, flujo de estados y efectos que al moverla en su proyecto.
- RF-18: MIENTRAS el usuario arrastre una tarjeta, EL SISTEMA marcará como no disponibles las columnas a las que el flujo de esa tarea no permite moverla.
- RF-19: SI el cambio de estado falla, ENTONCES EL SISTEMA devolverá la tarjeta a su columna y mostrará el error traducido.
- RF-20: EL SISTEMA ofrecerá, para cada tarjeta que el usuario pueda mover, una alternativa sin arrastre para elegir el nuevo estado.
- RF-21: SI una columna tiene más de 100 tareas, ENTONCES EL SISTEMA mostrará las 100 primeras según su orden y un aviso con cuántas faltan.
- RF-22: SI el usuario no tiene ninguna tarea abierta ni cerrada en los últimos 14 días, ENTONCES EL SISTEMA mostrará un mensaje que lo indique en lugar del Kanban.

### Mis proyectos
- RF-23: EL SISTEMA mostrará una lista de mis proyectos ordenada por número de tareas abiertas mías atrasadas, de más a menos, y después por nombre.
- RF-24: EL SISTEMA mostrará en cada proyecto su nombre, el papel del usuario (responsable, papel en el equipo del proyecto o «con tareas») y el número de tareas abiertas y atrasadas del usuario en él.
- RF-25: EL SISTEMA mostrará en cada proyecto su avance real y su semáforo con las reglas de la spec 009.
- RF-26: CUANDO el usuario pulse un proyecto de la lista, EL SISTEMA abrirá su Resumen.
- RF-27: SI el usuario no aporta a ningún proyecto activo, ENTONCES EL SISTEMA mostrará un mensaje que lo indique.

### Gráficas de avance
- RF-28: EL SISTEMA mostrará una gráfica de barras con las tareas que el usuario cerró en cada una de las últimas 12 semanas, la actual incluida.
- RF-29: EL SISTEMA mostrará en cada barra semanal el número de tareas cerradas y, al pasar por encima o al enfocarla, su suma de pesos.
- RF-30: EL SISTEMA mostrará la puntualidad del usuario en las últimas 12 semanas: el porcentaje de tareas cerradas a tiempo sobre las cerradas con fecha fin.
- RF-31: EL SISTEMA mostrará la puntualidad de cada una de las últimas 12 semanas junto a la cifra total.
- RF-32: SI en el periodo no hay tareas cerradas con fecha fin, ENTONCES EL SISTEMA mostrará «Sin datos» en lugar del porcentaje.
- RF-33: EL SISTEMA mostrará una gráfica de la carga actual del usuario, con sus tareas abiertas por proyecto y las atrasadas destacadas dentro de cada una.
- RF-34: SI el usuario tiene tareas abiertas en más de 8 proyectos, ENTONCES EL SISTEMA mostrará los 8 con más tareas y agrupará el resto en «Otros».
- RF-35: EL SISTEMA ofrecerá cada gráfica también como tabla de datos accesible para lectores de pantalla.
- RF-36: EL SISTEMA contará como cerradas por el usuario las tareas que tiene asignadas en el momento de la consulta, en la semana de su fecha de cierre.

## Requisitos no funcionales
- **Tiempo real:** las cifras se calculan en cada consulta; no se guardan fotos.
- **Rendimiento:** con un usuario que tiene 500 tareas abiertas en 20 proyectos, el panel responde en menos de 500 ms en el servidor de previsualización.
- **Idioma:** todos los textos visibles en español, desde el catálogo de traducciones.
- **Sin términos de un sector concreto** en textos, valores por defecto ni ejemplos.
- **Ancho mínimo:** la pantalla se usa a partir de 360 px sin desplazamiento horizontal de la página; en un móvil el Kanban se desplaza dentro de su caja y las tarjetas se apilan.
- **Accesibilidad:** las gráficas tienen texto alternativo y tabla de datos, y lo que se hace arrastrando tiene una alternativa con teclado.

## Casos límite
- **Tarea con varios asignados:** aparece en el panel de cada uno y cuenta en las gráficas de todos.
- **Tarea reasignada después de cerrarse:** cuenta para quien la tiene asignada en el momento de la consulta (RF-36).
- **Tarea reabierta:** vuelve a su columna abierta y deja de contar como cerrada en las gráficas.
- **Proyecto cerrado o archivado:** sus tareas salen del panel y el proyecto sale de «Mis proyectos».
- **Usuario sin tareas ni proyectos:** la tarjeta de perfil muestra ceros, y el Kanban y «Mis proyectos» muestran su mensaje.
- **Tarea sin fecha fin:** nunca está atrasada, va al final de su columna y no cuenta para la puntualidad.
- **Usuario solo con permiso de lectura** (observador): ve su panel pero no puede mover tarjetas que no le dejan editar.

## Fuera de alcance
- Ver el panel de otra persona o de un equipo.
- XP, monedas, niveles y logros (gamificación, spec futura).
- Personalizar el panel (mover u ocultar tarjetas o gráficas).
- Notificaciones o recordatorios de tareas atrasadas.
- Filtros y vistas guardadas en el Kanban personal.
- Crear tareas desde el panel.
- Carga frente a capacidad semanal.
- Mapa estratégico (pasa a ser la spec 014).

## Criterios de finalización
- Todos los RF con test en verde:
  - API: tareas, orden, columnas cerradas de 14 días, proyectos y papeles, series de las gráficas y aislamiento entre usuarios;
  - frontend: perfil, Kanban (arrastre, alternativa, flujo, error), proyectos y gráficas con sus tablas.
- Demo manual:
  1. Entrar como miembro y ver Mi panel en el inicio, con el estado del sistema en su propia ruta.
  2. Comprobar que la tarea más atrasada aparece la primera.
  3. Mover una tarea a «En curso» arrastrando y otra con el menú.
  4. Intentar moverla a un estado que el flujo no permite.
  5. Cerrar una tarea fuera de plazo y ver cómo cambian la gráfica semanal y la puntualidad.
  6. Ver un proyecto donde solo se tienen tareas, con el papel «con tareas».

## Decisiones tomadas con el usuario (2026-11-04)
- **Columnas:** una por estado del catálogo de tareas, y se pueden mover como en el kanban del proyecto.
- **Completadas:** solo las cerradas en los últimos 14 días.
- **Mis proyectos:** aquellos de los que soy responsable, en cuyo equipo estoy o en los que tengo tareas.
- **Gráficas:** tareas cerradas por semana, puntualidad y carga actual por proyecto.
- **Ubicación:** Mi panel sustituye a la página de inicio, y el estado del sistema pasa a su propia ruta.
- **Perfil:** iniciales, nombre, roles y equipos, más tres cifras (abiertas, atrasadas y cerradas este mes) y un enlace a Mi cuenta.

## Dudas abiertas
Ninguna.
