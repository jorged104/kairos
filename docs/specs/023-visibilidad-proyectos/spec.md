# Spec 023 — Quién ve cada proyecto, y el rol Dirección

**Estado:** Aprobada (2026-11-10) («ya con eso le das»), con los tres puntos de «Dudas abiertas» confirmados.

**Decisiones tomadas con el usuario (2026-11-10):**
- Un proyecto lo ven su equipo, los equipos de encima y de debajo, quien tiene un papel en él y quien lidera ese ámbito.
- Quien tiene una tarea asignada en un proyecto que no ve, ve esa tarea y el proyecto en modo lectura.
- Se añade el rol global **Dirección**: ve todo y no administra.
- Las cifras de un programa se calculan con todos sus proyectos, y la lista muestra solo los que se ven, con un aviso de cuántos faltan. Es la recomendación propuesta; el usuario no la cambió.
- Los miembros y los usuarios de solo lectura no tienen acceso a Portafolio ni a Seguimiento.

## Contexto y objetivo
Hoy cualquier usuario activo ve todos los proyectos que no son plantilla (spec 004). Esa regla se pensó para un equipo interno de 20–50 personas. Con varias áreas, todo el mundo ve el trabajo de todos: proyectos, tareas, Portafolio, Seguimiento, menciones, búsquedas, el MCP y el agente de informes.

Las reglas para escribir ya distinguen equipos y papeles (specs 004 y 008). Las de lectura no. Además, cada pantalla decide por su cuenta qué proyectos enseña, así que una regla nueva hay que repetirla en muchos sitios.

Esta spec:
- define **una única regla** de quién ve cada proyecto, que aplican igual todas las pantallas, la API, el MCP y el agente;
- separa **ver** de **gestionar** donde hoy están mezclados;
- añade el rol **Dirección** para quien necesita ver toda la organización sin administrarla.

## Usuarios / actores
- **Administrador:** ve y gestiona todo, como hoy.
- **Dirección:** ve todos los proyectos, el Portafolio y el Seguimiento completos y el panel de cualquier persona. No administra usuarios, catálogo ni sistema.
- **Líder:** ve los proyectos de los equipos que lidera y sus subequipos, además de los que vería como cualquier persona. Usa Portafolio y Seguimiento sobre los proyectos que ve.
- **Miembro y solo lectura:** ven los proyectos de sus equipos (de encima y de debajo), aquellos en los que tienen un papel y aquellos en los que tienen tareas asignadas. No tienen acceso a Portafolio ni a Seguimiento.

## Conceptos
- **Equipos de una persona:** los equipos a los que pertenece directamente.
- **Proyecto visible para una persona:** un proyecto, que no es plantilla ni la bandeja de tareas sin proyecto, que cumple alguna de estas condiciones:
  1. la persona tiene un rol global que ve todos los proyectos (administrador o Dirección);
  2. la persona es su responsable o tiene en él un papel dado expresamente (PM, líder técnico, miembro u observador; spec 008);
  3. el equipo del proyecto es uno de los equipos de la persona, o un antecesor o un descendiente de alguno de ellos;
  4. el equipo del proyecto está en el ámbito de liderazgo de la persona (los equipos que lidera y sus subequipos);
  5. la persona tiene asignada alguna tarea del proyecto, abierta o cerrada. Este acceso es **solo de lectura**.
- **Programa visible:** un programa (spec 010) visible por la regla anterior, o que tiene algún proyecto descendiente visible.
- **Acceso de lectura por tarea asignada:** el que da solo la condición 5. Permite ver el proyecto y sus tareas, pero no editar nada salvo las tareas propias (RF-15).

## Historias de usuario
- H1: Como persona de un área quiero ver solo los proyectos de mi área y aquellos en los que trabajo para no perderme en el trabajo de toda la organización.
- H2: Como responsable de un área quiero que los proyectos de mi área no los vean otras áreas salvo quien participa en ellos.
- H3: Como persona a la que asignaron una tarea en el proyecto de otra área quiero verla y trabajarla aunque no sea de mi equipo.
- H4: Como dirección quiero ver toda la organización, su Portafolio y su Seguimiento, sin poder cambiar su configuración.
- H5: Como líder quiero ver en el Portafolio cómo va un programa aunque no vea todos sus proyectos.

## Requisitos funcionales (criterios de aceptación en EARS)

### Regla única
- RF-1: EL SISTEMA solo mostrará a cada persona los proyectos visibles para ella (Conceptos).
- RF-2: EL SISTEMA aplicará la misma regla de visibilidad en la ficha del proyecto y sus pestañas, en Proyectos (Kanban, lista y cronograma), en Portafolio y Seguimiento para quien tiene acceso (RF-17a), en Mi panel, en Mi equipo, en búsquedas y menciones, en objetivos, en las vistas guardadas, en el MCP y en el agente de informes.
- RF-3: SI una persona pide un proyecto, un hito, una tarea, una página, un comentario o un archivo de un proyecto que no ve, ENTONCES EL SISTEMA responderá como si no existiera.
- RF-4: EL SISTEMA aplicará los cambios de visibilidad (nuevo equipo, nuevo papel, nueva tarea asignada, cambio de equipo del proyecto) dentro de la ventana de permisos de la spec 003 (RF-44 de 003).
- RF-5: EL SISTEMA no cambiará con esta spec quién puede crear, gestionar o escribir en un proyecto (specs 004 y 008), salvo lo indicado en RF-15.

### Rol Dirección
- RF-6: EL SISTEMA tendrá cinco roles globales, en orden de prioridad descendente: `admin`, `director`, `lead`, `member` y `viewer`; el nombre visible de `director` es «Dirección».
- RF-7: EL SISTEMA concederá a Dirección: ver todos los proyectos, el Portafolio y el Seguimiento completos, el panel de cualquier persona en modo lectura, comentar tareas y compartir vistas de la lista de proyectos.
- RF-8: EL SISTEMA concederá a Dirección gestionar los proyectos en los que es PM y escribir en las tareas de los proyectos en los que trabaja, como a un miembro.
- RF-9: EL SISTEMA no concederá a Dirección gestionar usuarios, equipos, catálogo, programas, gamificación ni sistema.
- RF-10: SI un líder intenta actuar sobre una persona con el rol Dirección, ENTONCES EL SISTEMA lo rechazará como hoy con un `lead` o un `admin` (spec 003, RF-63).
- RF-11: EL SISTEMA permitirá asignar el rol Dirección solo al administrador y homologarlo desde el IdP (spec 003).

### Ver y gestionar, por separado
- RF-12: EL SISTEMA mostrará los miembros y papeles de un proyecto a quien lo ve, y solo permitirá cambiarlos a quien gestiona su equipo (spec 008, RF-33).
- RF-13: EL SISTEMA mostrará el cronograma por persona de un proyecto (spec 021) solo a quien gestiona su equipo, como hoy.
- RF-14: EL SISTEMA dará los nombres de los estados, tipos, prioridades y etiquetas de un proyecto a quien lo ve.

### Tareas asignadas en proyectos ajenos
- RF-15: MIENTRAS una persona vea un proyecto solo por tener tareas asignadas, EL SISTEMA le permitirá cambiar el estado, el avance y las horas de esas tareas y comentarlas, y no le permitirá crear tareas ni editar otras.
- RF-16: CUANDO se quite a una persona de la última tarea que tenía asignada en un proyecto que solo veía por ella, EL SISTEMA dejará de mostrarle ese proyecto.

### Acceso a Portafolio y Seguimiento
- RF-17a: EL SISTEMA ofrecerá Portafolio y Seguimiento, en la navegación, en la API y en el MCP, solo a los roles administrador, Dirección y líder.
- RF-17b: SI una persona sin ese acceso abre Portafolio o Seguimiento, por enlace directo, por la API o desde el MCP, ENTONCES EL SISTEMA lo rechazará con un error de permisos.
- RF-17c: MIENTRAS una persona no tenga acceso a Portafolio, EL SISTEMA no le mostrará enlaces que lleven a él desde otras pantallas.

### Portafolio
- RF-17: EL SISTEMA mostrará en el Portafolio solo los programas visibles y, dentro de cada uno, solo sus proyectos visibles.
- RF-18: EL SISTEMA calculará el avance, el plan y la brecha de un programa con todos sus proyectos, los vea o no la persona.
- RF-19: SI un programa tiene proyectos que la persona no ve, ENTONCES EL SISTEMA mostrará en su tarjeta «N proyectos que no ves», sin su nombre ni sus cifras.
- RF-20: EL SISTEMA contará en el resumen del Portafolio («N proyectos, M bajo plan») solo los proyectos visibles, salvo dentro de la tarjeta de cada programa (RF-18).

### Seguimiento
- RF-21: MIENTRAS no haya ningún filtro elegido, EL SISTEMA seguirá los proyectos propios de la persona: aquellos de los que es responsable, en los que tiene un papel, que son de alguno de sus equipos (no de los de encima) o en los que tiene tareas asignadas.
- RF-22: SI no hay filtros y la persona no tiene proyectos propios, ENTONCES EL SISTEMA seguirá todos los proyectos activos visibles para ella. Sustituye a la spec 011, RF-7.
- RF-23: EL SISTEMA ofrecerá en los filtros de programa, proyecto y responsable de Seguimiento solo valores de proyectos visibles.

### Proyectos y vistas
- RF-24: EL SISTEMA mostrará en los tres modos de Proyectos solo los proyectos visibles. Concreta la spec 012, RF-29.
- RF-25: CUANDO una persona abra una vista compartida que filtra por proyectos que no ve, EL SISTEMA aplicará la vista solo sobre los proyectos visibles, sin error.

### Paneles, cronogramas y referencias
- RF-26: CUANDO una persona vea el panel o el cronograma de otra (specs 013 y 021), EL SISTEMA mostrará las tareas de proyectos que no ve como «Tarea de otro proyecto», con sus fechas y sin título, clave ni proyecto, y no permitirá abrirlas.
- RF-27: EL SISTEMA contará como bloqueante, sin revelarla, una tarea de un proyecto que no se ve (spec 007, RF-32).
- RF-28: EL SISTEMA mostrará como «sin acceso» la mención a un proyecto, hito o tarea que no se ve (spec 005, RF-28).
- RF-29: EL SISTEMA calculará el avance de un objetivo con todos sus proyectos ligados y le mostrará a cada persona solo los visibles, con «N proyectos que no ves».

### Cambio de estado forzado
- RF-30: EL SISTEMA permitirá saltarse el flujo de estados de un tipo (spec 004, RF-86 a RF-89) a quien tenga ese permiso, que solo tiene el administrador, y no por el nombre de su rol.

### Matriz
- RF-31: EL SISTEMA definirá la visibilidad con permisos de la matriz única: ver todos los proyectos (administrador y Dirección) y ver los proyectos del ámbito de liderazgo (líder); las condiciones 2, 3 y 5 se aplican a todos los roles.
- RF-32: EL SISTEMA definirá el acceso a Portafolio y Seguimiento con un permiso de la matriz única (administrador, Dirección y líder), no por el nombre del rol.

## Requisitos no funcionales
- **Seguridad:** ningún listado, contador, búsqueda, exportación, respuesta del MCP o del agente revela el título, la clave o las cifras de un proyecto que no se ve. Lo único que se revela es el número de proyectos ocultos de un programa o un objetivo (RF-19, RF-29).
- **Rendimiento** (entorno local, 500 proyectos, 5 000 tareas, 100 usuarios): Proyectos, Portafolio y Seguimiento responden en menos de 500 ms, y no más de un 20 % por encima del tiempo actual con los mismos datos.
- **Compatibilidad:** los cambios de esquema son aditivos; la versión anterior de la API sigue funcionando durante el despliegue.
- **Idioma:** todos los textos visibles en español, desde el catálogo de traducciones; los errores se traducen por su código.

## Casos límite
- **Persona sin equipo, sin papeles ni tareas:** no ve ningún proyecto; Proyectos, Portafolio y Seguimiento muestran su estado vacío.
- **Proyecto sin equipo o de un equipo archivado:** lo ven su responsable, quien tiene un papel en él, quien tiene tareas asignadas y quien ve todos.
- **Equipo de una persona movido a otro padre:** la visibilidad cambia con la nueva jerarquía dentro de la ventana de RF-4.
- **Observador añadido expresamente:** ve el proyecto aunque no sea de su equipo; sigue sin poder escribir (spec 008, RF-42).
- **Subtarea asignada en un proyecto ajeno:** da acceso de lectura al proyecto entero (RF-15), no solo a la rama.
- **Tarea reasignada a otra persona:** quien la tenía deja de ver el proyecto si no le queda otra condición (RF-16).
- **Persona con varios roles (por ejemplo `lead` y `director`):** tiene la unión de sus permisos (spec 003, RF-58).
- **Programa sin proyectos visibles y que la persona no ve por sí mismo:** no aparece en el Portafolio.
- **Dependencia entre proyectos:** quien no ve el otro proyecto la ve como bloqueante oculta (RF-27).
- **Informe con IA de un proyecto con subproyectos que no se ven:** el agente solo recibe los subproyectos visibles para quien genera el informe.
- **Notificación de una tarea que se deja de ver:** al abrirla responde como si no existiera (RF-3).

## Fuera de alcance
- Proyectos privados o secretos que no vea ni su equipo.
- Permisos de lectura distintos por pestaña o por campo (por ejemplo, ocultar solo las horas).
- Visibilidad por objetivo, por programa o por atributo de usuario.
- Cambiar quién gestiona o escribe, salvo RF-15.
- Que los flujos de estados usen el papel en el proyecto en lugar del rol global.
- Roles globales configurables desde la interfaz.

## Cambios a otras specs (se aplican al aprobar)
- **003:** RF-57 pasa de cuatro a cinco roles (RF-6); la matriz de permisos incluye Dirección.
- **004:** la nota de visibilidad («todo lo que no es plantilla lo ven todos los usuarios activos») se sustituye por esta spec.
- **010:** «Cualquier usuario con lectura de trabajo consulta el portafolio completo» pasa a RF-17a a RF-20, y RF-21 (Portafolio para todo usuario con lectura de trabajo) pasa a RF-17a; se quita «Restringir el portafolio a una parte de la organización» de su «Fuera de alcance».
- **011:** RF-1 (Seguimiento para todo usuario con lectura de trabajo) pasa a RF-17a; RF-6 y RF-7 se concretan en RF-21 y RF-22.
- **012:** RF-29 se concreta en RF-24.

## Criterios de finalización
- Todos los RF cubiertos por tests automáticos en verde:
  - una tabla de visibilidad en el test de matriz, con una persona por condición (Conceptos 1 a 5) y otra sin acceso, contra cada pantalla, el MCP y el agente;
  - Dirección en la matriz de permisos;
  - el Portafolio con un programa de proyectos mixtos (visibles y no visibles);
  - el acceso de lectura por tarea asignada (RF-15, RF-16).
- Demo manual con los datos de prueba locales:
  1. Entrar como Mario (Backend) y ver APP (Desarrollo), pero no VEN (BI) ni AUT (Innovación), ni Portafolio ni Seguimiento en el menú.
  2. Entrar como Carla (Backend, con una tarea en AUT) y ver AUT solo en lectura, pudiendo mover su tarea.
  3. Entrar como Ana (líder de BI) y ver en el Portafolio un programa con «N proyectos que no ves».
  4. Dar a Víctor el rol Dirección y ver todos los proyectos, el Portafolio y el Seguimiento completos sin acceso a Administración.
  5. Abrir por enlace directo una tarea de un proyecto no visible y recibir «no existe».

## Dudas abiertas
- Ninguna. Puntos confirmados al aprobar:
  - Dirección comparte vistas y puede ser PM de sus proyectos (RF-7, RF-8).
  - En el panel y el cronograma de otra persona, las tareas de proyectos no visibles se ven como «Tarea de otro proyecto» (RF-26), para que la carga siga siendo real.
  - Seguimiento sin filtros usa solo los equipos directos, no los de encima (RF-21), para que «mis proyectos» no sean los de toda el área.
