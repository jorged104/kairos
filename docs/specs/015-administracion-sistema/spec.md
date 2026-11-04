# Spec 015 — Administración del sistema: acceso, SSO, almacenamiento y apariencia

**Estado:** Aprobada (2026-11-04) («Dale»).

## Contexto y objetivo
Hoy el modo de acceso, el SSO (OIDC) y el almacenamiento de archivos (S3) solo se configuran con variables de entorno al instalar Kairos. Cualquier cambio exige tocar el servidor y reiniciar. Además, la apariencia es fija: todas las instalaciones tienen los mismos colores.

Esta spec lleva esa configuración a una pantalla de **administración del sistema**. Desde ahí el administrador:
- elige el modo de acceso y si hay registro abierto;
- conecta el SSO;
- conecta el almacenamiento S3;
- elige la paleta, el color primario y el radio de las esquinas.

Cada persona, además, elige si ve Kairos en claro, en oscuro o como su sistema.

La seguridad pesa más que la comodidad en tres puntos:
- los secretos nunca se vuelven a mostrar;
- un SSO mal configurado no puede dejar a todos fuera;
- el almacenamiento no se cambia con archivos dentro.

Es una configuración neutra, válida para cualquier organización.

## Usuarios / actores
- **Administrador del sistema** (con el permiso nuevo de administrar el sistema): configura acceso, SSO, almacenamiento y apariencia.
- **Administrador inicial:** la persona cuyo email indica la instalación como administrador de arranque. Conserva un acceso de emergencia con contraseña.
- **Cualquier usuario:** ve la apariencia elegida y escoge claro, oscuro o sistema para sí.
- **Visitante sin sesión:** ve en el login los métodos de entrada vigentes y la apariencia elegida.

## Vocabulario
- **Configuración del sistema:** los ajustes de esta spec, que Kairos guarda y que se pueden cambiar sin reiniciar.
- **Secreto:** un dato que da acceso a otro sistema, como el *client secret* del SSO o la clave secreta del S3.
- **Modo de acceso:** solo contraseña, solo SSO, o ambos (los modos `local`, `oidc` y `both` de la spec 003).
- **Login de prueba:** un inicio de sesión completo contra el SSO configurado. Lo hace el administrador para demostrar que funciona, y no cambia su sesión actual.
- **Almacenamiento de archivos:** dónde se guardan los archivos de páginas y comentarios (spec 005). Puede ser la base de datos o un S3.
- **Paleta base:** un conjunto de grises predefinido para fondos, bordes y textos: Neutral, Zinc, Slate, Stone o Gray.
- **Color primario:** el color de acento de botones, enlaces y selección. Se elige de una lista predefinida o se deja «el de la paleta».

## Historias de usuario
- H1: Como administrador quiero conectar el SSO de mi organización desde la interfaz para no depender de quien administra el servidor.
- H2: Como administrador quiero cambiar el modo de acceso sin riesgo de que nadie pueda volver a entrar.
- H3: Como administrador quiero conectar un S3 antes de que se suban archivos.
- H4: Como administrador quiero que Kairos use los colores de mi organización.
- H5: Como usuario quiero ver Kairos en modo oscuro aunque mi sistema esté en claro.

## Requisitos funcionales (criterios de aceptación en EARS)

### General
- RF-1: EL SISTEMA mostrará la pantalla «Sistema» en Configuración solo a quien tenga el permiso de administrar el sistema.
- RF-2: SI alguien sin ese permiso intenta leer o cambiar la configuración del sistema, ENTONCES EL SISTEMA lo rechazará.
- RF-3: CUANDO se guarde un cambio de la configuración del sistema, EL SISTEMA lo aplicará en menos de 10 s, sin reiniciar.
- RF-4: EL SISTEMA auditará cada cambio de la configuración del sistema. Para cada campo cambiado registrará el valor anterior y el nuevo, salvo en los secretos (RF-6).
- RF-5: EL SISTEMA guardará los secretos cifrados.
- RF-6: EL SISTEMA no devolverá nunca un secreto guardado. Mostrará solo si está configurado, y la auditoría solo registrará que cambió.
- RF-7: CUANDO el administrador guarde un formulario sin escribir un secreto ya configurado, EL SISTEMA conservará el secreto anterior.

### Procedencia de la configuración
- RF-8: CUANDO Kairos arranque por primera vez con esta funcionalidad, EL SISTEMA copiará a la configuración del sistema los valores de las variables de entorno que haya de:
  - modo de acceso;
  - registro abierto y rol por defecto;
  - SSO;
  - S3.
- RF-9: Después de esa primera copia, EL SISTEMA usará la configuración guardada desde la interfaz, aunque las variables de entorno cambien.
- RF-10: EL SISTEMA seguirá tomando de las variables de entorno lo que esta spec no lleva a la interfaz:
  - la URL pública;
  - el secreto de sesión;
  - el administrador inicial;
  - de dónde salen los roles del SSO;
  - la vinculación por email.

### Modo de acceso y registro
- RF-11: EL SISTEMA permitirá elegir el modo de acceso: solo contraseña, solo SSO o ambos.
- RF-12: SI se elige «solo SSO» o «ambos» sin una configuración de SSO guardada, ENTONCES EL SISTEMA rechazará el cambio.
- RF-13: SI se elige «solo SSO» sin un login de prueba correcto con la configuración de SSO vigente (RF-24), ENTONCES EL SISTEMA rechazará el cambio.
- RF-14: EL SISTEMA permitirá activar o desactivar el registro abierto. Solo tendrá efecto mientras el acceso con contraseña esté habilitado (sigue a RF-11 y RF-12 de la spec 003).
- RF-15: EL SISTEMA permitirá elegir el rol por defecto de los usuarios nuevos entre «sin rol», `viewer`, `member` y `lead`.
- RF-16: EL SISTEMA no ofrecerá `admin` como rol por defecto.
- RF-17: CUANDO cambie el modo de acceso, EL SISTEMA mostrará en la pantalla de login solo los métodos del modo nuevo (sigue a RF-6 de la spec 003).
- RF-18: CUANDO cambie el modo de acceso, EL SISTEMA no cerrará las sesiones abiertas.

### Acceso de emergencia
- RF-19: MIENTRAS el modo sea «solo SSO», EL SISTEMA aceptará el login con contraseña del administrador inicial, y de nadie más.
- RF-20: MIENTRAS el modo sea «solo SSO», EL SISTEMA mostrará en la pantalla de login un enlace discreto «Acceso de emergencia», que abre el formulario de contraseña.
- RF-21: EL SISTEMA aplicará al acceso de emergencia el mismo límite de intentos fallidos que al login con contraseña (RF-9 de la spec 003).
- RF-22: EL SISTEMA auditará cada acceso de emergencia que tenga éxito.
- RF-23: SI no hay administrador inicial configurado, o este no tiene contraseña local, ENTONCES EL SISTEMA avisará en la pantalla «Sistema» de que no hay acceso de emergencia antes de permitir el modo «solo SSO».

### SSO
- RF-24: EL SISTEMA permitirá configurar el SSO con:
  - el emisor (issuer);
  - la URL de descubrimiento, que es opcional;
  - el *client id*;
  - el *client secret*.
- RF-25: EL SISTEMA mostrará la URL de retorno que hay que registrar en el proveedor. La calcula a partir de la URL pública y no se puede editar.
- RF-26: CUANDO se guarde la configuración del SSO, EL SISTEMA comprobará que el descubrimiento del emisor responde en menos de 5 s.
- RF-27: SI el descubrimiento no responde o no es válido, ENTONCES EL SISTEMA rechazará la configuración e indicará el motivo.
- RF-28: EL SISTEMA permitirá al administrador lanzar un login de prueba contra la configuración de SSO guardada.
- RF-29: CUANDO el login de prueba termine, EL SISTEMA mostrará si fue correcto y, si lo fue, el email y el nombre que devolvió el proveedor.
- RF-30: EL SISTEMA no abrirá sesión, ni creará ni vinculará ningún usuario, a partir de un login de prueba.
- RF-31: CUANDO cambie cualquier dato de la configuración del SSO, EL SISTEMA dará por no hecho el login de prueba anterior.
- RF-32: SI se cambia la configuración del SSO mientras el modo es «solo SSO», ENTONCES EL SISTEMA rechazará el cambio salvo que se pase antes el modo a «ambos».

### Almacenamiento de archivos
- RF-33: EL SISTEMA permitirá elegir entre guardar los archivos en la base de datos o en un S3.
- RF-34: Para un S3, EL SISTEMA pedirá:
  - el endpoint (host y puerto, sin esquema);
  - el bucket;
  - la región;
  - la clave de acceso;
  - la clave secreta;
  - si usa TLS.
- RF-35: CUANDO se guarde una configuración de S3, EL SISTEMA comprobará en menos de 10 s que puede escribir, leer y borrar un objeto de prueba en el bucket.
- RF-36: SI esa comprobación falla, ENTONCES EL SISTEMA rechazará la configuración e indicará en qué paso falló.
- RF-37: MIENTRAS exista al menos un archivo guardado, EL SISTEMA no permitirá cambiar el tipo de almacenamiento, el endpoint ni el bucket, e indicará cuántos archivos hay.
- RF-38: MIENTRAS exista al menos un archivo guardado en un S3, EL SISTEMA permitirá cambiar la región, las claves y el uso de TLS, con la comprobación de RF-35.

### Apariencia
- RF-39: EL SISTEMA permitirá elegir la paleta base entre Neutral, Zinc, Slate, Stone y Gray.
- RF-40: EL SISTEMA permitirá elegir el color primario entre «el de la paleta» y una lista predefinida: azul, verde, naranja, rojo, rosa, violeta y amarillo.
- RF-41: EL SISTEMA aplicará cada combinación de paleta y color primario con una variante clara y otra oscura.
- RF-42: EL SISTEMA garantizará en las dos variantes un contraste de al menos 4,5:1 entre el color primario y su texto.
- RF-43: EL SISTEMA permitirá elegir el radio de las esquinas entre recto, suave y redondeado.
- RF-44: EL SISTEMA mostrará una vista previa de la apariencia elegida, en claro y en oscuro, antes de guardarla.
- RF-45: CUANDO se guarde la apariencia, EL SISTEMA la aplicará a todos los usuarios y a la pantalla de login.
- RF-46: EL SISTEMA empezará con la apariencia actual: paleta Neutral, color primario «el de la paleta» y radio suave.
- RF-47: EL SISTEMA permitirá volver a la apariencia inicial con una sola acción.

### Claro u oscuro por usuario
- RF-48: EL SISTEMA permitirá a cada usuario elegir en Mi cuenta entre claro, oscuro y «como el sistema».
- RF-49: EL SISTEMA usará «como el sistema» por defecto.
- RF-50: EL SISTEMA recordará la elección de cada usuario en todos sus dispositivos.
- RF-51: MIENTRAS el usuario no tenga sesión, EL SISTEMA mostrará el login según la preferencia del sistema operativo.

## Requisitos no funcionales
- **Seguridad:**
  - los secretos se guardan cifrados y no salen nunca del servidor (RF-5, RF-6);
  - ni los logs ni la auditoría muestran su valor.
- **Disponibilidad:** un fallo al comprobar el SSO o el S3 no cambia la configuración vigente.
- **Permisos:** administrar el sistema es un permiso de la matriz, solo para `admin`, nunca un nombre de rol.
- **Idioma:** todos los textos en español desde el catálogo de textos.

## Casos límite
- **Instalación nueva sin variables de SSO ni S3:** empieza en «solo contraseña», con los archivos en la base de datos y la apariencia inicial.
- **Variables de entorno con un SSO incompleto en la primera copia:** se copian los datos que haya. El modo de acceso se queda en «solo contraseña» hasta que el SSO esté completo y probado, y se avisa en la pantalla «Sistema».
- **El proveedor de SSO cae con el modo «solo SSO»:** el administrador inicial entra por el acceso de emergencia (RF-19) y pasa el modo a «ambos».
- **Dos administradores guardan a la vez:** gana el último, y cada cambio queda auditado.
- **Archivos borrados hasta quedar 0:** el almacenamiento vuelve a poder cambiarse (RF-37).
- **Un S3 que deja de aceptar las claves:** las subidas fallan con el error de almacenamiento que ya existe, hasta que se corrigen las claves (RF-38).

## Fuera de alcance
- Migrar archivos de un almacenamiento a otro.
- Editar a mano cada variable de color, o colores fuera de la lista.
- Logo y nombre de la aplicación personalizados.
- Varios proveedores de SSO a la vez, y SAML.
- Llevar a la interfaz el origen de los roles del SSO, la vinculación por email, la duración de la sesión o la URL pública.
- Configurar el correo saliente (SMTP).
- Apariencias distintas por equipo o por usuario, más allá de claro y oscuro.

## Criterios de finalización
- Todos los RF con test en verde:
  - backend: apitest con la fila del permiso nuevo en la matriz, la primera copia desde las variables de entorno y las comprobaciones simuladas de SSO y S3;
  - frontend.
- Demo manual:
  1. configurar el SSO de prueba, hacer el login de prueba y pasar a «solo SSO»;
  2. entrar por el acceso de emergencia;
  3. conectar un S3 con la instalación vacía y ver que queda bloqueado al subir un archivo;
  4. cambiar la paleta, el color primario y el radio, y ver el login;
  5. elegir el modo oscuro en Mi cuenta.

## Dudas abiertas
Ninguna. Confirmados el 2026-11-04 el rol por defecto sin `admin` (RF-15, RF-16) y la lista de colores primarios (RF-40).

### Resueltas
- RF-15 y RF-16 limitan el rol por defecto a «sin rol», `viewer`, `member` y `lead`; hoy la variable de entorno admite cualquier rol. Confírmalo.
- RF-40 fija la lista de colores primarios. Confírmala o cámbiala.
