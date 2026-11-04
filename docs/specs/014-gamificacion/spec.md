# Spec 014 — Gamificación: XP, monedas, niveles y rankings por equipo

**Estado:** Aprobada (2026-11-04) («Dale»).

## Contexto y objetivo
Kairos registra qué trabajo se cierra, pero no lo reconoce. Esta spec premia el trabajo terminado:
- cada persona gana **XP** y **monedas** cuando se cierra un work item en el que está asignada;
- su **nivel** se deriva del XP acumulado;
- cada persona puede compararse con su equipo en un **ranking**.

Todo movimiento de puntos queda en un **registro inmutable**, para poder explicar de dónde sale cada saldo y corregirlo sin borrar historia.

La base son los eventos que ya emite la spec 004:
- `work_item.closed` y `work_item.reopened`;
- a quién está asignado cada work item (spec 006, RF-122 de la spec 004);
- la talla de cada work item (spec 006).

Los conceptos sirven a cualquier industria.

## Usuarios / actores
- **Cualquier usuario activo:**
  - gana puntos por sus work items;
  - ve su XP, nivel, monedas e historial;
  - ve el ranking de sus equipos.
- **Administrador de la gamificación** (con permiso para ello):
  - configura cuántos puntos da cada talla;
  - configura los niveles;
  - ajusta a mano los puntos de una persona.
- **Auditor:** consulta los ajustes y los cambios de configuración en la auditoría.

## Vocabulario
- **XP:** puntos de experiencia. Solo crecen con trabajo cerrado y con ajustes del administrador.
- **Monedas:** un segundo saldo que se gana igual que el XP. En esta versión solo se acumulan; no se gastan.
- **Movimiento:** una entrada del registro de puntos. Indica:
  - la persona;
  - el XP y las monedas, que pueden ser positivos o negativos;
  - el motivo (cierre, reapertura o ajuste);
  - el work item, si lo hay;
  - el momento;
  - quién lo causó.
- **Saldo:** la suma de los movimientos de una persona, uno para el XP y otro para las monedas.
- **Tabla de puntos:** cuántos XP y monedas vale cerrar un work item de cada talla, más el valor «sin talla».
- **Nivel:** el escalón que corresponde al XP de una persona según la tabla de niveles. Cada nivel tiene un número, un nombre y el XP mínimo para alcanzarlo.

## Historias de usuario
- H1: Como persona del equipo quiero ganar XP y monedas al cerrar mis work items para ver reconocido mi trabajo.
- H2: Como persona del equipo quiero ver mi nivel y cuánto me falta para el siguiente.
- H3: Como persona del equipo quiero ver de dónde sale cada punto para entender y confiar en mi saldo.
- H4: Como persona del equipo quiero compararme con mi equipo en un ranking.
- H5: Como administrador quiero decidir cuánto vale cada talla y cada nivel para ajustar el sistema a mi organización.
- H6: Como administrador quiero corregir los puntos de alguien con un motivo para arreglar errores sin borrar historia.

## Requisitos funcionales (criterios de aceptación en EARS)

### Registro de puntos
- RF-1: EL SISTEMA guardará cada cambio de XP o de monedas como un movimiento nuevo del registro de puntos.
- RF-2: EL SISTEMA no permitirá modificar ni borrar un movimiento ya registrado.
- RF-3: EL SISTEMA calculará el saldo de XP de una persona como la suma del XP de sus movimientos.
- RF-4: EL SISTEMA calculará el saldo de monedas de una persona como la suma de las monedas de sus movimientos.
- RF-5: EL SISTEMA no guardará el XP, las monedas ni el nivel como datos editables del usuario.

### Puntos por cerrar un work item
- RF-6: CUANDO se cierre un work item, EL SISTEMA registrará para cada uno de sus asignados en ese momento un movimiento con el XP y las monedas completos que la tabla de puntos asigna a su talla.
- RF-7: CUANDO se cierre un work item sin talla, EL SISTEMA usará el valor «sin talla» de la tabla de puntos.
- RF-8: SI el work item se cierra sin asignados, ENTONCES EL SISTEMA no registrará ningún movimiento.
- RF-9: SI el work item pertenece a un proyecto plantilla, ENTONCES EL SISTEMA no registrará ningún movimiento.
- RF-10: SI el valor de la tabla para esa talla es 0 XP y 0 monedas, ENTONCES EL SISTEMA no registrará ningún movimiento.
- RF-11: EL SISTEMA usará la tabla de puntos vigente en el momento del cierre. Un cambio posterior de la tabla no modificará los movimientos ya registrados.
- RF-12: CUANDO llegue dos veces el mismo evento de cierre, EL SISTEMA registrará sus movimientos una sola vez.
- RF-13: SI un asignado está inactivo o dado de baja en el momento del cierre, ENTONCES EL SISTEMA le registrará igualmente los puntos (RF-6).

### Reapertura
- RF-14: CUANDO se reabra un work item, EL SISTEMA registrará, por cada movimiento de su último cierre, un movimiento contrario con el mismo XP y las mismas monedas en negativo, para la misma persona.
- RF-15: CUANDO se vuelva a cerrar un work item reabierto, EL SISTEMA registrará los puntos como en un primer cierre (RF-6 a RF-11), con los asignados, la talla y la tabla de ese momento.
- RF-16: SI se reabre un work item cuyo último cierre no registró movimientos, ENTONCES EL SISTEMA no registrará ningún movimiento.
- RF-17: CUANDO un work item cambie entre dos estados cerrados, EL SISTEMA no registrará ningún movimiento (sigue a RF-118 de la spec 004).

### Ajustes manuales
- RF-18: EL SISTEMA permitirá a un administrador de la gamificación registrar un ajuste a una persona, con XP y monedas positivos o negativos y un motivo obligatorio de 3 a 500 caracteres.
- RF-19: SI el ajuste tiene 0 XP y 0 monedas, ENTONCES EL SISTEMA lo rechazará.
- RF-20: SI el ajuste dejaría el saldo de XP o el de monedas de la persona por debajo de 0, ENTONCES EL SISTEMA lo rechazará.
- RF-21: SI quien ajusta intenta ajustar sus propios puntos, ENTONCES EL SISTEMA rechazará el ajuste.
- RF-22: EL SISTEMA auditará cada ajuste en la misma operación que lo registra.
- RF-23: SI quien intenta un ajuste no tiene permiso de administrar la gamificación, ENTONCES EL SISTEMA lo rechazará.

### Tabla de puntos
- RF-24: EL SISTEMA permitirá a un administrador de la gamificación fijar el XP y las monedas, enteros de 0 a 1000, de cada talla del catálogo y del valor «sin talla».
- RF-25: CUANDO se cree una talla nueva en el catálogo, EL SISTEMA le asignará 0 XP y 0 monedas hasta que se configure.
- RF-26: EL SISTEMA mostrará en la tabla las tallas retiradas que tengan valores, marcadas como retiradas, porque los work items que ya las tienen siguen usándolas.
- RF-27: EL SISTEMA creará la tabla inicial así: sin talla, 5 XP y 1 moneda; XS, 5 XP y 1 moneda; S, 10 XP y 2 monedas; M, 20 XP y 4 monedas; L, 40 XP y 8 monedas; XL, 80 XP y 16 monedas.
- RF-28: EL SISTEMA auditará cada cambio de la tabla de puntos.

### Niveles
- RF-29: EL SISTEMA calculará el nivel de una persona como el nivel de mayor XP mínimo que no supere su saldo de XP.
- RF-30: EL SISTEMA permitirá a un administrador de la gamificación crear, renombrar, cambiar el XP mínimo y borrar niveles. Cada nivel tiene un nombre de 1 a 40 caracteres.
- RF-31: EL SISTEMA mantendrá siempre un primer nivel con XP mínimo 0, que no se puede borrar ni cambiar de mínimo.
- RF-32: SI dos niveles tendrían el mismo XP mínimo, ENTONCES EL SISTEMA rechazará el cambio.
- RF-33: EL SISTEMA numerará los niveles del 1 en adelante, por orden de XP mínimo.
- RF-34: EL SISTEMA creará la tabla inicial con 10 niveles: el nivel n exige 50 × n × (n − 1) XP (0, 100, 300, 600, 1000, 1500, 2100, 2800, 3600 y 4500), con los nombres «Nivel 1» a «Nivel 10».
- RF-35: CUANDO cambie la tabla de niveles, EL SISTEMA recalculará el nivel de todas las personas sin registrar movimientos.
- RF-36: EL SISTEMA auditará cada cambio de la tabla de niveles.

### Lo que ve cada persona
- RF-37: EL SISTEMA mostrará a cada persona, en su tarjeta de Mi panel y en Mi cuenta:
  - su nivel (número y nombre);
  - su XP;
  - sus monedas;
  - una barra con el avance hacia el siguiente nivel, con el XP que le falta.
- RF-38: MIENTRAS una persona esté en el nivel más alto, EL SISTEMA mostrará que ha alcanzado el nivel máximo en lugar de la barra.
- RF-39: EL SISTEMA mostrará a cada persona su historial de movimientos, del más reciente al más antiguo, 50 por página. Cada movimiento indica:
  - la fecha;
  - el XP y las monedas;
  - el motivo;
  - el work item enlazado a su ficha, si lo hay;
  - en los ajustes, el texto del motivo.
- RF-40: EL SISTEMA no mostrará a una persona el historial de otra, salvo a quien tenga permiso de administrar la gamificación.
- RF-41: CUANDO una persona gane puntos al cerrar un work item desde la interfaz, EL SISTEMA le mostrará un aviso con el XP y las monedas ganados.
- RF-42: CUANDO ese cierre le haga subir de nivel, EL SISTEMA lo indicará en el mismo aviso con el nombre del nivel nuevo.

### Ranking por equipo
- RF-43: EL SISTEMA mostrará a cada persona un ranking por cada equipo del que es miembro directo. Solo cuentan los miembros activos de ese equipo y de sus subequipos.
- RF-44: EL SISTEMA ordenará el ranking por el XP del periodo elegido (RF-47), de mayor a menor; en caso de empate, por nombre.
- RF-45: EL SISTEMA mostrará de cada persona del ranking su posición, su nombre, su nivel (según su XP de siempre) y su XP del periodo.
- RF-46: EL SISTEMA no mostrará en ningún ranking a los usuarios inactivos ni a los dados de baja.
- RF-47: EL SISTEMA permitirá elegir el periodo del ranking:
  - **Semana:** de lunes a domingo, la que contiene la fecha de hoy de quien consulta;
  - **Mes:** el mes natural en curso;
  - **Siempre.**
- RF-48: EL SISTEMA mostrará el periodo «Mes» por defecto.
- RF-49: EL SISTEMA calculará el XP de un periodo como la suma del XP de los movimientos registrados dentro de él, incluidas las reaperturas y los ajustes negativos.
- RF-50: SI nadie del equipo tiene movimientos en el periodo, ENTONCES EL SISTEMA mostrará a todos los miembros con 0 XP, ordenados por nombre.
- RF-51: SI una persona no pertenece a ningún equipo, ENTONCES EL SISTEMA le indicará que no tiene ranking.
- RF-52: SI alguien pide el ranking de un equipo del que no es miembro, ENTONCES EL SISTEMA lo rechazará, salvo que tenga permiso de administrar la gamificación.

### Puesta en marcha
- RF-53: CUANDO se active esta funcionalidad, EL SISTEMA empezará a todas las personas con 0 XP y 0 monedas, sin dar puntos por los work items que ya estaban cerrados.
- RF-54: SI se reabre un work item cerrado antes de activar la funcionalidad, ENTONCES EL SISTEMA no registrará ningún movimiento (RF-16).

## Requisitos no funcionales
- **Coherencia:** los movimientos de un cierre se registran a partir del evento del cierre, con entrega al menos una vez (spec 004, RF-121). Que un evento llegue dos veces no duplica puntos. Que falle el registro de puntos no deshace el cierre.
- **Retraso:** los puntos de un cierre aparecen en el saldo en menos de 10 s.
- **Seguridad:** el frontend nunca registra movimientos. Los puntos solo se generan en el servidor, por eventos o ajustes.
- **Permisos:** administrar la gamificación es un permiso de la matriz, no un rol.
- **Idioma:** todos los textos en español desde el catálogo de textos.

## Casos límite
- **Cerrar, reabrir y volver a cerrar con otros asignados:** la reapertura descuenta a los del primer cierre y el nuevo cierre paga a los nuevos (RF-14, RF-15).
- **Cambio de la tabla entre el cierre y la reapertura:** se descuenta exactamente lo que se dio, no lo que vale hoy (RF-14).
- **Cambio de talla con el work item cerrado:** no mueve puntos. Cuenta la talla del próximo cierre.
- **Reapertura tras un ajuste negativo:** la reapertura se registra completa aunque el saldo quede negativo. Solo los ajustes respetan el mínimo 0 (RF-20).
- **Work item cerrado mientras el proyecto se archiva:** puntúa igual. El proyecto activo solo importa en Mi panel.
- **20 asignados en una talla XL:** cada uno recibe el valor completo (RF-6), hasta 1600 XP por cierre.
- **Persona que deja un equipo:** sale de su ranking y su XP no cambia.
- **Borrar el nivel en el que están personas:** pasan al nivel inferior que les corresponda (RF-35).

## Fuera de alcance
- Gastar o canjear monedas: tienda, recompensas o regalos entre compañeros.
- Puntos por cumplir hitos, por puntualidad o penalizaciones por retraso.
- Puntos por cualquier otra acción que no sea cerrar un work item (comentar, crear, estimar…).
- Ranking de toda la organización o entre equipos.
- Insignias, logros y rachas.
- Notificaciones fuera de la interfaz (correo, chat).
- Tablas de puntos distintas por proyecto, equipo o tipo de work item.
- Repartir los puntos entre los asignados.

## Criterios de finalización
- Todos los RF con test en verde: backend, apitest con la fila del permiso nuevo en la matriz, y frontend.
- Demo manual:
  1. cerrar un work item con dos asignados y ver el saldo y el aviso de cada uno;
  2. reabrirlo y ver el descuento en el historial;
  3. cambiar la tabla de puntos y la de niveles;
  4. hacer un ajuste manual y verlo en la auditoría;
  5. ver el ranking de un equipo.

## Dudas abiertas
Ninguna. Resueltas el 2026-11-04:
- los asignados inactivos o dados de baja también reciben puntos (RF-13);
- el ranking tiene periodo Semana, Mes o Siempre, con Mes por defecto (RF-47);
- todos empiezan en 0 (RF-53).

Los valores iniciales de las tablas de puntos (RF-27) y de niveles (RF-34) quedan como propuestos.

## Impacto en otras specs
- **Spec 013:** la tarjeta de perfil de Mi panel añade el nivel, el XP, las monedas y la barra hacia el siguiente nivel (RF-37).
