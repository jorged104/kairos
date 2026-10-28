# Spec 003 — Usuarios: creación y gestión de usuarios

**Estado:** Aprobada (2026-10-22). Dudas resueltas el 2026-10-22; ver «Decisiones de la revisión».

> **Nota de numeración.** El árbol de ejemplo de `AGENTS.md` reserva `001-auth`,
> `002-work-hierarchy` y `003-gamification`, pero en disco 001 y 002 se usaron para la
> infraestructura base y el entorno de previsualización. Esta spec toma el 003, como la rama
> `feat/003-usuarios`. La jerarquía de trabajo y la gamificación tomarán los siguientes números
> libres; esta spec se refiere a ellas como «spec de jerarquía de trabajo» y «spec de gamificación».

## Contexto y objetivo
Kairos lo usa un equipo interno de 20–50 personas organizado en equipos (*teams*),
que a su vez pueden agruparse en equipos de equipos (por ejemplo, las áreas BI, Innovación,
Desarrollo de Software y Ciencia de Datos, cada una con sus subequipos). Toda la jerarquía de
trabajo y la gamificación necesitan saber **quién** es cada persona, **qué puede
hacer** y **a qué equipo pertenece**. Esta spec define la autenticación (local, por
un proveedor de identidad OIDC o ambas), el ciclo de vida de los usuarios, sus
atributos configurables sin cambios de código, los roles, los permisos y los
equipos. La jerarquía de trabajo y la gamificación solo aparecen donde
dependen de lo que aquí se define.

## Usuarios / actores
- **Administrador** (`admin`): gestiona todos los usuarios, equipos, atributos y homologaciones.
- **Líder** (`lead`): gestiona a los miembros de los equipos que lidera.
- **Miembro** (`member`): trabaja en tareas; gestiona solo su propia cuenta.
- **Observador** (`viewer`): solo lectura.
- **Visitante sin sesión**: puede iniciar sesión o, si está habilitado, registrarse.
- **Proveedor de identidad (IdP)** OIDC genérico; Keycloak es la referencia en desarrollo.
- **Herramientas de orquestación** que consultan la salud de la API (spec 001).

## Decisiones de diseño acordadas
La skill de specs pide dejar los detalles técnicos para el plan. Estas decisiones las
tomó el responsable del producto antes de redactar la spec, así que se recogen aquí
como restricciones de entrada para el plan y **no se reabren**:

1. **Modo configurable** `AUTH_MODE=local|oidc|both`. OIDC genérico: nada específico de Keycloak en el código.
2. **Usuario separado de identidad:** `users` (id `uuid` interno, el único que referencia el resto del sistema), `user_identities` (issuer + subject, únicos en conjunto) y `local_credentials`. Ni el `sub` ni el email se usan nunca como clave foránea.
3. **Backend como cliente OIDC confidencial** (patrón BFF): Authorization Code + PKCE, con `state` y `nonce`, y validación del `id_token` con la JWKS del IdP.
4. **Alta JIT** en el primer login OIDC. La vinculación con un usuario local existente por email solo ocurre si `email_verified=true` y la configuración la habilita.
5. **Sesión propia:** JWT emitido por Kairos en una cookie `httpOnly; Secure; SameSite=Lax`. El JWT solo lleva identidad (`sub` = `users.id`, `exp`). El middleware lee los roles y permisos de BD, con una caché corta (1–2 min). El WebSocket se autentica con la misma cookie.
6. **Homologación de roles y grupos del IdP:**
   - El IdP expone `roles` (roles del cliente `kairos`) y `groups` (membresía de grupos con la ruta completa) en el `id_token`.
   - Tabla `idp_mappings` (issuer, claim, external_value, target_type `role|team`, target), con comodín `/*` para subgrupos.
   - En cada login se reemplaza solo lo asignado con `source='idp'`. Las asignaciones `source='manual'` se respetan.
   - Si no hay rol mapeado, se aplica `DEFAULT_ROLE` (configurable) o se rechaza el login, según la configuración.
   - `OIDC_ROLE_SOURCE=idp|local` decide si los roles vienen del IdP o solo se gestionan en Kairos.
7. **Roles locales** en orden de prioridad: `admin` > `lead` > `member` > `viewer`. Los **permisos** se definen en código y se asignan a los roles en un único lugar. Los endpoints comprueban permisos, nunca roles.
8. **Equipos** (`teams`, con `external_ref` opcional a la ruta del grupo en el IdP). Un equipo puede tener un equipo padre (equipo de equipos); no existe un concepto aparte de «coordinación». Alimentan los leaderboards por equipo.
9. **Bootstrap:** `BOOTSTRAP_ADMIN_EMAIL` convierte en admin al primer usuario con ese email.
10. **Logout:** borra la sesión propia y redirige al `end_session_endpoint` del IdP. `users.is_active=false` bloquea el acceso aunque el IdP siga autenticando.
11. **Desarrollo:** Keycloak en `docker-compose.yml` bajo `profile: sso`, con el realm importado de `realm-export.json`. Sin ese profile, la app funciona en modo local.

## Historias de usuario
- H1: Como administrador quiero elegir si la autenticación es local, por IdP o ambas para adaptar Kairos a la infraestructura de la organización.
- H2: Como persona del equipo quiero iniciar sesión con usuario y contraseña de Kairos para usarlo sin depender de un IdP.
- H3: Como persona del equipo quiero iniciar sesión con la cuenta corporativa (SSO) para no gestionar otra contraseña.
- H4: Como persona nueva quiero que mi cuenta se cree sola en el primer login SSO para empezar a trabajar sin esperar a un administrador.
- H5: Como visitante quiero registrarme con email y contraseña, si la organización lo permite, para tener cuenta sin intervención de un administrador.
- H6: Como administrador quiero dar de alta usuarios a mano para preparar sus cuentas antes de que entren.
- H7: Como administrador quiero que los roles y equipos del IdP se traduzcan automáticamente a Kairos para no mantenerlos en dos sitios.
- H8: Como administrador quiero asignar roles y equipos a mano sin que el siguiente login los borre para cubrir excepciones que el IdP no refleja.
- H9: Como administrador quiero definir atributos de usuario (texto, número, fecha, booleano, selección) sin cambiar código para registrar datos como la fecha de ingreso o de salida.
- H10: Como administrador quiero retirar un atributo sin perder los valores ya capturados para conservar el histórico.
- H11: Como administrador o líder quiero editar, desactivar y reactivar usuarios para reflejar altas, cambios y salidas del equipo.
- H12: Como administrador quiero dar de baja a un usuario sin romper su histórico de trabajo y puntos.
- H13: Como administrador o líder quiero listar y buscar usuarios con filtros para encontrar rápido a una persona o a un grupo.
- H14: Como líder quiero gestionar solo a las personas de mi equipo y sus subequipos para no interferir con otros equipos.
- H15: Como persona del equipo quiero cerrar sesión en Kairos y en el IdP para no dejar la sesión abierta en un equipo compartido.
- H16: Como administrador quiero que un usuario desactivado pierda el acceso aunque el IdP le siga autenticando.
- H17: Como responsable de seguridad quiero un registro de los cambios de rol, equipo y estado para saber quién cambió qué y cuándo.
- H18: Como persona que instala Kairos quiero que el primer usuario con un email concreto sea administrador para no quedarme sin acceso.
- H19: Como desarrollador quiero levantar un IdP de prueba con un solo comando para probar SSO en local.

## Requisitos funcionales (criterios de aceptación en EARS)

### Modo de autenticación y configuración
- RF-1: EL SISTEMA leerá el modo de autenticación de la configuración `AUTH_MODE`, con los valores `local`, `oidc` o `both`.
- RF-2: SI `AUTH_MODE` falta o tiene un valor no válido, ENTONCES EL SISTEMA no arrancará la API, mostrará un mensaje de error y terminará con código de salida distinto de 0.
- RF-3: SI `AUTH_MODE` es `oidc` o `both` y falta alguno de los datos obligatorios del IdP (issuer, client id, client secret o URL de retorno), ENTONCES EL SISTEMA no arrancará la API y mostrará qué dato falta.
- RF-4: MIENTRAS `AUTH_MODE` sea `oidc`, EL SISTEMA rechazará el login y el registro con contraseña local.
- RF-5: MIENTRAS `AUTH_MODE` sea `local`, EL SISTEMA rechazará cualquier inicio de login OIDC.
- RF-6: CUANDO un visitante sin sesión abra la pantalla de login, EL SISTEMA mostrará solo los métodos de entrada habilitados por `AUTH_MODE`.

### Login local
- RF-7: DONDE el login local esté habilitado, CUANDO un visitante envíe un email y una contraseña correctos de un usuario activo, EL SISTEMA abrirá una sesión.
- RF-8: SI el email no existe o la contraseña no coincide, ENTONCES EL SISTEMA rechazará el login con el mismo código de error en ambos casos.
- RF-9: SI un mismo email o una misma IP acumulan 5 intentos de login fallidos en 15 minutos, ENTONCES EL SISTEMA rechazará los siguientes intentos desde ese email o esa IP durante 15 minutos, aunque la contraseña sea correcta.
- RF-10: EL SISTEMA comparará los emails sin distinguir mayúsculas ni espacios exteriores.

### Registro local
- RF-11: DONDE el login local esté habilitado y el registro abierto esté activado por configuración, CUANDO un visitante se registre con nombre, email y contraseña válidos, EL SISTEMA creará un usuario activo con el rol por defecto y sin equipo.
- RF-12: MIENTRAS el registro abierto esté desactivado, EL SISTEMA rechazará los registros y no mostrará la opción en la pantalla de login.
- RF-13: SI el email del registro ya pertenece a otro usuario, ENTONCES EL SISTEMA no creará un duplicado y responderá con un código de conflicto.
- RF-14: SI la contraseña tiene menos de 12 caracteres, ENTONCES EL SISTEMA rechazará el registro indicando el motivo.

### Login OIDC
- RF-15: DONDE el login OIDC esté habilitado, CUANDO un visitante elija entrar con SSO, EL SISTEMA lo redirigirá al IdP con una petición de autorización protegida con PKCE, `state` y `nonce`.
- RF-16: CUANDO el IdP devuelva al visitante con un código de autorización válido, EL SISTEMA obtendrá y validará el `id_token` (firma, emisor, audiencia, expiración y `nonce`) antes de abrir sesión.
- RF-17: SI el `state` devuelto no coincide con el emitido o ha caducado (más de 10 minutos), ENTONCES EL SISTEMA rechazará el login sin contactar con el IdP.
- RF-18: SI la validación del `id_token` falla, ENTONCES EL SISTEMA rechazará el login y registrará el motivo en el log sin incluir el token.
- RF-19: SI el IdP no responde en 5 segundos durante el login, ENTONCES EL SISTEMA rechazará el login con un código de error de IdP no disponible.
- RF-20: EL SISTEMA identificará a la persona que llega por OIDC por la pareja emisor + `sub`, nunca solo por el email.
- RF-21: CUANDO llegue por OIDC una pareja emisor + `sub` ya vinculada a un usuario activo, EL SISTEMA abrirá sesión para ese usuario.
- RF-22: CUANDO un usuario existente inicie sesión por OIDC, EL SISTEMA actualizará su nombre y su email con los del `id_token`, aunque se hubieran cambiado a mano en Kairos.
- RF-22a: SI el email del `id_token` pertenece a otro usuario, ENTONCES EL SISTEMA conservará el email actual, abrirá la sesión y registrará un aviso en el log.

### Alta JIT y vinculación
- RF-23: CUANDO llegue por OIDC una pareja emisor + `sub` desconocida y ningún usuario tenga su email, EL SISTEMA creará un usuario activo y lo vinculará a esa identidad.
- RF-24: DONDE la vinculación por email esté habilitada, CUANDO llegue por OIDC una identidad desconocida con `email_verified=true` y un usuario existente tenga ese email, EL SISTEMA vinculará la identidad a ese usuario en lugar de crear otro.
- RF-25: SI llega por OIDC una identidad desconocida cuyo email ya tiene un usuario existente y el `email_verified` no es `true` o la vinculación está deshabilitada, ENTONCES EL SISTEMA rechazará el login con un código de conflicto de cuenta y no creará ni vinculará ningún usuario.
- RF-26: EL SISTEMA vinculará una identidad OIDC a un único usuario.
- RF-27: SI el `id_token` de una identidad desconocida no trae email, ENTONCES EL SISTEMA rechazará el alta JIT con un código de email obligatorio. Todo usuario tiene email.

### Homologación de roles y equipos del IdP
- RF-28: CUANDO un usuario inicie sesión por OIDC, EL SISTEMA calculará sus roles y equipos de origen IdP a partir de los claims `roles` y `groups` del `id_token` y de las homologaciones configuradas para ese emisor.
- RF-29: CUANDO una homologación de un grupo termine en `/*`, EL SISTEMA la aplicará a todos los subgrupos de esa ruta, a cualquier profundidad, pero no al grupo padre.
- RF-30: CUANDO termine el cálculo de RF-28, EL SISTEMA reemplazará las asignaciones de origen IdP del usuario por las calculadas.
- RF-31: EL SISTEMA no modificará ni eliminará las asignaciones de origen manual al sincronizar en el login.
- RF-32: SI una asignación calculada desde el IdP coincide con una asignación manual existente, ENTONCES EL SISTEMA conservará solo la manual.
- RF-33: MIENTRAS `OIDC_ROLE_SOURCE` sea `local`, EL SISTEMA ignorará el claim `roles` y no creará ni eliminará roles de origen IdP.
- RF-34: EL SISTEMA sincronizará los equipos desde el claim `groups` sea cual sea el valor de `OIDC_ROLE_SOURCE`.
- RF-35: SI tras la sincronización el usuario no tiene ningún rol y la configuración tiene un `DEFAULT_ROLE`, ENTONCES EL SISTEMA le asignará ese rol con origen IdP.
- RF-36: SI tras la sincronización el usuario no tiene ningún rol y la configuración indica rechazo, ENTONCES EL SISTEMA rechazará el login con un código de error de usuario sin rol.
- RF-37: SI un valor del claim no tiene homologación, ENTONCES EL SISTEMA lo ignorará sin rechazar el login.
- RF-38: SI la sincronización fuese a quitar el rol `admin` al último administrador activo, ENTONCES EL SISTEMA conservará ese rol y registrará un aviso en la auditoría.
- RF-39: CUANDO un administrador cree, edite o elimine una homologación, EL SISTEMA la aplicará a partir del siguiente login de cada usuario afectado.

### Sesión
- RF-40: CUANDO se abra una sesión por cualquier método, EL SISTEMA entregará al navegador una cookie de sesión `httpOnly`, `Secure` y `SameSite=Lax` que solo identifica al usuario y su caducidad.
- RF-41: EL SISTEMA hará caducar la sesión a las 8 horas de abrirse, sin renovarla con la actividad.
- RF-42: SI una petición a un endpoint protegido llega sin sesión, con la sesión caducada o con una sesión manipulada, ENTONCES EL SISTEMA responderá con un código de no autenticado.
- RF-43: EL SISTEMA obtendrá los roles, permisos y estado del usuario de la base de datos, no de la cookie.
- RF-44: EL SISTEMA aplicará los cambios de rol, permiso, equipo o estado a las peticiones de ese usuario en 120 segundos como máximo.
- RF-45: SI el usuario de una sesión válida está desactivado o dado de baja, ENTONCES EL SISTEMA rechazará sus peticiones con un código de cuenta inactiva.
- RF-46: SI un usuario desactivado o dado de baja intenta iniciar sesión por cualquier método, ENTONCES EL SISTEMA rechazará el login con un código de cuenta inactiva.
- RF-47: EL SISTEMA autenticará las conexiones de tiempo real con la misma cookie de sesión.
- RF-48: SI una conexión de tiempo real se abre sin sesión válida, ENTONCES EL SISTEMA la rechazará.
- RF-49: CUANDO un usuario con una conexión de tiempo real abierta quede desactivado o dado de baja, EL SISTEMA cerrará esa conexión en 120 segundos como máximo.
- RF-50: CUANDO un usuario con sesión pida sus propios datos, EL SISTEMA devolverá su perfil, sus roles, sus equipos y sus permisos efectivos.

### Logout
- RF-51: CUANDO un usuario cierre sesión, EL SISTEMA eliminará la cookie de sesión.
- RF-52: CUANDO un usuario que entró por OIDC cierre sesión, EL SISTEMA lo redirigirá al endpoint de cierre de sesión del IdP y después de vuelta a la pantalla de login de Kairos.
- RF-53: SI el IdP no publica un endpoint de cierre de sesión, ENTONCES EL SISTEMA cerrará solo la sesión de Kairos y llevará al usuario a la pantalla de login.

### Bootstrap del administrador
- RF-54: CUANDO se cree el primer usuario cuyo email coincida con `BOOTSTRAP_ADMIN_EMAIL`, por cualquier método de alta, EL SISTEMA le asignará el rol `admin` con origen manual.
- RF-55: CUANDO un usuario existente con el email de `BOOTSTRAP_ADMIN_EMAIL` inicie sesión y no exista ningún administrador activo, EL SISTEMA le asignará el rol `admin` con origen manual.
- RF-56: SI ya existe un administrador activo, ENTONCES EL SISTEMA no aplicará `BOOTSTRAP_ADMIN_EMAIL` a nuevos usuarios.

### Roles y permisos
- RF-57: EL SISTEMA tendrá exactamente cuatro roles, en orden de prioridad descendente: `admin`, `lead`, `member` y `viewer`.
- RF-58: EL SISTEMA concederá a cada usuario la unión de los permisos de todos sus roles.
- RF-59: EL SISTEMA definirá la correspondencia entre roles y permisos en un único lugar, según la matriz de permisos de esta spec.
- RF-60: EL SISTEMA decidirá el acceso a cada operación protegida comprobando un permiso, nunca el nombre de un rol.
- RF-61: SI un usuario autenticado intenta una operación sin el permiso necesario o fuera de su ámbito, ENTONCES EL SISTEMA la rechazará con un código de prohibido sin modificar nada.
- RF-62: EL SISTEMA tendrá tests automáticos que comprueben, para cada operación protegida, que cada rol obtiene el resultado que indica la matriz.

#### Matriz de permisos
«Equipo» = usuarios que pertenecen a algún equipo que el actor lidera o a cualquiera de sus
subequipos, a cualquier profundidad. Un `lead`
solo actúa sobre usuarios de su equipo cuyo rol de mayor prioridad sea `member` o `viewer`.

| Permiso | admin | lead | member | viewer |
|---|---|---|---|---|
| Ver el directorio básico (nombre, equipos, estado) | todos | todos | todos | todos |
| Ver el perfil completo (email, atributos, roles) | todos | equipo | solo el suyo | solo el suyo |
| Crear usuarios | sí | en su equipo | no | no |
| Editar datos y atributos de un usuario | todos | equipo | no | no |
| Desactivar / reactivar usuarios | todos | equipo | no | no |
| Dar de baja usuarios y revertir la baja | sí | no | no | no |
| Asignar o quitar roles | todos los roles | solo `member` y `viewer`, en su equipo | no | no |
| Asignar o quitar miembros de equipos | todos los equipos | los que lidera y sus subequipos | no | no |
| Gestionar equipos (crear, renombrar, mover de equipo padre, archivar, nombrar líderes) | sí | no | no | no |
| Definir atributos dinámicos | sí | no | no | no |
| Gestionar homologaciones del IdP | sí | no | no | no |
| Establecer la contraseña local de otro usuario | sí | no | no | no |
| Consultar la auditoría | sí | no | no | no |
| Cambiar su propia contraseña local | sí | sí | sí | sí |

- RF-63: SI un `lead` intenta actuar sobre un usuario fuera de su equipo o con rol `lead` o `admin`, ENTONCES EL SISTEMA rechazará la operación con un código de prohibido.
- RF-64: SI un `lead` intenta asignar el rol `lead` o `admin`, ENTONCES EL SISTEMA rechazará la operación con un código de prohibido.
- RF-65: SI un usuario intenta cambiar sus propios roles, ENTONCES EL SISTEMA rechazará la operación con un código de prohibido, salvo que tenga rol `admin`.

### Alta manual
- RF-66: CUANDO un actor con permiso cree un usuario con nombre y email válidos, EL SISTEMA lo creará activo.
- RF-67: CUANDO un `lead` cree un usuario, EL SISTEMA lo añadirá, con origen manual, a uno de los equipos que ese `lead` lidera o a uno de sus subequipos, elegido en la petición.
- RF-68: CUANDO se cree un usuario manualmente sin indicar rol, EL SISTEMA le asignará el rol por defecto con origen manual.
- RF-69: DONDE el login local esté habilitado, CUANDO un administrador cree un usuario con contraseña inicial, EL SISTEMA guardará esa credencial local.
- RF-70: DONDE el login OIDC esté habilitado, CUANDO un usuario creado a mano entre por primera vez por OIDC con su mismo email, EL SISTEMA aplicará las reglas de vinculación de RF-24 y RF-25.
- RF-71: SI el email del alta manual ya pertenece a otro usuario, incluido uno desactivado o dado de baja, ENTONCES EL SISTEMA no creará un duplicado y responderá con un código de conflicto.

### Edición, desactivación y baja
- RF-72: CUANDO un actor con permiso edite el nombre, el email o los atributos de un usuario, EL SISTEMA guardará los cambios validados.
- RF-73: SI la edición deja el email igual al de otro usuario, ENTONCES EL SISTEMA rechazará el cambio con un código de conflicto.
- RF-74: CUANDO un actor con permiso desactive un usuario, EL SISTEMA lo marcará como inactivo sin borrar ninguno de sus datos, roles, equipos ni identidades.
- RF-75: CUANDO un actor con permiso reactive un usuario desactivado, EL SISTEMA le devolverá el acceso con los mismos roles y equipos que tenía.
- RF-76: CUANDO un administrador dé de baja a un usuario, EL SISTEMA lo marcará como dado de baja, le quitará el acceso y lo ocultará del directorio por defecto, conservando su identificador y su histórico.
- RF-77: EL SISTEMA no borrará físicamente ningún usuario.
- RF-78: CUANDO un administrador revierta la baja de un usuario, EL SISTEMA lo dejará desactivado, con los mismos datos, roles, equipos e identidades que tenía, y lo registrará en la auditoría.
- RF-78a: SI alguien sin permiso de dar de baja intenta reactivar o revertir la baja de un usuario dado de baja, ENTONCES EL SISTEMA rechazará la operación con un código de prohibido.
- RF-79: SI la operación dejaría el sistema sin ningún administrador activo (quitar el rol `admin`, desactivar o dar de baja al último), ENTONCES EL SISTEMA la rechazará con un código de último administrador.
- RF-80: CUANDO un usuario cambie su propia contraseña local aportando la actual, EL SISTEMA reemplazará la credencial.
- RF-81: CUANDO un administrador establezca una nueva contraseña local para otro usuario, EL SISTEMA reemplazará la credencial y la registrará en la auditoría sin el valor de la contraseña.
- RF-82: SI la contraseña actual aportada no coincide, ENTONCES EL SISTEMA rechazará el cambio de contraseña propia.

### Asignación de roles y equipos
- RF-83: CUANDO un actor con permiso asigne a mano un rol o un equipo, EL SISTEMA lo guardará con origen manual.
- RF-84: CUANDO un actor con permiso quite a mano una asignación de origen manual, EL SISTEMA la eliminará.
- RF-85: SI un actor intenta quitar a mano un equipo de origen IdP, o un rol de origen IdP mientras `OIDC_ROLE_SOURCE` sea `idp`, ENTONCES EL SISTEMA la rechazará e indicará que se gestiona en el IdP.
- RF-86: EL SISTEMA mostrará el origen (manual o IdP) de cada rol y cada equipo asignado a un usuario.
- RF-87: EL SISTEMA permitirá que un usuario pertenezca a varios equipos, incluso de ramas distintas de la jerarquía.

### Equipos
- RF-88: CUANDO un administrador cree un equipo con un nombre no usado, EL SISTEMA lo guardará, con su referencia opcional al grupo del IdP.
- RF-89: SI el nombre del equipo ya existe (comparación sin mayúsculas ni espacios exteriores), ENTONCES EL SISTEMA no creará un duplicado y responderá con un código de conflicto.
- RF-90: CUANDO un administrador cree o edite un equipo indicando un equipo padre, EL SISTEMA lo guardará como subequipo de ese padre.
- RF-90a: SI el equipo padre indicado es el propio equipo o uno de sus descendientes, ENTONCES EL SISTEMA rechazará el cambio con un código de ciclo de equipos.
- RF-90b: SI el equipo padre indicado está archivado, ENTONCES EL SISTEMA rechazará el cambio.
- RF-90c: SI un administrador intenta archivar un equipo con subequipos sin archivar, ENTONCES EL SISTEMA rechazará la operación.
- RF-90d: EL SISTEMA considerará que pertenecer a un subequipo implica pertenecer, de forma indirecta, a todos sus equipos antecesores, a efectos del ámbito de los líderes y de los filtros del directorio.
- RF-91: CUANDO un administrador nombre líder de un equipo a un usuario, EL SISTEMA le dará ámbito de `lead` sobre ese equipo y sus subequipos, siempre que el usuario tenga el rol `lead`.
- RF-92: CUANDO un administrador archive un equipo, EL SISTEMA dejará de mostrarlo para nuevas asignaciones y conservará sus asignaciones históricas.
- RF-93: SI una homologación apunta a un equipo archivado, ENTONCES EL SISTEMA no asignará ese equipo en el login y registrará un aviso.

### Atributos dinámicos
- RF-94: CUANDO un administrador defina un atributo con nombre, tipo y obligatoriedad, EL SISTEMA lo añadirá a todos los usuarios sin cambios de código.
- RF-95: EL SISTEMA admitirá atributos de tipo texto, número, fecha, booleano y selección.
- RF-96: CUANDO un administrador defina un atributo de tipo selección, EL SISTEMA exigirá al menos una opción.
- RF-97: SI se guarda un valor que no corresponde al tipo del atributo (o una opción que no existe en una selección), ENTONCES EL SISTEMA rechazará el cambio indicando el atributo inválido.
- RF-98: SI se crea o edita un usuario sin valor para un atributo obligatorio activo, ENTONCES EL SISTEMA rechazará la operación indicando el atributo que falta.
- RF-99: CUANDO un atributo pase a ser obligatorio, EL SISTEMA no invalidará a los usuarios que ya existían sin valor y les exigirá el valor en su siguiente edición.
- RF-100: EL SISTEMA permitirá que un usuario tenga valor en un atributo no obligatorio y otro usuario no lo tenga.
- RF-101: SI el nombre de un atributo ya existe entre los atributos activos, ENTONCES EL SISTEMA no creará un duplicado y responderá con un código de conflicto.
- RF-102: SI un administrador intenta cambiar el tipo de un atributo que ya tiene valores, ENTONCES EL SISTEMA rechazará el cambio.
- RF-103: CUANDO un administrador elimine un atributo, EL SISTEMA lo marcará como retirado y conservará todos los valores ya capturados.
- RF-104: MIENTRAS un atributo esté retirado, EL SISTEMA no lo pedirá, no lo validará y no permitirá editar sus valores.
- RF-105: MIENTRAS un atributo esté retirado, EL SISTEMA mostrará sus valores capturados solo a los administradores, como histórico de solo lectura.
- RF-106: CUANDO un administrador restaure un atributo retirado, EL SISTEMA volverá a mostrar y permitir editar sus valores conservados.
- RF-107: CUANDO un administrador retire una opción de un atributo de selección, EL SISTEMA conservará esa opción en los usuarios que ya la tenían y no permitirá elegirla de nuevo.
- RF-108: CUANDO el SSO cree un usuario por JIT, EL SISTEMA lo creará aunque le falten atributos obligatorios.

### Listado y búsqueda
- RF-109: CUANDO un usuario con sesión pida el directorio, EL SISTEMA devolverá una página de usuarios ordenada por nombre, con 25 elementos por defecto y 100 como máximo.
- RF-110: CUANDO la petición incluya un texto de búsqueda, EL SISTEMA devolverá los usuarios cuyo nombre o email lo contengan, sin distinguir mayúsculas ni acentos.
- RF-111: CUANDO la petición incluya filtros por equipo (incluidos sus subequipos), rol o estado (activo, inactivo, dado de baja), EL SISTEMA devolverá solo los usuarios que cumplan todos los filtros a la vez.
- RF-112: CUANDO la petición filtre por un atributo dinámico, EL SISTEMA aplicará igualdad a los atributos de texto, booleano y selección, y un rango a los de número y fecha.
- RF-113: MIENTRAS no se filtre por estado, EL SISTEMA excluirá del directorio a los usuarios dados de baja.
- RF-114: EL SISTEMA devolverá junto con cada página el número total de resultados.
- RF-115: EL SISTEMA mostrará a cada actor solo los campos que la matriz de permisos le deja ver, y solo permitirá filtrar y buscar por esos campos.
- RF-116: SI un filtro hace referencia a un atributo, equipo o rol inexistente, ENTONCES EL SISTEMA responderá con un código de petición inválida.

### Auditoría
- RF-117: CUANDO cambien los roles, equipos o estado de un usuario, o su contraseña la establezca otra persona, EL SISTEMA registrará quién lo hizo (o «IdP» si fue la sincronización), a quién, qué cambió, el valor anterior y el nuevo, y cuándo.
- RF-118: CUANDO se cree, modifique, retire o restaure un atributo, un equipo o una homologación, EL SISTEMA registrará quién lo hizo, qué cambió y cuándo.
- RF-119: EL SISTEMA no permitirá modificar ni borrar entradas de la auditoría.
- RF-120: CUANDO un administrador consulte la auditoría, EL SISTEMA la devolverá paginada y filtrable por usuario afectado, actor y rango de fechas.

### Integración con la pantalla de estado (spec 001)
- RF-121: SI un visitante sin sesión abre la pantalla de estado, ENTONCES EL SISTEMA lo redirigirá a la pantalla de login.
- RF-122: CUANDO el visitante inicie sesión tras la redirección de RF-121, EL SISTEMA lo devolverá a la pantalla de estado.
- RF-123: EL SISTEMA mantendrá los endpoints de vida y disponibilidad de la API accesibles sin sesión.

### Interfaz
- RF-124: EL SISTEMA obtendrá todos los textos visibles de las pantallas de esta spec del catálogo de traducciones, con español como idioma por defecto.
- RF-125: CUANDO la API devuelva un error, EL SISTEMA mostrará al usuario el texto del catálogo asociado a su código de error, no el mensaje técnico de la API.
- RF-128: EL SISTEMA aplicará al directorio de usuarios y a la pantalla de equipos la edición en tabla de la spec 004 (RF-139 a RF-143): nombre y email del usuario; nombre, equipo padre y grupo del IdP del equipo, y alta rápida de usuarios (nombre, email y equipo) y de equipos (nombre y equipo padre) desde la última fila. Cada fila del directorio indicará si quien consulta puede editarla. El alta completa (contraseña inicial, rol y atributos) sigue en su pantalla.

### Entorno de desarrollo con IdP
- RF-126: CUANDO el desarrollador arranque el entorno con el profile de SSO, EL SISTEMA levantará un IdP de prueba con un realm importado que incluye el cliente de Kairos, usuarios de ejemplo, roles y grupos de las cuatro áreas con algún subgrupo.
- RF-127: CUANDO el desarrollador arranque el entorno sin el profile de SSO, EL SISTEMA no levantará el IdP y la aplicación funcionará en modo local.

## Dependencias con otras specs (contrato, no se implementa aquí)
- **Jerarquía de trabajo:** responsables, creadores y participantes de objetivos, proyectos, hitos, actividades y subactividades se referencian por el identificador interno del usuario. Esta spec garantiza que ese identificador nunca cambia ni se borra (RF-77). La spec de jerarquía de trabajo decide si se puede asignar trabajo nuevo a usuarios inactivos o dados de baja y qué permisos de trabajo tiene cada rol. Esos permisos se añadirán a la misma matriz única (RF-59).
- **Gamificación:** XP, monedas y nivel no se guardan como datos editables del usuario. Salen del ledger de puntos, que referencia al usuario por su identificador interno. Esta spec no muestra ni modifica XP ni monedas. Ofrece a la gamificación los equipos de cada usuario y la jerarquía de equipos (para los leaderboards) y su estado. La spec de gamificación decide si los usuarios inactivos o dados de baja aparecen en los leaderboards.
- **Infraestructura base (spec 001):** la pantalla de estado pasa a requerir sesión (RF-121, RF-122). Los endpoints de salud siguen siendo públicos (RF-123).

## Requisitos no funcionales
- **Contraseñas:** se guardan solo con un hash adaptativo con sal, de coste configurable. Mínimo 12 caracteres y máximo 128. Nunca aparecen en logs, respuestas de la API ni auditoría.
- **Secretos:** la clave de firma de sesión y el secreto del cliente OIDC vienen solo del entorno. `.env.example` solo lleva valores de desarrollo marcados como tales. Ningún secreto se registra en los logs. Los tokens del IdP (access, refresh, id) no se persisten después de completar el login. El cierre de sesión en el IdP se pide con el client id, sin `id_token_hint`.
- **Cookies:** la cookie de sesión no es accesible desde JavaScript. Fuera de desarrollo solo viaja por HTTPS.
- **CSRF:** las operaciones que modifican datos rechazan las peticiones cuyo origen no sea el del frontend configurado.
- **Rate limiting:** el que indica RF-9. Las respuestas rechazadas no revelan si el email existe.
- **Enumeración:** login, registro y alta no permiten saber si un email existe a quien no tenga permiso para verlo, salvo el código de conflicto del registro (RF-13), riesgo aceptado.
- **Auditoría:** RF-117 a RF-119. Se conserva indefinidamente en esta iteración (sin purga automática).
- **Tamaño:** dimensionado para 500 usuarios y 50 atributos dinámicos (10 veces el tamaño esperado). Con ese volumen, el directorio responde en menos de 500 ms en el entorno local.
- **Ventana de permisos:** los cambios de autorización tardan como máximo 120 s en aplicarse (RF-44).
- **Idioma:** español. Las pantallas nuevas cumplen la regla de catálogo de traducciones de la spec 001 (RF-29, RF-30 de 001).

## Casos límite
- **Login OIDC sin email verificado:** si no existe ningún usuario con ese email, se crea por JIT (RF-23). Si existe, se rechaza con conflicto de cuenta y no se vincula (RF-25).
- **Usuario local con el mismo email que llega por OIDC:** con vinculación habilitada y `email_verified=true`, la identidad se vincula al usuario local, que conserva su contraseña, roles manuales y atributos (RF-24). En cualquier otro caso se rechaza (RF-25).
- **JIT con el email de un usuario local inactivo:** si se cumplen las condiciones de vinculación, la identidad se vincula pero el login se rechaza por cuenta inactiva (RF-24 + RF-46). Nunca se crea un segundo usuario con ese email (RF-71).
- **IdP caído:** el login OIDC falla en 5 s con el código de IdP no disponible (RF-19). En modo `both` el login local sigue funcionando. Las sesiones de Kairos ya abiertas no se ven afectadas, porque son propias. El logout cierra la sesión de Kairos aunque la redirección al IdP falle.
- **Caché de permisos desactualizada:** durante un máximo de 120 s tras quitar un permiso o desactivar a alguien, sus peticiones aún pueden aceptarse (RF-44). Es un riesgo aceptado. Pasado ese tiempo se rechazan (RF-45) y su tiempo real se cierra (RF-49).
- **Atributo eliminado con valores capturados:** queda retirado, sus valores se conservan y solo los administradores los ven en solo lectura (RF-103 a RF-105). Restaurarlo los recupera (RF-106).
- **Opción de selección retirada:** los usuarios que la tenían la conservan; nadie puede elegirla de nuevo (RF-107).
- **El administrador se quita a sí mismo y es el último:** se rechaza (RF-79). Lo mismo si es la sincronización del IdP la que se lo quitaría (RF-38).
- **Rol quitado en el IdP pero también asignado a mano:** el usuario lo conserva, porque lo manual no se toca (RF-31, RF-32).
- **Intento de quitar a mano un rol que viene del IdP:** se rechaza, porque volvería en el siguiente login (RF-85).
- **Usuario sin ningún rol homologado:** recibe `DEFAULT_ROLE` o se rechaza su login, según la configuración (RF-35, RF-36).
- **Cambio de `OIDC_ROLE_SOURCE` de `idp` a `local`:** los roles de origen IdP ya asignados se conservan; mientras dure ese modo, un administrador puede quitarlos a mano (excepción a RF-85).
- **`state` reutilizado o caducado:** el login se rechaza (RF-17).
- **Dos logins OIDC simultáneos de la misma identidad nueva:** se crea un único usuario (RF-26).
- **Email con distinta capitalización o espacios:** se trata como el mismo email (RF-10).
- **Un `lead` que intenta desactivar a otro `lead` de su equipo:** se rechaza (RF-63).
- **Directorio con filtro por atributo retirado:** solo lo pueden usar los administradores (RF-105, RF-115).
- **Usuario JIT sin atributos obligatorios:** se crea igual (RF-108) y se le exigen en su primera edición (RF-98, RF-99).

## Fuera de alcance
- **Aplicación móvil.**
- **Integración específica con Google, GitHub u otros IdP sociales:** solo OIDC genérico. Cualquier IdP compatible debería funcionar, pero solo se prueba con Keycloak.
- **Notificaciones por correo** (invitaciones, avisos de alta o de cambio de rol): Kairos aún no tiene envío de correo.
- **MFA:** se delega en el IdP. Quien la necesite debe usar el modo `oidc`.
- **Recuperación de contraseña por correo:** necesita envío de correo, que está fuera de alcance. Mientras tanto, un administrador establece una nueva contraseña (RF-81).
- **Gestión de usuarios, grupos o roles dentro del IdP desde Kairos:** la homologación es solo de lectura.
- **Sincronización periódica con el IdP fuera del login** (SCIM, sondeo del directorio).
- **Varios emisores OIDC a la vez.** El diseño (issuer + subject) lo permite, pero en esta iteración se configura uno solo.
- **Roles personalizados** distintos de los cuatro fijos.
- **Permisos sobre la jerarquía de trabajo y la gamificación** (specs 002 y de gamificación).
- **Avatares y fotos de perfil.**
- **Importación masiva de usuarios** (CSV).
- **Exportación del directorio.**
- **Revocación inmediata de sesiones concretas** («cerrar sesión en todos los dispositivos»), más allá de desactivar al usuario.
- **Idiomas distintos del español.**

## Criterios de finalización
- RF-1 a RF-120 cubiertos por tests automáticos del backend en verde. Incluyen la matriz de permisos completa (RF-62) y un IdP simulado para RF-15 a RF-39.
- RF-121, RF-122, RF-124, RF-125 y RF-128 cubiertos por tests automáticos del frontend en verde.
- Demo manual desde un clon limpio:
  1. Configurar `AUTH_MODE=local`, `BOOTSTRAP_ADMIN_EMAIL` y el registro abierto. Arrancar el entorno sin profile SSO y aplicar migraciones.
  2. Abrir la pantalla de estado sin sesión y comprobar la redirección al login (RF-121). Comprobar que el endpoint de vida responde sin sesión (RF-123).
  3. Registrarse con el email de bootstrap y comprobar que el usuario es `admin` y vuelve a la pantalla de estado (RF-54, RF-122).
  4. Como admin, definir los atributos «Fecha de ingreso» (fecha, obligatorio) y «Fecha de salida» (fecha, opcional). Crear el equipo «BI» y su subequipo «BI Reportes». Dar de alta a mano un `lead` de BI y dos `member` de «BI Reportes».
  5. Intentar crear un usuario sin «Fecha de ingreso» y ver el error (RF-98). Intentar guardar «abc» como fecha y ver el error (RF-97).
  6. Entrar como el `lead`: editar y desactivar a un `member` de «BI Reportes» (ámbito heredado), e intentar asignar `admin` a otro y ver el rechazo (RF-64).
  7. Entrar como el `member` desactivado y ver el rechazo por cuenta inactiva (RF-46). Reactivarlo como `lead` y comprobar que entra con los mismos roles (RF-75).
  8. Como `member`, intentar crear un usuario y ver el rechazo (RF-61).
  9. Como admin, retirar «Fecha de salida» y comprobar que los valores siguen visibles en solo lectura. Restaurarla y comprobar que vuelven a ser editables (RF-103 a RF-106).
  10. Buscar en el directorio por texto, por equipo «BI» (debe incluir a los de «BI Reportes»), por rol `member` y por rango de «Fecha de ingreso» (RF-110 a RF-112).
  11. Como único admin, intentar quitarse el rol `admin` y ver el rechazo (RF-79). Consultar la auditoría y ver los cambios de los pasos anteriores (RF-117, RF-120).
  12. Fallar 5 veces la contraseña y comprobar que el sexto intento, aunque sea correcto, se rechaza (RF-9).
  13. Parar, configurar `AUTH_MODE=both` con homologaciones de `roles` → `lead` y de `/Areas/BI/*` → equipo BI, y arrancar con el profile SSO.
  14. Entrar por SSO con un usuario de ejemplo del realm y comprobar que se crea por JIT con el rol y el equipo homologados, marcados con origen IdP (RF-23, RF-28 a RF-30, RF-86).
  15. Asignarle a mano otro equipo. Quitarle en el IdP el grupo de BI y volver a entrar: el equipo IdP desaparece y el manual se mantiene (RF-30, RF-31).
  16. Entrar por SSO con un usuario del realm cuyo email coincida con un usuario local, con vinculación habilitada y email verificado, y comprobar que no se crea un duplicado (RF-24).
  17. Desactivar en Kairos a un usuario SSO con sesión abierta y comprobar que, como máximo en 120 s, sus peticiones se rechazan y su conexión de tiempo real se cierra (RF-44, RF-45, RF-49).
  18. Cerrar sesión como usuario SSO y comprobar que también se cierra la sesión del IdP: al volver a entrar con SSO, el IdP pide credenciales (RF-52).
  19. Parar el IdP: el botón de SSO falla con el mensaje de IdP no disponible en ≤ 5 s y el login local sigue funcionando (RF-19).

## Decisiones de la revisión (2026-10-22)
- Numeración: la jerarquía de trabajo y la gamificación toman los siguientes números libres; el árbol de `AGENTS.md` se actualiza.
- RF-22: el IdP manda siempre sobre nombre y email (RF-22, RF-22a).
- RF-27: no se admiten usuarios sin email.
- RF-41: sesión de 8 h fijas, sin renovación.
- RF-78: la baja es reversible por un administrador; el usuario vuelve como desactivado.
- RF-87 / RF-90: desaparece la «coordinación». Los equipos son jerárquicos (un equipo puede tener padre). Un usuario puede estar en varios equipos. El ámbito del `lead` y los filtros por equipo incluyen los subequipos.
- Atributos: solo los editan admin y lead; todos aplican a todos los usuarios. Los atributos retirados solo los ven los administradores (RF-105).
- Cierre de sesión en el IdP con client id, sin `id_token_hint`. Auditoría sin purga. Se acepta el código de conflicto del registro abierto.
- Cambio de `OIDC_ROLE_SOURCE` a `local`: los roles de origen IdP se conservan y pueden quitarse a mano (RF-85).
- Un único emisor OIDC en esta iteración.

## Dudas abiertas
- Aplazada: no existe `docs/constitution.md`. El plan de la spec 001 cita «la constitución», pero el archivo no está en el repositorio. Cuando se cree, contrastar esta spec con ella.
