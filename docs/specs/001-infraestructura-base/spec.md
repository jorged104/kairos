# Spec 001 — Infraestructura base

**Estado:** Aprobada (2026-10-22)

## Contexto y objetivo
Antes de construir cualquier funcionalidad del gestor de proyectos hace falta
un entorno local reproducible en el que base de datos, API y frontend arranquen
y se comuniquen entre sí. Esta spec define el mínimo verificable de esa
conexión: dos comprobaciones de salud en la API y una pantalla que muestra su
resultado. Las tecnologías concretas son las acordadas en la guía de agentes
del proyecto y se concretan en el plan, no aquí.

## Usuarios / actores
- Desarrollador del proyecto (humano o agente de código).
- Herramientas de orquestación que consultan la salud de la API.

## Historias de usuario
- H1: Como desarrollador quiero levantar el entorno local para empezar a trabajar sin configurar nada a mano.
- H2: Como desarrollador quiero aplicar las migraciones de base de datos cuando yo decida para controlar el estado del esquema.
- H3: Como herramienta de orquestación quiero saber si el proceso de la API está vivo para reiniciarlo si no lo está.
- H4: Como herramienta de orquestación quiero saber si la API puede atender peticiones (incluida la base de datos) para no enviarle tráfico si no puede.
- H5: Como desarrollador quiero ver en el navegador el estado de la API y de la base de datos para confirmar de un vistazo que todo está conectado.

## Requisitos funcionales (criterios de aceptación en EARS)

### Entorno local
- RF-1: CUANDO el desarrollador ejecute el comando de arranque de la base de datos, EL SISTEMA pondrá en marcha la base de datos.
- RF-2: CUANDO el desarrollador ejecute el comando de arranque de la API, EL SISTEMA pondrá en marcha la API.
- RF-3: CUANDO el desarrollador ejecute el comando de arranque del frontend, EL SISTEMA servirá la pantalla de estado en el navegador.
- RF-4: CUANDO el desarrollador ejecute el comando de arranque unificado, EL SISTEMA pondrá en marcha base de datos, API y frontend con el mismo resultado que RF-1, RF-2 y RF-3 por separado.
- RF-5: MIENTRAS la base de datos no acepte conexiones, el comando de arranque unificado esperará antes de arrancar la API.
- RF-6: SI la base de datos no acepta conexiones en 60 segundos, ENTONCES el comando de arranque unificado mostrará un mensaje de error y terminará con código de salida distinto de 0.
- RF-7: EL SISTEMA no aplicará migraciones de base de datos al arrancar.
- RF-8: CUANDO el desarrollador ejecute el comando de migración, EL SISTEMA aplicará todas las migraciones pendientes en orden.
- RF-9: SI no hay migraciones pendientes, ENTONCES el comando de migración terminará con éxito sin modificar la base de datos.
- RF-10: SI la base de datos no está disponible al ejecutar el comando de migración, ENTONCES EL SISTEMA mostrará un mensaje de error y terminará con código de salida distinto de 0.

### Salud de la API
- RF-11: CUANDO se consulte el endpoint de vida, EL SISTEMA responderá con éxito mientras el proceso de la API esté en marcha.
- RF-12: EL SISTEMA no consultará la base de datos para responder al endpoint de vida.
- RF-13: CUANDO se consulte el endpoint de disponibilidad y la base de datos responda, EL SISTEMA responderá con éxito indicando que la base de datos está disponible.
- RF-14: SI la base de datos no responde al consultar el endpoint de disponibilidad, ENTONCES EL SISTEMA responderá con un código de fallo indicando que la base de datos no está disponible.
- RF-15: SI la base de datos tarda más de 2 segundos en responder, ENTONCES EL SISTEMA la considerará no disponible.
- RF-16: SI la base de datos no está disponible cuando arranca la API, ENTONCES EL SISTEMA no arrancará la API, mostrará un mensaje de error y terminará con código de salida distinto de 0.
- RF-17: EL SISTEMA devolverá las respuestas de error con el formato de error común del proyecto.
- RF-18: EL SISTEMA registrará cada petición recibida en formato estructurado con método, ruta, código de respuesta y duración.

### Pantalla de estado
- RF-19: CUANDO el usuario abra la pantalla de estado, EL SISTEMA consultará la disponibilidad de la API.
- RF-20: CUANDO la consulta responda con éxito, EL SISTEMA mostrará la API como disponible.
- RF-21: CUANDO la consulta responda, EL SISTEMA mostrará el estado de la base de datos que informe la API (disponible o no disponible).
- RF-22: SI la consulta no recibe respuesta en 5 segundos, ENTONCES EL SISTEMA la dará por fallida.
- RF-23: SI la consulta falla sin respuesta de la API, ENTONCES EL SISTEMA mostrará la API como no disponible.
- RF-24: SI la consulta falla sin respuesta de la API, ENTONCES EL SISTEMA mostrará el estado de la base de datos como desconocido.
- RF-25: SI la API o la base de datos no están disponibles, ENTONCES EL SISTEMA mostrará un mensaje de error visible en la pantalla.
- RF-26: MIENTRAS la pantalla de estado esté abierta, EL SISTEMA repetirá la consulta cada 30 segundos.
- RF-27: CUANDO cambie el resultado de una consulta repetida, EL SISTEMA actualizará la pantalla sin que el usuario recargue la página.
- RF-28: EL SISTEMA mostrará los textos de la pantalla en español.
- RF-29: EL SISTEMA obtendrá todos los textos visibles de la pantalla de un catálogo de traducciones por idioma, sin textos fijos en la interfaz.
- RF-30: SI la interfaz usa una clave de texto que no existe en el catálogo de español, ENTONCES los tests automáticos del frontend fallarán indicando la clave ausente.

## Requisitos no funcionales
- Seguridad: ninguna credencial real se guarda en el repositorio; la configuración se toma del entorno y el repositorio incluye un ejemplo de configuración con valores de desarrollo.
- Reproducibilidad: un desarrollador con un clon limpio puede completar RF-1 a RF-8 siguiendo solo los comandos documentados. Se presupone instalado Docker, Go y Node.
- Plataforma: los servicios se ejecutan en contenedores Linux. No se exige compatibilidad nativa con Windows ni macOS.
- Idioma: español como único idioma en esta iteración; añadir otro idioma solo requiere añadir su catálogo de textos (RF-29).

## Casos límite
- Base de datos parada antes de arrancar la API: la API no arranca (RF-16).
- Comando unificado con la base de datos tardando en estar lista: espera antes de arrancar la API (RF-5), con un límite (RF-6).
- Base de datos que cae con la API ya en marcha: la siguiente consulta de disponibilidad la informa como no disponible (RF-14) y la pantalla lo refleja en el siguiente ciclo, como máximo en 35 s (RF-26 + RF-22).
- Base de datos que vuelve: la pantalla vuelve a disponible en el siguiente ciclo sin recargar (RF-27).
- API parada con la pantalla abierta: la pantalla muestra API no disponible y BD desconocida (RF-23, RF-24).
- Consulta lenta (> 5 s): se trata como fallida aunque termine respondiendo (RF-22).
- Migración ejecutada dos veces seguidas: la segunda no cambia nada (RF-9).
- Base de datos sin ninguna migración aplicada: la API y los endpoints de salud funcionan igualmente.
- Clave de texto ausente en el catálogo: se detecta como fallo en los tests, no en ejecución (RF-30).

## Fuera de alcance
- Despliegue a producción (TLS, dominios, entornos remotos).
- Autenticación y usuarios.
- Tablas y lógica de negocio (objetivos, proyectos, hitos, actividades, gamificación).
- Tiempo real (WebSocket).
- CI/CD.
- Idiomas distintos del español y selector de idioma.
- Ejecución nativa fuera de contenedores Linux.
- Cualquier pantalla distinta de la de estado, y cualquier navegación o diseño más allá de mostrar los estados.

## Criterios de finalización
- RF-11 a RF-18 cubiertos por tests automáticos del backend en verde.
- RF-30 cubierto por tests automáticos del frontend en verde.
- Demo manual desde un clon limpio:
  0. Con la base de datos parada, arrancar la API y comprobar que termina con error (RF-16).
  1. Arrancar el entorno con el comando unificado y aplicar migraciones con los comandos documentados.
  2. Abrir la pantalla y ver API y BD disponibles.
  3. Parar la base de datos y ver BD no disponible y mensaje de error en ≤ 35 s, sin recargar.
  4. Volver a arrancar la base de datos y ver BD disponible en ≤ 35 s, sin recargar.
  5. Parar la API y ver API no disponible y BD desconocida en ≤ 35 s.

## Dudas abiertas
- Aplazada: no existe `docs/constitution.md`; cuando se cree, contrastar esta spec con ella.
