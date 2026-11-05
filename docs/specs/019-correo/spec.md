# Spec 019 — Correo: invitaciones y recuperación de contraseña

**Estado:** Aprobada (2026-11-05) («dámole»).

## Contexto y objetivo
Kairos no envía correo. Cuando un administrador da de alta a una persona, le pone una contraseña inicial y tiene que pasársela por otro medio, lo que es incómodo y poco seguro. Si alguien olvida su contraseña, solo un administrador puede ponerle otra. La spec 003 dejó el correo fuera porque no había envío, y la spec 015 dejó fuera la configuración del correo saliente.

Esta spec añade:
- **Correo saliente**, que el administrador del sistema configura en la pantalla «Sistema» con un servidor SMTP.
- **Invitaciones:** al dar de alta a alguien, Kairos le envía un enlace para crear su contraseña. El administrador ya no la pone.
- **Recuperación de contraseña:** desde el login, la persona pide un enlace para poner una nueva.

Si el correo no está configurado o falla, el administrador obtiene el enlace para copiarlo y pasarlo por otro medio, así que nadie se queda sin poder entrar.

Es neutro respecto al sector: vale para cualquier organización.

## Usuarios / actores
- **Administrador del sistema** (permiso de administrar el sistema, spec 015): configura el correo y envía un correo de prueba.
- **Quien puede dar de alta usuarios** (spec 003): da de alta, reenvía invitaciones y envía enlaces de nueva contraseña.
- **Persona invitada:** recibe la invitación y crea su contraseña.
- **Cualquier persona con contraseña:** pide un enlace para restablecerla.

## Vocabulario
- **Correo configurado:** hay un servidor SMTP guardado y activo en Sistema.
- **Enlace de invitación:** dirección de un solo uso que deja a una persona nueva crear su contraseña. Vale 7 días.
- **Enlace de recuperación:** dirección de un solo uso que deja poner una contraseña nueva. Vale 1 hora.
- **Invitación pendiente:** la de una persona que todavía no ha usado su enlace de invitación.

## Historias de usuario
- H1: Como administrador quiero dar de alta a una persona sin inventarle una contraseña, y que le llegue un correo para crearla ella.
- H2: Como persona que olvidó su contraseña quiero recuperarla yo misma desde el login.
- H3: Como administrador quiero configurar el servidor de correo de mi organización y comprobar que funciona.
- H4: Como administrador quiero poder dar de alta aunque el correo falle, pasando el enlace por otro medio.

## Requisitos funcionales (criterios de aceptación en EARS)

### Configurar el correo
- RF-1: EL SISTEMA mostrará en la pantalla «Sistema» una pestaña «Correo», solo a quien tenga el permiso de administrar el sistema.
- RF-2: EL SISTEMA guardará del correo:
  - el servidor y el puerto;
  - la seguridad de la conexión: ninguna, STARTTLS o TLS;
  - el usuario y la contraseña, ambos opcionales;
  - la dirección y el nombre del remitente.
- RF-3: EL SISTEMA permitirá activar o desactivar el envío de correo sin borrar su configuración.
- RF-4: EL SISTEMA guardará la contraseña del servidor de correo cifrada y no la devolverá nunca, como los demás secretos de la spec 015 (RF-5 a RF-7).
- RF-5: SI la dirección del remitente no es un correo válido, ENTONCES EL SISTEMA rechazará la configuración e indicará el campo.
- RF-6: CUANDO el administrador pulse «Enviar correo de prueba», EL SISTEMA enviará un correo a la dirección del administrador con la configuración guardada.
- RF-7: SI el correo de prueba falla, ENTONCES EL SISTEMA mostrará en qué paso falló: conexión, seguridad, autenticación o envío.
- RF-8: EL SISTEMA auditará cada cambio de la configuración del correo, con la contraseña solo como «cambiada».
- RF-9: CUANDO se guarde un cambio de la configuración del correo, EL SISTEMA lo aplicará en menos de 10 s, sin reiniciar (como RF-3 de la spec 015).

### Invitaciones
- RF-10: CUANDO se dé de alta a una persona, EL SISTEMA no pedirá contraseña inicial.
- RF-11: CUANDO se dé de alta a una persona, EL SISTEMA creará para ella un enlace de invitación válido durante 7 días.
- RF-12: MIENTRAS el correo esté configurado, CUANDO se cree un enlace de invitación, EL SISTEMA lo enviará a la dirección de la persona.
- RF-13: SI el correo no está configurado o el envío falla, ENTONCES EL SISTEMA completará igualmente el alta, avisará al administrador y le mostrará el enlace para copiarlo.
- RF-14: MIENTRAS el modo de acceso permita contraseña (solo contraseña, o contraseña y SSO), EL SISTEMA hará que el enlace de invitación lleve a una página para crear la contraseña.
- RF-15: MIENTRAS el modo de acceso sea solo SSO, EL SISTEMA enviará como invitación un aviso con el enlace al login, sin crear contraseña.
- RF-16: CUANDO la persona cree su contraseña desde una invitación válida, EL SISTEMA la guardará, anulará el enlace y la llevará al login con un aviso de que ya puede entrar.
- RF-17: EL SISTEMA aplicará a la contraseña creada desde un enlace las mismas reglas que a cualquier contraseña (spec 003).
- RF-18: SI un enlace de invitación está caducado, ya usado o anulado, ENTONCES EL SISTEMA mostrará que no es válido y que hay que pedir otro a un administrador.
- RF-19: EL SISTEMA mostrará en la ficha y en el directorio de usuarios si la persona tiene una invitación pendiente y hasta cuándo vale.
- RF-20: EL SISTEMA permitirá reenviar la invitación a quien tenga una pendiente o caducada; reenviarla anula el enlace anterior y crea uno nuevo de 7 días.
- RF-21: EL SISTEMA aplicará al reenvío de una invitación lo mismo que al alta cuando el correo no está configurado o falla (RF-13).

### Recuperar la contraseña
- RF-22: MIENTRAS el modo de acceso permita contraseña, EL SISTEMA mostrará en el login «¿Olvidaste tu contraseña?».
- RF-23: MIENTRAS el modo de acceso sea solo SSO, EL SISTEMA mostrará «¿Olvidaste tu contraseña?» solo dentro del acceso de emergencia (spec 015).
- RF-24: MIENTRAS el modo de acceso sea solo SSO, EL SISTEMA atenderá la recuperación solo para el administrador inicial (spec 015).
- RF-25: CUANDO alguien pida recuperar la contraseña de una dirección, EL SISTEMA responderá siempre con el mismo mensaje, exista o no esa dirección, para no revelar quién tiene cuenta.
- RF-26: CUANDO se pida recuperar la contraseña de una persona activa que entra con contraseña, y el correo esté configurado, EL SISTEMA le enviará un enlace de recuperación válido durante 1 hora.
- RF-27: SI se piden más de 3 recuperaciones para la misma dirección en una hora, ENTONCES EL SISTEMA no enviará más correos a esa dirección en esa hora, sin cambiar el mensaje de respuesta.
- RF-28: CUANDO se cree un enlace de recuperación, EL SISTEMA anulará los enlaces de recuperación anteriores de esa persona.
- RF-29: CUANDO la persona ponga una contraseña nueva desde un enlace de recuperación válido, EL SISTEMA la guardará y anulará el enlace.
- RF-30: SI un enlace de recuperación está caducado, ya usado o anulado, ENTONCES EL SISTEMA mostrará que no es válido y ofrecerá pedir otro.
- RF-31: CUANDO se cambie la contraseña desde un enlace de recuperación, EL SISTEMA enviará un aviso a la persona, si el correo está configurado.

### El administrador y las cuentas existentes
- RF-32: EL SISTEMA permitirá a quien gestiona una persona enviarle un enlace de recuperación, en lugar de ponerle una contraseña a mano.
- RF-33: SI el correo no está configurado o el envío falla, ENTONCES EL SISTEMA mostrará al administrador el enlace de recuperación para copiarlo.
- RF-34: EL SISTEMA dejará de ofrecer al administrador poner a mano la contraseña de otra persona.
- RF-35: EL SISTEMA auditará cada invitación, reenvío y enlace de recuperación creado por un administrador, sin guardar el enlace.
- RF-36: EL SISTEMA auditará cada contraseña puesta desde un enlace, sin guardar la contraseña.

### Enlaces y correos
- RF-37: EL SISTEMA generará los enlaces con al menos 128 bits aleatorios.
- RF-38: EL SISTEMA guardará los enlaces de forma que no se puedan recuperar a partir de lo guardado.
- RF-39: EL SISTEMA no escribirá los enlaces en los logs.
- RF-40: EL SISTEMA enviará los correos en español, en texto plano y en HTML, con el nombre de la aplicación y el enlace completo.
- RF-41: EL SISTEMA no incluirá nunca una contraseña en un correo.
- RF-42: CUANDO se desactive a una persona, EL SISTEMA anulará sus enlaces de invitación y de recuperación.

## Requisitos no funcionales
- **Sin infraestructura nueva:** Kairos se conecta al servidor SMTP que indique la organización; no lleva su propio servidor de correo.
- **Sin esperas:** el alta y la petición de recuperación responden en menos de 2 s aunque el servidor de correo tarde. El envío de recuperación no bloquea la respuesta.
- **Seguridad:**
  - la contraseña del servidor de correo, como un secreto de la spec 015;
  - los enlaces, como los tokens de la spec 016: solo su huella se guarda.
- **Idioma:** los textos de la interfaz salen de `es.json`, y los correos también están en español.

## Casos límite
- **Persona creada por SSO (alta automática, spec 003):** no recibe invitación; entra por SSO.
- **Alta en modo solo SSO que luego pasa a contraseña:** la persona no tiene contraseña; el administrador le reenvía la invitación o un enlace de recuperación.
- **Invitación pendiente y la persona entra por SSO:** puede entrar; la invitación sigue pendiente hasta que caduque o se anule.
- **Invitación pendiente y la persona pide recuperar la contraseña:** no tiene contraseña, así que no se envía nada (RF-26); el mensaje es el mismo (RF-25).
- **Persona desactivada que pide recuperar:** no se envía nada y el mensaje es el mismo.
- **Varios enlaces de recuperación pedidos:** solo vale el último (RF-28).
- **Correo desactivado en Sistema:** cuenta como no configurado; el administrador ve los enlaces para copiarlos.
- **Sesiones abiertas al restablecer la contraseña:** siguen abiertas, porque Kairos no puede cerrar sesiones concretas (fuera de alcance en la spec 003).
- **Registro abierto (spec 015):** quien se registra pone su propia contraseña, sin invitación.
- **Administrador inicial:** se crea al arrancar como hasta ahora (spec 003), sin invitación.

## Fuera de alcance
- Notificaciones por correo (comentarios, menciones, cambios), resúmenes y preferencias de correo.
- Avisos de cuenta: cambio de rol o desactivación.
- Cambiar el propio correo con confirmación por enlace.
- Verificar la dirección en el registro abierto.
- Plantillas de correo editables, logotipos e idiomas distintos del español.
- Proveedores de correo por API (sin SMTP) y servidor de correo propio.
- Cerrar las sesiones abiertas al restablecer la contraseña.
- Copiar la configuración del correo desde variables de entorno.

## Criterios de finalización
- Todos los RF con test en verde:
  - backend:
    - apitest de la configuración, con un servidor SMTP de prueba en el propio test;
    - invitación con y sin correo, en cada modo de acceso;
    - recuperación: mismo mensaje, límite, último enlace, caducidad y uso único;
    - permisos y auditoría;
  - frontend: pestaña Correo, alta sin contraseña, enlace para copiar, reenviar, «¿Olvidaste tu contraseña?» y páginas de crear y restablecer contraseña.
- Demo manual:
  - con el servidor de correo de prueba de desarrollo, dar de alta a una persona, recibir la invitación y entrar;
  - pedir una recuperación y usar el enlace.

## Decisiones tomadas con el usuario (2026-11-05)
1. Correos: invitación al alta y «olvidé mi contraseña». Las notificaciones por correo quedan fuera.
2. Siempre invitación: desaparece la contraseña inicial.
3. Sin correo o con fallo, el alta se hace y el administrador copia el enlace.
4. Invitación válida 7 días y recuperación 1 hora, de un solo uso.
5. Según el modo de acceso: con contraseña, la invitación la crea; con solo SSO, avisa y la recuperación es solo para el administrador inicial.
6. El administrador puede reenviar invitaciones y enviar enlaces de recuperación, que sustituyen a poner la contraseña a mano.

Propuestas mías que van en el borrador, para confirmar:
- prueba de envío al correo del propio administrador, indicando el paso que falla (RF-6, RF-7);
- límite de 3 recuperaciones por dirección y hora (RF-27);
- aviso a la persona cuando su contraseña cambia por recuperación (RF-31);
- anular los enlaces al desactivar a la persona (RF-42);
- la configuración del correo solo se hace desde la interfaz, sin copiarla de variables de entorno;
- un servidor de correo de prueba para desarrollo (como el SSO y el S3 de prueba), solo en el entorno local.

## Dudas abiertas
- Ninguna.
