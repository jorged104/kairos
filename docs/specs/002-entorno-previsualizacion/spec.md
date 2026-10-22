# Spec 002 — Entorno de previsualización

**Estado:** Aprobada (2026-10-22)

## Contexto y objetivo
Hoy el proyecto solo se ve en la máquina de quien lo arranca. Para trabajar en remoto hace
falta una URL estable con la última versión de la rama principal, actualizada sola tras cada
cambio. El entorno vive en un servidor doméstico que ya aloja otros servicios (Home Assistant)
y los publica en Internet a través de un túnel y un proxy inverso existentes. Esta spec
define cómo llega un cambio a esa URL y qué pasa cuando algo falla. No es un entorno de
producción.

## Usuarios / actores
- Desarrollador del proyecto, que publica cambios y consulta el resultado.
- Cualquier persona con el enlace, que abre la URL de previsualización.

## Historias de usuario
- H1: Como desarrollador quiero que cada cambio en la rama principal se despliegue solo para ver el resultado desde cualquier sitio.
- H2: Como desarrollador quiero que un cambio roto no tire la URL para que siempre haya una versión que funcione.
- H3: Como desarrollador quiero ver en el repositorio si cada despliegue ha ido bien para no tener que entrar al servidor.
- H4: Como desarrollador quiero que los datos de previsualización sobrevivan a los despliegues para poder probar con datos acumulados.


## Requisitos funcionales (criterios de aceptación en EARS)

### Disparo y comprobaciones
- RF-1: CUANDO se publique un cambio en la rama principal, EL SISTEMA iniciará un despliegue al entorno de previsualización.
- RF-2: EL SISTEMA no desplegará cambios de ninguna otra rama.
- RF-3: CUANDO se inicie un despliegue, EL SISTEMA ejecutará los tests automáticos de backend y frontend antes de desplegar.
- RF-4: SI algún test falla, ENTONCES EL SISTEMA no desplegará el cambio.
- RF-5: SI la construcción de la nueva versión falla, ENTONCES EL SISTEMA no desplegará el cambio.
- RF-6: SI se publica un cambio mientras otro despliegue está en curso, ENTONCES EL SISTEMA esperará a que termine el actual antes de iniciar el nuevo.
- RF-7: SI el servidor está apagado cuando se publica un cambio, ENTONCES EL SISTEMA ejecutará el despliegue pendiente cuando el servidor vuelva a estar en marcha.

### Despliegue
- RF-8: CUANDO un despliegue supere RF-3 a RF-5, EL SISTEMA aplicará las migraciones de base de datos pendientes antes de arrancar la nueva versión.
- RF-9: SI las migraciones fallan, ENTONCES EL SISTEMA no arrancará la nueva versión.
- RF-10: SI la nueva versión no informa de que está disponible en 60 segundos tras arrancar, ENTONCES EL SISTEMA volverá a servir la versión anterior.
- RF-11: MIENTRAS un despliegue esté en curso o haya fallado, EL SISTEMA seguirá sirviendo la última versión desplegada con éxito.
- RF-12: EL SISTEMA conservará los datos de la base de datos entre despliegues.
- RF-13: CUANDO el servidor se reinicie, EL SISTEMA volverá a servir la última versión desplegada con éxito, con sus datos, sin intervención manual.

### Resultado y acceso
- RF-14: CUANDO termine un despliegue, EL SISTEMA mostrará en el repositorio, junto al commit desplegado, si tuvo éxito o falló.
- RF-15: CUANDO termine un despliegue, EL SISTEMA dejará consultable desde el repositorio el registro de cada paso.
- RF-16: EL SISTEMA servirá la pantalla de estado de la spec 001 en una URL HTTPS cuyo hostname se configura en el servidor.
- RF-17: EL SISTEMA no incluirá el hostname de la URL en el repositorio ni en los registros de despliegue visibles desde él.
- RF-18: EL SISTEMA permitirá abrir la URL sin autenticación.
- RF-19: EL SISTEMA no expondrá la base de datos fuera del servidor.
- RF-20: EL SISTEMA no requerirá abrir puertos de entrada en la red doméstica.
- RF-21: EL SISTEMA no modificará ni interrumpirá los servicios que ya corren en el servidor.

## Requisitos no funcionales
- Seguridad: ninguna credencial del entorno (base de datos, tokens) se guarda en el repositorio.
- Seguridad: los registros de despliegue visibles en el repositorio no muestran credenciales.
- Privacidad: la URL es privada por desconocida, no por autenticación (RF-17, RF-18). Quien conozca o descubra el hostname puede abrirla; se acepta ese riesgo.
- Compatibilidad: cada migración deja la base de datos en un estado con el que la versión anterior de la API sigue respondiendo como disponible, para que la vuelta atrás de RF-10 funcione.
- Plataforma: el servidor es Linux x86_64 con Docker; el entorno corre en contenedores Linux.
- Tiempo: sin límite de tiempo entre la publicación de un cambio y su despliegue.
- Comportamiento de la app: la pantalla de estado cumple en el entorno los RF-19 a RF-30 de la spec 001.

## Casos límite
- Tests en rojo: no se despliega y la URL sigue mostrando la versión anterior (RF-4, RF-11).
- Migración aplicada y nueva versión que no arranca: se vuelve a la versión anterior (RF-10), que sigue funcionando gracias a la regla de compatibilidad.
- Dos cambios publicados seguidos: se despliegan uno tras otro, en orden (RF-6).
- Servidor apagado durante un push: el despliegue se ejecuta al volver (RF-7).
- Reinicio del servidor: vuelve la última versión buena con sus datos (RF-13).
- Primer despliegue sin versión anterior: si falla, la URL no muestra nada hasta el primer despliegue con éxito.

## Fuera de alcance
- Entornos de producción.
- Despliegues por rama o por PR (solo se despliega la rama principal).
- Copias de seguridad de la base de datos.
- Protección de acceso (login, Cloudflare Access).
- Avisos o monitorización fuera del repositorio.

## Criterios de finalización
- Un cambio en la rama principal aparece en la URL y la pantalla muestra API y base de datos disponibles.
- Un cambio con un test en rojo queda marcado como fallido en el repositorio y la URL sigue mostrando la versión anterior.
- Un cambio cuya API no arranca vuelve a la versión anterior en 60 s (RF-10).
- Tras reiniciar el servidor, la URL vuelve sola con los datos intactos.
- Home Assistant sigue accesible igual que antes durante y después de cada prueba.
- El hostname no aparece en el repositorio ni en los registros de despliegue (RF-17).

## Dudas abiertas
- Aplazada: `docs/constitution.md` sigue sin estar en el repo y parece de otro proyecto («habits-cli»).
