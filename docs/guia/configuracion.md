# Configuración

Kairos se configura en dos sitios:

1. **Variables de entorno de la API.** Se leen al arrancar. Para cambiarlas hay que reiniciar la API.
2. **La pantalla Sistema** (solo administradores): acceso, SSO, almacenamiento, apariencia, conexiones de IA y correo. Se aplica al momento, sin reiniciar.

Algunas variables (marcadas con **①** abajo) **solo cuentan la primera vez que arranca la API**: con ellas se rellena la pantalla Sistema. A partir de ahí manda lo que se guarde en Sistema, y cambiar la variable ya no tiene efecto.

## Variables de entorno

### Obligatorias

| Variable | Qué es |
|---|---|
| `DATABASE_URL` | Conexión a PostgreSQL: `postgres://usuario:contraseña@host:5432/bd?sslmode=disable`. Los `compose.yml` la arman a partir de `POSTGRES_*` |
| `AUTH_MODE` ① | Cómo se entra: `local` (usuario y contraseña), `oidc` (solo SSO) o `both` |
| `SESSION_SECRET` | Al menos 32 bytes aleatorios (`openssl rand -base64 48`). Firma las sesiones y **cifra los secretos guardados en Sistema** (contraseñas de SMTP, SSO, S3 y la clave del servicio de IA): si cambia, hay que volver a escribirlos |
| `PUBLIC_URL` | La dirección exacta con la que se abre Kairos, sin barra final. Se usa para la protección CSRF, el WebSocket, la vuelta del SSO y los enlaces de los correos |

### Acceso

| Variable | Por defecto | Qué es |
|---|---|---|
| `BOOTSTRAP_ADMIN_EMAIL` | — | Quien se registre o entre con este correo pasa a ser administrador mientras no haya ningún otro administrador activo. Con `AUTH_MODE=oidc` es también la única cuenta que puede entrar con contraseña, como acceso de emergencia si el SSO falla |
| `REGISTRATION_ENABLED` ① | `false` | Permite que cualquiera se cree una cuenta desde la pantalla de login |
| `DEFAULT_ROLE` ① | `member` | Rol de las cuentas nuevas: `admin`, `lead`, `member` o `viewer`. Vacío = rechazar a quien no tenga rol |
| `COOKIE_SECURE` | `true` | La cookie de sesión solo viaja por HTTPS. Pon `false` únicamente en desarrollo sobre `http://localhost` |
| `TRUST_PROXY` | `false` | Toma la IP del cliente de las cabeceras del proxy (para el límite de intentos de login). `true` detrás del nginx de `web` |
| `PASSWORD_HASH_MEMORY_KIB` | `65536` | Memoria del hash Argon2id de las contraseñas |
| `PASSWORD_HASH_TIME` | `3` | Iteraciones del hash Argon2id |

### SSO (OIDC)

Solo se piden con `AUTH_MODE=oidc` o `both`. También puedes arrancar con `local` y configurar el SSO más tarde en **Sistema → SSO**.

| Variable | Por defecto | Qué es |
|---|---|---|
| `OIDC_ISSUER` ① | — | URL del emisor del proveedor de identidad |
| `OIDC_DISCOVERY_URL` ① | el issuer | Cómo ve la API al proveedor, si es distinto del issuer (por ejemplo, dentro de la red de Docker) |
| `OIDC_CLIENT_ID` ① | — | Cliente registrado en el proveedor |
| `OIDC_CLIENT_SECRET` ① | — | Secreto de ese cliente |
| `OIDC_REDIRECT_URL` | — | Obligatoria al arrancar con SSO; Kairos usa siempre `<PUBLIC_URL>/api/auth/oidc/callback`, que es la que debes registrar en el proveedor |
| `OIDC_ROLE_SOURCE` | `idp` | `idp`: los roles vienen del proveedor (Usuarios → Homologaciones); `local`: se asignan en Kairos |
| `OIDC_LINK_BY_EMAIL` | `false` | Une una cuenta del SSO con una cuenta local que tenga el mismo correo |

### Archivos de las páginas

Sin `FILES_S3_ENDPOINT`, los archivos se guardan en PostgreSQL. **El almacenamiento no se puede cambiar una vez hay archivos guardados.**

| Variable | Por defecto | Qué es |
|---|---|---|
| `FILES_S3_ENDPOINT` ① | — | `host[:puerto]` del servicio S3, sin `https://` |
| `FILES_S3_BUCKET` ① | — | Bucket, que debe existir |
| `FILES_S3_REGION` ① | — | Región, si el proveedor la pide |
| `FILES_S3_ACCESS_KEY` ① | — | Clave de acceso |
| `FILES_S3_SECRET_KEY` ① | — | Clave secreta |
| `FILES_S3_USE_SSL` ① | `true` | Conectar por HTTPS |

### Informes con IA

El servicio de IA (dirección, clave y modelo) no va en variables: se configura en **Sistema → Informes con IA**. Las variables solo dicen dónde está el agente que escribe los informes ([ADR 0004](../adr/0004-agente-de-informes-en-python.md)); los `compose.yml` ya las ponen.

| Variable | Por defecto | Qué es |
|---|---|---|
| `AGENT_URL` | — | Dirección interna del agente, por ejemplo `http://agent:8000`. Vacía: no se pueden generar informes, aunque se leen y editan los guardados |
| `AGENT_CALLBACK_URL` | `http://api:<API_PORT>` | Cómo llega el agente a la API para hacer sus consultas |

### Otras

| Variable | Por defecto | Qué es |
|---|---|---|
| `API_PORT` | `8080` | Puerto en el que escucha la API |
| `WEB_PORT` | `5173` en local, `8081` en servidor | Puerto publicado del frontend |
| `TEST_DATABASE_URL` | — | Solo para los tests de integración |

## La pantalla Sistema

| Pestaña | Qué se ajusta |
|---|---|
| Acceso | Modo de acceso, registro libre y rol por defecto |
| SSO | Proveedor OIDC, con una prueba de conexión antes de guardar |
| Almacenamiento | Base de datos o S3, con una prueba antes de guardar |
| Apariencia | Paleta, color primario y esquinas de toda la interfaz |
| IA (MCP) | Activar las conexiones de clientes de IA y revisar o revocar las de todos |
| Informes con IA | Servicio compatible con la API de OpenAI (dirección base, modelo y clave), con una prueba de conexión; y las plantillas de informe con su prompt |
| Correo | Servidor SMTP y remitente, con un correo de prueba. Activa las invitaciones y «¿Olvidaste tu contraseña?» |

Los secretos guardados aquí no se vuelven a mostrar: el campo queda vacío y solo se reemplazan si escribes uno nuevo.

## Seguridad

- No subas nunca al repositorio un `.env` ni un `kairos.env` con valores reales.
- Las rutas `/api/mcp/...` llevan el secreto de cada conexión de IA. El nginx de `web` no las registra; no las registres tampoco en tu proxy.
- Los enlaces de invitación y recuperación llevan el secreto tras `#`, que el navegador nunca envía al servidor.
- El agente de informes no publica puertos fuera de la red de Docker y no lee la base de datos: solo consulta la API con el token de cada generación, que deja de valer al terminar.
