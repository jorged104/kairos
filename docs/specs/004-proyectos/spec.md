# Spec 004 — Proyectos, metodologías, hitos, work items y objetivos

**Estado:** Aprobada (2026-10-23). Las decisiones abiertas
del borrador original quedan tomadas en «Decisiones de la revisión».

**Cambio:** 2026-10-29. La ficha de work item ya no muestra los hitos efectivos ni las dependencias (spec 005, RF-60). La API las sigue ofreciendo; la pantalla para gestionarlas queda pendiente de otra spec.

**Cambio:** 2026-10-29. Al reabrir un work item se reinician su avance y sus horas restantes: RF-160.

**Cambio:** Aprobado (2026-10-28) · Issue #10. Alta rápida desde cualquier campo y en móvil: RF-140 se sustituye por RF-145 a RF-148.

**Cambio:** Aprobado (2026-10-28) · PR #12. Pesos de work items y peso por defecto por tipo: RF-54, RF-81 y RF-82 se sustituyen por RF-149 a RF-159.

## Contexto y objetivo
Kairos necesita gestionar proyectos con metodologías distintas (ágil, PM², PRINCE2, OKR…)
sin un modelo distinto por metodología. Como OpenProject, se usan piezas genéricas
configurables: plantillas, tipos de work item, estados, flujos, fases y campos personalizados.
Se corrigen dos debilidades de OpenProject:

1. Allí un **hito** es un work package de tipo «milestone» unido a las tareas por
   dependencias, así que «qué compone el hito X» es difícil de responder. En Kairos el hito
   es una **entidad propia del proyecto** que agrupa los work items que cuentan para él.
2. Allí los **OKR** se simulan con tipos de work package dentro de un proyecto. En Kairos
   los **objetivos** son una entidad propia que puede abarcar varios proyectos.

Jerarquía funcional: **Objetivos → Proyectos → Hitos → Actividades → Subactividades**.
Esta spec define esa jerarquía y su configuración. Los usuarios, roles, equipos y la
auditoría vienen de la spec 003; la gamificación consume los eventos que aquí se emiten.

## Usuarios / actores
Los roles son los cuatro de la spec 003. En esta spec:
- **Administrador** (`admin`): configura el catálogo global (metodologías, tipos, estados,
  flujos, fases, campos personalizados, plantillas) y puede hacer cualquier operación.
- **Líder** (`lead`): crea y gestiona proyectos y objetivos de los equipos de su ámbito.
- **Responsable de proyecto**: el usuario indicado como responsable de un proyecto, sea
  cual sea su rol (salvo `viewer`); gestiona ese proyecto.
- **Miembro** (`member`): trabaja en los work items de los proyectos en los que participa.
- **Observador** (`viewer`): solo lectura.
- **Módulo de gamificación** (spec futura): consume los eventos de cierre y cumplimiento.

## Conceptos
| Concepto | Definición |
|---|---|
| **Proyecto** | Unidad de trabajo con responsable, equipo, fechas, estado de ciclo de vida y salud. Es la única entidad «proyecto»; las metodologías no crean entidades nuevas. Puede tener un proyecto padre (portafolio → programa → proyecto). |
| **Equipo del proyecto** | Equipo de la spec 003 al que pertenece el proyecto. Determina el ámbito del `lead` y quién participa. |
| **Participante** | El responsable del proyecto o cualquier miembro del equipo del proyecto o de sus subequipos, a cualquier profundidad. |
| **Metodología** | Elemento de catálogo (PRINCE2, Scrum, PM²…) que agrupa definiciones de fases. Añadir una es configuración, no un cambio de esquema. |
| **Plantilla** | Proyecto marcado como plantilla. Crear un proyecto desde ella copia su configuración. |
| **Fase** | Etapa del ciclo de vida del proyecto (Inicio, Planificación…), definida por la metodología e instanciada en cada proyecto con fechas y gates de entrada y salida. |
| **Hito** | Punto de control o entregable con fecha objetivo. Pertenece a un solo proyecto, agrupa los work items que cuentan para él y puede contribuir a objetivos. **No** es un work item. |
| **Work item** | Unidad de trabajo ejecutable (actividad, subactividad, story, bug…; el tipo es configurable). Pertenece a un proyecto y puede tener un padre. |
| **Composición** | Relación padre-hijo entre work items. Distinta de la pertenencia a un hito y de las dependencias. |
| **Dependencia** | Relación no jerárquica entre dos work items: `blocks`, `precedes`, `relates`, `duplicates`. |
| **Hitos efectivos** | De un work item: sus hitos directos más los de todos sus ancestros, sin repetir. |
| **Objetivo** | Resultado que se quiere lograr (OKR), con alcance de organización, equipo o proyecto. Puede abarcar varios proyectos. |
| **Resultado clave** | Métrica medible de un objetivo: valor inicial, meta y valor actual, manual o derivado. |
| **Evento de dominio** | Hecho registrado por Kairos (`work_item.closed`, `milestone.completed`…) que otros módulos consumen. |

## Decisiones de diseño acordadas
Vienen del borrador del responsable de producto y **no se reabren**:
1. El hito es una entidad propia del proyecto, no un tipo de work item.
2. Un work item puede contar para varios hitos y un hito agrupa varios work items (N:M).
3. La pertenencia a un hito se guarda solo donde la asigna el usuario; los descendientes la
   heredan al consultar.
4. Los objetivos son una entidad propia fuera de la jerarquía de work items y pueden ligarse a
   varios proyectos, hitos y work items.
5. Tipos, estados y flujos son catálogos globales; cada proyecto activa los tipos que usa.
6. Ninguna metodología nueva requiere migraciones de esquema.

## Historias de usuario
- H1: Como administrador quiero definir metodologías, tipos, estados, flujos, fases y campos personalizados para que cada equipo trabaje con su método sin pedir cambios de código.
- H2: Como líder quiero crear un proyecto desde una plantilla PRINCE2 o Scrum para empezar con la configuración correcta.
- H3: Como responsable de proyecto quiero definir hitos y asignarles work items para saber qué falta para cada entrega y su avance.
- H4: Como responsable quiero asignar una épica a un hito y que sus subactividades cuenten sin asignarlas una a una.
- H5: Como miembro quiero crear actividades y subactividades, estimarlas y cambiar su estado según el flujo de mi equipo.
- H6: Como miembro quiero marcar que una actividad bloquea o precede a otra para que el equipo vea el orden del trabajo.
- H7: Como líder quiero definir objetivos con resultados clave ligados a hitos y work items de varios proyectos para ver su avance real.
- H8: Como observador quiero consultar proyectos, hitos, work items y objetivos sin poder cambiarlos.

## Requisitos funcionales (criterios de aceptación en EARS)

### Catálogo de configuración
- RF-1: EL SISTEMA permitirá a un administrador crear, renombrar, reordenar y retirar metodologías.
- RF-2: EL SISTEMA permitirá a un administrador definir, por metodología, fases ordenadas con nombre e indicadores de gate de entrada y de salida.
- RF-3: EL SISTEMA permitirá a un administrador crear, editar, reordenar y retirar tipos de work item con nombre y color.
- RF-4: EL SISTEMA permitirá a un administrador crear, editar, reordenar y retirar estados con nombre, color e indicador de estado cerrado.
- RF-5: EL SISTEMA tendrá en todo momento exactamente un estado marcado como inicial por defecto, que no estará cerrado.
- RF-6: EL SISTEMA permitirá a un administrador definir transiciones permitidas por tipo de work item, con estado de origen, estado de destino y, opcionalmente, el rol de la spec 003 al que se limitan.
- RF-7: EL SISTEMA permitirá a un administrador definir prioridades ordenadas, con exactamente una prioridad por defecto.
- RF-8: SI el nombre de una metodología, tipo, estado o prioridad ya existe entre los activos de su catálogo (sin distinguir mayúsculas ni espacios exteriores), ENTONCES EL SISTEMA no creará un duplicado y responderá con un código de conflicto.
- RF-9: CUANDO un administrador retire un elemento del catálogo en uso, EL SISTEMA conservará las referencias existentes y no permitirá elegirlo en usos nuevos.
- RF-10: SI un administrador intenta retirar el estado inicial por defecto o la prioridad por defecto, ENTONCES EL SISTEMA rechazará la operación.
- RF-11: EL SISTEMA traerá precargados los estados, tipos, prioridades y metodologías de ejemplo necesarios para la demo (Scrum y PRINCE2), editables como cualquier otro elemento.

### Campos personalizados
- RF-12: EL SISTEMA permitirá a un administrador definir campos personalizados para proyectos o para work items, con nombre, tipo y obligatoriedad.
- RF-13: EL SISTEMA admitirá en los campos personalizados los mismos tipos y reglas de validación que los atributos de usuario de la spec 003 (texto, número, fecha, booleano y selección; RF-95 a RF-97 y RF-107 de 003).
- RF-14: DONDE un campo personalizado de work item esté limitado a ciertos tipos, EL SISTEMA solo lo pedirá y aceptará en work items de esos tipos.
- RF-15: EL SISTEMA solo pedirá y aceptará en un proyecto los campos personalizados que ese proyecto tenga activados.
- RF-16: SI se crea o edita un proyecto o work item sin valor en un campo obligatorio activo que le aplique, ENTONCES EL SISTEMA rechazará la operación indicando el campo.
- RF-17: CUANDO un campo personalizado pase a ser obligatorio o se retire, EL SISTEMA se comportará como con los atributos de usuario (RF-99 y RF-103 a RF-106 de 003).

### Proyectos
- RF-18: EL SISTEMA guardará de cada proyecto nombre, identificador, descripción, responsable, equipo, metodología, fechas de inicio y fin, estado de ciclo de vida, salud y explicación de la salud.
- RF-19: EL SISTEMA exigirá un identificador de 2 a 10 caracteres, letras mayúsculas sin acentos y dígitos, que empiece por letra.
- RF-20: SI el identificador ya existe en otro proyecto, plantillas y archivados incluidos, ENTONCES EL SISTEMA responderá con un código de conflicto.
- RF-21: EL SISTEMA no permitirá cambiar el identificador de un proyecto después de crearlo.
- RF-22: EL SISTEMA admitirá como estado de ciclo de vida `activo`, `cerrado` o `archivado`.
- RF-23: EL SISTEMA admitirá como salud `en_curso`, `en_riesgo` o `fuera_de_curso`, independiente del ciclo de vida.
- RF-24: SI la fecha de fin de un proyecto es anterior a la de inicio, ENTONCES EL SISTEMA rechazará el cambio.
- RF-25: SI el responsable indicado está inactivo, dado de baja o solo tiene el rol `viewer`, ENTONCES EL SISTEMA rechazará el cambio.
- RF-26: EL SISTEMA permitirá que un proyecto tenga un proyecto padre.
- RF-27: SI asignar un padre crearía un ciclo o una profundidad mayor de 5 niveles, ENTONCES EL SISTEMA rechazará el cambio.
- RF-28: CUANDO se pida el subárbol de un proyecto, EL SISTEMA devolverá todos sus descendientes en una sola petición.
- RF-29: MIENTRAS un proyecto esté `cerrado`, EL SISTEMA no permitirá crear ni modificar sus hitos, fases ni work items, salvo para reabrir el proyecto.
- RF-30: MIENTRAS un proyecto esté `archivado`, EL SISTEMA lo ocultará de los listados por defecto y lo tratará como de solo lectura.
- RF-31: EL SISTEMA no ofrecerá borrado físico de proyectos; archivar sustituye al borrado.
- RF-32: CUANDO se active un tipo de work item en un proyecto, EL SISTEMA permitirá crear work items de ese tipo en él.
- RF-33: SI se intenta desactivar en un proyecto un tipo que tiene work items en ese proyecto, ENTONCES EL SISTEMA rechazará la operación.

### Plantillas
- RF-34: EL SISTEMA permitirá marcar un proyecto como plantilla y desmarcarlo.
- RF-35: MIENTRAS un proyecto sea plantilla, EL SISTEMA no lo mostrará en los listados de proyectos, no emitirá eventos de dominio por sus cambios y no permitirá ligarlo a objetivos.
- RF-36: CUANDO se cree un proyecto desde una plantilla, EL SISTEMA copiará su metodología, tipos activos, fases, campos personalizados activos y valores por defecto de esos campos.
- RF-37: DONDE quien crea el proyecto pida incluir el contenido de ejemplo, EL SISTEMA copiará además los hitos y work items de la plantilla, con su composición, dependencias internas y pertenencia a hitos, en estado inicial, sin asignado y con fechas desplazadas según la nueva fecha de inicio.
- RF-38: EL SISTEMA guardará en el proyecto la plantilla de la que salió.
- RF-39: CUANDO cambie una plantilla, EL SISTEMA no modificará los proyectos ya creados desde ella.

### Fases
- RF-40: CUANDO se cree un proyecto con metodología, EL SISTEMA instanciará en él las fases activas de esa metodología, en orden y sin fechas.
- RF-41: EL SISTEMA permitirá a quien gestione el proyecto fijar fechas de inicio y fin de cada fase dentro de las fechas del proyecto.
- RF-42: CUANDO quien gestione el proyecto marque superado un gate, EL SISTEMA guardará la fecha y el usuario.
- RF-43: EL SISTEMA no bloqueará ninguna operación por el estado de un gate; los gates son informativos.

### Hitos
- RF-44: EL SISTEMA guardará de cada hito su proyecto, nombre, descripción, fecha objetivo, fecha real de cumplimiento, estado, orden y, opcionalmente, una fase del mismo proyecto.
- RF-45: EL SISTEMA admitirá como estado de un hito `abierto`, `cumplido` o `cancelado`.
- RF-46: CUANDO un hito pase a `cumplido`, EL SISTEMA guardará la fecha real de cumplimiento, que por defecto será la fecha del día.
- RF-47: CUANDO un hito `cumplido` vuelva a `abierto`, EL SISTEMA borrará su fecha real de cumplimiento.
- RF-48: EL SISTEMA marcará como atrasado un hito `abierto` cuya fecha objetivo ya pasó, calculándolo al consultar.
- RF-49: EL SISTEMA permitirá cumplir un hito con work items abiertos y mostrará cuántos quedan abiertos.
- RF-50: EL SISTEMA permitirá ligar un work item a varios hitos y un hito a varios work items.
- RF-51: SI el hito y el work item son de proyectos distintos, ENTONCES EL SISTEMA rechazará la vinculación.
- RF-52: CUANDO se consulte un hito, EL SISTEMA devolverá sus work items efectivos (los ligados directamente y todos sus descendientes), cada uno una sola vez e indicando si es directo o heredado.
- RF-53: CUANDO se consulten los hitos de un work item, EL SISTEMA devolverá sus hitos efectivos (directos y de sus ancestros) sin repetir, indicando cuáles son heredados.
- ~~RF-54: EL SISTEMA calculará el avance de un hito como el porcentaje de sus work items efectivos en un estado cerrado.~~ Sustituida por RF-154 (pesos).
- RF-55: EL SISTEMA calculará las horas estimadas y restantes de un hito sumando las horas propias (no derivadas) de sus work items efectivos, contando cada uno una sola vez.
- RF-56: EL SISTEMA contará un work item ligado a dos hitos en el avance de ambos.
- RF-57: CUANDO se mueva un work item a otro padre, EL SISTEMA reflejará en la siguiente consulta los hitos que hereda del nuevo padre y dejará de reflejar los del anterior.
- RF-58: SI la fase indicada para un hito es de otro proyecto, ENTONCES EL SISTEMA rechazará el cambio.
- RF-59: EL SISTEMA no ofrecerá borrado físico de hitos con work items ligados u objetivos ligados; para eso está el estado `cancelado`.
- RF-60: CUANDO se borre un hito sin work items ni objetivos ligados, EL SISTEMA lo eliminará.

### Work items
- RF-61: EL SISTEMA guardará de cada work item su proyecto, tipo, estado, prioridad, título, descripción, autor, asignado, fechas de inicio y fin, horas estimadas, horas restantes, avance y padre opcional.
- RF-62: CUANDO se cree un work item, EL SISTEMA le asignará el siguiente número del contador de su proyecto y lo mostrará como `<identificador>-<número>` (por ejemplo `KAI-42`).
- RF-63: EL SISTEMA no reutilizará un número aunque se borre el work item, y dos altas simultáneas en el mismo proyecto recibirán números distintos.
- RF-64: CUANDO se cree un work item sin estado, EL SISTEMA le pondrá el estado inicial por defecto; sin prioridad, la prioridad por defecto.
- RF-65: SI el tipo del work item no está activo en su proyecto, ENTONCES EL SISTEMA rechazará el alta.
- RF-66: SI la fecha de fin es anterior a la de inicio, ENTONCES EL SISTEMA rechazará el cambio.
- RF-67: SI las horas estimadas o restantes son negativas o mayores de 10 000, o el avance está fuera de 0–100, ENTONCES EL SISTEMA rechazará el cambio.
- RF-68: SI el asignado está inactivo, dado de baja o solo tiene el rol `viewer`, ENTONCES EL SISTEMA rechazará la asignación.
- RF-69: CUANDO un usuario asignado se desactive o se dé de baja, EL SISTEMA conservará sus asignaciones existentes.
- RF-70: EL SISTEMA guardará la descripción como texto enriquecido con formato básico (títulos, listas, negrita, cursiva, enlaces, código y tablas) de hasta 50 000 caracteres.
- RF-71: EL SISTEMA nunca guardará ni devolverá en una descripción HTML o scripts ejecutables; los enlaces solo admitirán `http`, `https` y `mailto`.
- RF-72: SI el título está vacío o supera 255 caracteres, ENTONCES EL SISTEMA rechazará el cambio.
- RF-73: SI dos usuarios modifican el mismo work item partiendo de la misma versión, ENTONCES EL SISTEMA aceptará el primer cambio y rechazará el segundo con un código de conflicto de versión, sin perder datos.
- RF-74: EL SISTEMA aplicará la misma protección de conflicto de versión a proyectos, hitos y objetivos.

### Composición
- RF-75: EL SISTEMA permitirá que un work item tenga un padre del mismo proyecto.
- RF-76: SI el padre es de otro proyecto, ENTONCES EL SISTEMA rechazará el cambio.
- RF-77: SI asignar un padre crearía un ciclo, ENTONCES EL SISTEMA rechazará el cambio.
- RF-78: SI asignar un padre dejaría algún work item a más de 10 niveles de profundidad, ENTONCES EL SISTEMA rechazará el cambio.
- RF-79: CUANDO se pidan los descendientes o los ancestros de un work item, EL SISTEMA los devolverá en una sola petición.
- RF-80: EL SISTEMA calculará las horas estimadas y restantes derivadas de un work item como sus horas propias más las de todos sus descendientes.
- ~~RF-81: EL SISTEMA calculará el avance derivado de un work item con descendientes como la media de su avance y el de sus descendientes ponderada por horas estimadas, contando como 100 % los cerrados.~~ Sustituida por RF-153 (pesos).
- ~~RF-82: SI ningún work item del subárbol tiene horas estimadas, ENTONCES EL SISTEMA calculará el avance derivado como la media simple de sus avances.~~ Sustituida por RF-153 (pesos).
- RF-83: CUANDO cambien las horas, el avance, el estado o el padre de un work item, EL SISTEMA dejará actualizados los valores derivados de todos sus ancestros (y de los antiguos, si cambió de padre) al confirmar el cambio.
- RF-84: CUANDO un work item pase a un estado cerrado, EL SISTEMA pondrá su avance en 100 % y sus horas restantes en 0.
- RF-160: CUANDO un work item cerrado pase a un estado abierto, EL SISTEMA pondrá su avance en 0 % y sus horas restantes en sus horas estimadas (vacías si no las tiene), aunque la petición traiga otros valores.
- RF-85: SI se intenta borrar un work item que tiene hijos, ENTONCES EL SISTEMA rechazará el borrado.

### Estados y flujos
- RF-86: SI un tipo tiene transiciones definidas y el cambio de estado pedido no está permitido para ese tipo y los roles del usuario, ENTONCES EL SISTEMA lo rechazará con un código de transición no permitida.
- RF-87: MIENTRAS un tipo no tenga ninguna transición definida, EL SISTEMA permitirá cualquier cambio entre estados activos.
- RF-88: EL SISTEMA permitirá a un administrador cualquier cambio de estado, sin pasar por el flujo.
- RF-89: CUANDO se consulte un work item, EL SISTEMA indicará a qué estados puede pasarlo el usuario que consulta.

### Dependencias
- RF-90: EL SISTEMA permitirá crear dependencias `blocks`, `precedes`, `relates` y `duplicates` entre dos work items, del mismo o de distinto proyecto.
- RF-91: DONDE la dependencia sea `precedes`, EL SISTEMA admitirá un desfase en días entre −365 y 365.
- RF-92: SI la dependencia es de un work item consigo mismo, o entre un work item y uno de sus ancestros o descendientes, ENTONCES EL SISTEMA la rechazará.
- RF-93: SI ya existe la misma dependencia entre los dos work items, ENTONCES EL SISTEMA responderá con un código de conflicto.
- RF-94: SI una dependencia `blocks` o `precedes` cerraría un ciclo de dependencias del mismo tipo, ENTONCES EL SISTEMA la rechazará.
- RF-95: SI se intenta cerrar un work item bloqueado (`blocks`) por otro que no está cerrado, ENTONCES EL SISTEMA rechazará el cambio indicando el work item que lo bloquea.
- RF-96: EL SISTEMA no reprogramará fechas automáticamente por las dependencias `precedes`; indicará cuándo las fechas las incumplen.

### Objetivos y resultados clave
- RF-97: EL SISTEMA guardará de cada objetivo título, descripción, responsable, periodo (fecha de inicio y fin), alcance y estado.
- RF-98: EL SISTEMA admitirá como alcance `organizacion`, `equipo` (con un equipo de la spec 003) o `proyecto` (con un proyecto).
- RF-99: EL SISTEMA admitirá como estado de un objetivo `borrador`, `activo`, `cerrado` o `cancelado`.
- RF-100: SI se intenta activar un objetivo sin resultados clave, ENTONCES EL SISTEMA rechazará el cambio.
- RF-101: EL SISTEMA guardará de cada resultado clave título, unidad, valor inicial, meta, valor actual y modo de cálculo (`manual` o `derivado`).
- RF-102: SI la meta es igual al valor inicial, ENTONCES EL SISTEMA rechazará el resultado clave.
- RF-103: EL SISTEMA calculará el avance de un resultado clave como (actual − inicial) / (meta − inicial), limitado a 0–100 %.
- RF-104: EL SISTEMA permitirá ligar un objetivo a varios proyectos, que no sean plantillas.
- RF-105: EL SISTEMA permitirá ligar un objetivo a varios hitos, de uno o varios proyectos.
- RF-106: CUANDO se ligue un hito o un work item a un objetivo o a uno de sus resultados clave, EL SISTEMA ligará también su proyecto al objetivo si no lo estaba.
- RF-107: EL SISTEMA permitirá ligar un resultado clave a varios work items y a varios hitos.
- RF-108: SI se intenta ligar a un resultado clave un hito que no está ligado a su objetivo, ENTONCES EL SISTEMA rechazará la vinculación.
- RF-109: CUANDO se desligue un hito de un objetivo, EL SISTEMA lo desligará también de los resultados clave de ese objetivo.
- RF-110: MIENTRAS un resultado clave sea `derivado`, EL SISTEMA calculará su valor actual como inicial + (meta − inicial) × la fracción de sus elementos ligados cumplidos, donde un work item cumple si está cerrado y un hito si está `cumplido`, todos con el mismo peso.
- RF-111: MIENTRAS un resultado clave sea `derivado`, EL SISTEMA no permitirá editar su valor actual a mano.
- RF-112: MIENTRAS un resultado clave `derivado` no tenga elementos ligados, EL SISTEMA mostrará su avance como 0 %.
- RF-113: EL SISTEMA calculará el avance de un objetivo como la media simple del avance de sus resultados clave.
- RF-114: CUANDO se cierre o reabra un work item, o se cumpla o reabra un hito, EL SISTEMA reflejará el cambio en el avance de los resultados clave derivados en la siguiente consulta.
- RF-115: CUANDO se consulte un objetivo, EL SISTEMA mostrará sus proyectos, sus hitos contribuyentes (con fecha, estado y proyecto) y el avance de cada resultado clave.

### Eventos de dominio
- RF-116: CUANDO un work item pase de un estado no cerrado a uno cerrado, EL SISTEMA emitirá el evento `work_item.closed`.
- RF-117: CUANDO un work item pase de un estado cerrado a uno no cerrado, EL SISTEMA emitirá el evento `work_item.reopened`.
- RF-118: CUANDO un work item cambie entre dos estados cerrados, EL SISTEMA no emitirá ningún evento de cierre.
- RF-119: CUANDO un hito pase a `cumplido`, EL SISTEMA emitirá el evento `milestone.completed`; cuando deje de estarlo, `milestone.reopened`.
- RF-120: EL SISTEMA registrará cada evento en la misma transacción que el cambio que lo causa; si el cambio se deshace, no quedará evento.
- RF-121: EL SISTEMA entregará cada evento a sus consumidores al menos una vez, en el orden en que se registró, con un identificador único que permita ignorar duplicados.
- RF-122: Cada evento llevará su tipo, el momento, el usuario que lo causó, el proyecto y el work item o hito afectado, y (para work items) su asignado en ese momento.

### Permisos
- RF-123: EL SISTEMA añadirá los permisos de esta spec a la matriz única de la spec 003 (RF-59 de 003) y los comprobará como permisos, nunca por nombre de rol.
- RF-124: SI un usuario intenta una operación sin permiso o fuera de su ámbito, ENTONCES EL SISTEMA la rechazará con un código de prohibido sin modificar nada.
- RF-125: EL SISTEMA tendrá tests automáticos que recorran cada operación protegida de esta spec con cada rol, en y fuera de ámbito.

#### Matriz de permisos
«Ámbito» del `lead` = proyectos cuyo equipo es un equipo que lidera o uno de sus subequipos
(spec 003). «Gestor» de un proyecto = `admin`, `lead` con el proyecto en su ámbito o
responsable del proyecto. «Participante» según Conceptos.

| Permiso | admin | lead | member | viewer |
|---|---|---|---|---|
| Ver proyectos, fases, hitos, work items, dependencias y objetivos (no plantillas) | todos | todos | todos | todos |
| Ver y usar plantillas | sí | sí | no | no |
| Gestionar el catálogo (metodologías, fases, tipos, estados, flujos, prioridades, campos) | sí | no | no | no |
| Crear proyectos | sí | con equipo de su ámbito | no | no |
| Editar, cerrar, reabrir y archivar un proyecto; tipos y campos activos; fases y gates | sí | en su ámbito | si es responsable | no |
| Marcar y desmarcar plantillas | sí | no | no | no |
| Crear, editar, cumplir, cancelar y borrar hitos; ligar work items a hitos | sí | en su ámbito | si es responsable | no |
| Crear y editar work items, cambiar su estado, composición y dependencias | sí | en su ámbito | si participa | no |
| Borrar work items | sí | en su ámbito | si es responsable del proyecto o autor del work item sin hijos | no |
| Crear objetivos de alcance organización | sí | no | no | no |
| Crear objetivos de alcance equipo o proyecto | sí | en su ámbito | no | no |
| Editar objetivos y resultados clave, ligar proyectos, hitos y work items | sí | si el alcance está en su ámbito | si es responsable del objetivo | no |
| Actualizar el valor actual de un resultado clave manual | sí | si el alcance está en su ámbito | si es responsable del objetivo | no |

- RF-126: SI un usuario responsable de un objetivo o proyecto pierde los permisos que le daba ese papel (por ejemplo, pasa a `viewer`), ENTONCES EL SISTEMA dejará de concederle esas operaciones en el plazo de la ventana de permisos de la spec 003 (RF-44 de 003).
- RF-127: SI un `member` intenta ligar a un work item un hito u objetivo, ENTONCES EL SISTEMA aplicará el permiso de gestión del hito u objetivo, no el del work item.

### Listados y búsqueda
- RF-128: EL SISTEMA listará proyectos paginados (25 por defecto, máximo 100) con filtros por texto, equipo (incluidos subequipos), responsable, ciclo de vida, salud y metodología.
- RF-129: EL SISTEMA listará los work items de un proyecto paginados (50 por defecto, máximo 200) con filtros por tipo, estado, abierto/cerrado, prioridad, asignado, hito efectivo, texto en título e identificador.
- RF-130: EL SISTEMA devolverá el árbol completo de work items de un proyecto (con identificador, título, tipo, estado, asignado, avance derivado y padre) en una sola petición.
- RF-131: EL SISTEMA listará objetivos paginados con filtros por estado, alcance, periodo y responsable.
- RF-132: SI un filtro tiene un valor de formato inválido, ENTONCES EL SISTEMA responderá con el código de filtro inválido.
- RF-133: EL SISTEMA permitirá abrir un work item por su identificador legible (`KAI-42`).

### Auditoría
- RF-134: EL SISTEMA auditará con `audit` de la spec 003, en la misma transacción, toda creación, modificación, cambio de estado, borrado y vinculación de catálogo, proyectos, fases, hitos, work items, dependencias, objetivos y resultados clave.
- RF-135: EL SISTEMA no guardará en la auditoría el texto completo de las descripciones, solo que cambiaron.

### Interfaz
- RF-136: EL SISTEMA ofrecerá pantallas de: listado de proyectos, alta desde plantilla, ficha de proyecto (datos, fases, hitos con avance y árbol de work items), ficha de hito, ficha de work item (con hitos efectivos, dependencias, hijos y transiciones posibles), objetivos (listado y ficha) y administración del catálogo.
- RF-137: EL SISTEMA solo mostrará en la interfaz las acciones que el usuario puede hacer, según lo que indique la API para cada elemento.
- RF-138: EL SISTEMA mostrará todos los textos desde el catálogo de traducciones y traducirá los errores por código (spec 001 y 003).

#### Edición en tabla
- RF-139: EL SISTEMA permitirá editar en la propia tabla, celda a celda y sin botón de guardar, los campos simples de los listados de proyectos (nombre, responsable, salud, inicio y fin), hitos (nombre, fase y fecha objetivo), work items (título, tipo, estado, prioridad, asignado y fin) y del catálogo (nombre, color, orden, cerrado y por defecto). Cada celda se guarda al confirmarla y solo cambia ese campo.
- ~~RF-140: EL SISTEMA ofrecerá al final de esas tablas una fila de alta rápida: se escribe el nombre, se completan en la misma fila los datos obligatorios mínimos (identificador y equipo en proyectos, tipo en work items) y se crea con Enter. La fila queda abierta y vacía para el siguiente. En el árbol de work items, cada fila ofrece la misma alta rápida para un hijo.~~ Sustituida por RF-145 a RF-148 (Issue #10).
- RF-141: EL SISTEMA solo dejará editar las celdas que el usuario puede cambiar según lo que indique la API para cada fila (RF-137). Las columnas calculadas (avance, conteos, ciclo de vida) no se editan, y la celda de estado solo ofrece las transiciones permitidas por el flujo (RF-87).
- RF-142: CUANDO una celda no se pueda guardar, EL SISTEMA mostrará junto a ella el texto del error por su código, conservará el valor escrito y permitirá restaurar el anterior con Esc. SI el error es un conflicto de versión, ENTONCES ofrecerá recargar.
- RF-143: EL SISTEMA permitirá moverse entre celdas editables con las flechas, empezar a editar con Enter o escribiendo, confirmar con Enter o Tab y cancelar con Esc.
- RF-144: EL SISTEMA mantendrá el alta de proyectos desde plantilla en su propia pantalla (RF-36 a RF-38); la fila rápida crea proyectos en blanco.
- RF-145: EL SISTEMA ofrecerá al final de esas tablas una fila de alta rápida: se escribe el nombre y se completan en la misma fila los datos obligatorios mínimos (identificador y equipo en proyectos, tipo en work items). Tras crear, la fila queda abierta y vacía para el siguiente, con el foco en el nombre; Esc la cierra sin crear. En el árbol de work items, cada fila ofrece la misma alta rápida para un hijo.
- RF-146: CUANDO el usuario pulse Enter en cualquier campo de la fila de alta rápida (nombre, textos, selectores, fechas, color y casillas), o pulse el botón «Crear» que la fila muestra al final, EL SISTEMA creará el elemento. Mientras se crea, el botón queda deshabilitado y un segundo Enter o pulsación no crea otro.
- RF-147: SI falta el nombre u otro dato obligatorio de la fila, ENTONCES EL SISTEMA no creará el elemento, llevará el foco al primer campo que falte y mostrará en la fila el aviso de campo obligatorio. SI la API rechaza el alta, ENTONCES mostrará en la fila el texto del error por su código y conservará lo escrito.
- RF-148: EL SISTEMA mostrará el botón «Crear» y los campos de la fila de alta rápida sin desplazar la tabla en horizontal, también en pantallas de 400 px de ancho.

### Pesos
- RF-149: EL SISTEMA guardará de cada work item un peso entero de 0 a 100. Un work item de peso 0 es de seguimiento: se muestra, se asigna y se cierra como los demás, pero no cuenta en ningún avance. Puede cambiar el peso quien puede modificar el work item (RF-123 a RF-127).
- RF-150: EL SISTEMA permitirá a un administrador fijar en cada tipo de work item un peso por defecto entero de 0 a 100. Los tipos que no lo tengan fijado tendrán 1.
- RF-151: CUANDO se cree un work item sin indicar peso, EL SISTEMA le pondrá el peso por defecto de su tipo. CUANDO cambie el peso por defecto de un tipo, o el tipo de un work item, EL SISTEMA no cambiará el peso de los work items existentes.
- RF-152: SI el peso de un work item o el peso por defecto de un tipo no es un entero de 0 a 100, ENTONCES EL SISTEMA rechazará el cambio.
- RF-153: EL SISTEMA calculará el avance derivado de un work item con descendientes como la media de su avance y el de sus descendientes ponderada por su peso, contando como 100 % los cerrados. SI todo el subárbol tiene peso 0, ENTONCES lo calculará como la media simple de sus avances.
- RF-154: EL SISTEMA calculará el avance de un hito como la suma de los pesos de sus work items efectivos en un estado cerrado entre la suma de los pesos de todos sus work items efectivos, en porcentaje. SI esa suma es 0, ENTONCES el avance será 0 %. Los conteos de work items cerrados y totales del hito siguen contando todos, también los de peso 0.
- RF-155: CUANDO se emita `work_item.closed`, EL SISTEMA incluirá en él el peso del work item en ese momento, para que la gamificación lo use como base.
- RF-156: EL SISTEMA permitirá editar el peso de un work item en el árbol (RF-139) y en su ficha, y el peso por defecto de un tipo en la tabla de tipos del catálogo, e indicará junto al campo que el peso 0 es de seguimiento.
- RF-157: CUANDO se copie el contenido de ejemplo de una plantilla (RF-37), EL SISTEMA conservará el peso de cada work item copiado.
- RF-158: CUANDO cambie el peso de un work item, EL SISTEMA dejará actualizados los valores derivados de todos sus ancestros al confirmar el cambio, como en RF-83.
- RF-159: EL SISTEMA seguirá sumando las horas estimadas y restantes de los work items de peso 0 a las de su hito (RF-55) y a las derivadas de sus ancestros (RF-80): el peso solo afecta al avance.

## Dependencias con otras specs (contrato, no se implementa aquí)
- **Spec 003 (usuarios):** responsables, autores y asignados se referencian por el id interno del usuario. Se reutilizan los equipos y su jerarquía, la matriz de permisos, la auditoría y el formato de errores.
- **Gamificación (spec futura):** consume `work_item.closed`, `work_item.reopened`, `milestone.completed` y `milestone.reopened` (RF-116 a RF-122). Decide cuántos XP o monedas da cada evento, a quién y cómo se compensa una reapertura. Esta spec no toca XP ni monedas.
- **Kanban y tiempo real (spec futura):** usarán los estados, transiciones y el control de versión (RF-73) de esta spec. Esta spec no difunde cambios por WebSocket.

## Requisitos no funcionales
- **Tamaño de diseño:** 500 proyectos, 50 000 work items en total, 5 000 en un proyecto, 1 000 work items efectivos en un hito, 200 objetivos con 10 resultados clave cada uno, profundidad máxima de 10 niveles en work items y 5 en proyectos.
- **Rendimiento** (entorno local, con el tamaño de diseño): listados paginados < 300 ms; árbol completo de un proyecto de 5 000 work items < 500 ms; ficha de hito con avance < 200 ms; ficha de objetivo con avance < 300 ms; un cambio de estado o de padre, incluida la actualización de derivados, < 200 ms.
- **Consultas acotadas:** ninguna petición ejecuta un número de consultas que crezca con el número de elementos devueltos.
- **Integridad:** las reglas de mismo proyecto, ausencia de ciclos y unicidad se garantizan también ante peticiones concurrentes, no solo en la validación previa.
- **Seguridad:** toda operación pasa por la sesión, CSRF y permisos de la spec 003. Los identificadores que llegan en la petición (proyecto, padre, hito, objetivo, usuario) se validan contra el ámbito del usuario. Las descripciones se sanean en el servidor (RF-71). Los listados tienen un tamaño máximo de página.
- **Auditoría:** RF-134 y RF-135.
- **Idioma:** español.

## Casos límite
- **Work item con el mismo hito directo y heredado:** cuenta una vez y se muestra como directo (RF-52, RF-53).
- **Padre e hijo ligados al mismo hito:** cada uno cuenta una vez en el avance; las horas del hito usan las horas propias, así que no se duplican (RF-55).
- **Mover un subárbol a otro padre:** los hitos heredados cambian con él; los directos se conservan (RF-57). Los derivados del padre antiguo y del nuevo se recalculan (RF-83).
- **Altas simultáneas en un proyecto:** números distintos, sin huecos salvo por borrados o altas fallidas (RF-63).
- **Cierre de un work item con hijos abiertos:** se permite; el avance derivado del padre lo refleja.
- **Cierre de un work item bloqueado:** se rechaza mientras el bloqueante siga abierto (RF-95).
- **Cambio entre dos estados cerrados** («resuelto» → «rechazado»): sin evento (RF-118).
- **Cerrar, reabrir y volver a cerrar:** se emiten tres eventos; la gamificación decide cómo compensar (RF-116, RF-117).
- **Hito cumplido con work items abiertos:** se permite y se informa (RF-49).
- **Hito cancelado:** no cuenta como cumplido en los resultados clave derivados.
- **Resultado clave derivado sin elementos:** avance 0 % (RF-112).
- **Meta menor que el valor inicial** (por ejemplo, reducir incidencias de 40 a 10): la fórmula de RF-103 la admite.
- **Proyecto cerrado con objetivos ligados:** el objetivo lo sigue mostrando; sus work items y hitos siguen contando.
- **Responsable o asignado desactivado:** conserva sus asignaciones (RF-69); no se le asigna trabajo nuevo (RF-25, RF-68).
- **Tipo retirado del catálogo con work items:** los existentes lo conservan; no se crean nuevos de ese tipo (RF-9).
- **Estado retirado:** los work items que lo tienen lo conservan; no se puede pasar a él.
- **Dependencia entre proyectos donde el usuario solo gestiona uno:** necesita poder editar el work item de origen; el de destino solo debe ser visible.
- **Plantilla con contenido de ejemplo y fechas:** las fechas se desplazan respecto a la nueva fecha de inicio (RF-37).
- **Dos usuarios editan el mismo work item:** el segundo recibe conflicto de versión (RF-73).
- **Hito cuyos work items efectivos tienen todos peso 0:** avance 0 %; los conteos siguen mostrándolos (RF-154).
- **Subárbol con todo a peso 0:** avance derivado como media simple (RF-153).
- **Hijo de peso 0 cerrado bajo un padre abierto:** el avance del padre no cambia (RF-153).
- **Cambio de tipo de un work item:** conserva su peso (RF-151).
- **Datos existentes al aplicar el cambio:** todos los work items y tipos quedan con peso 1. El avance derivado de los padres con hijos de horas distintas puede cambiar, porque deja de ponderarse por horas (RF-153); aceptado al aprobar el cambio.
- **Enter mantenido o doble pulsación de «Crear» en la fila rápida:** se crea un solo elemento (RF-146).
- **Enter con un selector desplegado:** el navegador lo usa para elegir la opción; el siguiente Enter crea (RF-146).

## Fuera de alcance
- Tablero kanban, diagrama de Gantt y vistas de calendario.
- Difusión de cambios en tiempo real.
- Reprogramación automática por dependencias.
- Mover work items entre proyectos.
- Registro de tiempo, costes y presupuestos.
- Historial de cambios visible en la ficha del work item (los cambios se auditan, RF-134).
- Comentarios, menciones, adjuntos y vigilantes de work items.
- Notificaciones.
- XP, monedas y niveles (spec de gamificación).
- Permisos por proyecto distintos de la matriz (roles por proyecto, miembros de proyecto aparte del equipo).
- Proyectos o work items privados: todo lo que no es plantilla es visible para cualquier usuario activo.
- Pesos en los resultados clave: los work items tienen peso (RF-149), pero los resultados clave derivados siguen contando todos sus elementos por igual (RF-110). Los hitos no tienen peso.
- Aprobaciones formales de gates.
- Importación y exportación (CSV, MS Project, Jira).
- Borrado físico de proyectos.
- Un Enter en cualquier parte de la pantalla que cree el elemento de la fila rápida: en la tabla, Enter empieza a editar la celda enfocada (RF-143); el alta se hace desde la fila (RF-146).

## Criterios de finalización
- RF-1 a RF-135 y RF-149 a RF-159 (sin las sustituidas RF-54, RF-81 y RF-82) cubiertos por tests automáticos del backend en verde, incluida la matriz de permisos (RF-125) y pruebas de concurrencia para RF-63, RF-73 y la ausencia de ciclos.
- Prueba de rendimiento con datos del tamaño de diseño que cumple los umbrales de los requisitos no funcionales.
- RF-136 a RF-139, RF-141 a RF-148 y RF-156 con tests del frontend en verde.
- Demo manual:
  1. Como admin, revisar las metodologías Scrum y PRINCE2 precargadas con sus fases, y definir un flujo para el tipo «Bug» que impida pasar de «Nuevo» a «Cerrado» directamente.
  2. Crear las plantillas «Scrum» y «PRINCE2», esta con hitos y work items de ejemplo.
  3. Como `lead` de BI, crear el proyecto `BIR` desde la plantilla PRINCE2 con contenido de ejemplo y el proyecto `BIS` desde la de Scrum, sin cambiar el esquema; comprobar fases y tipos de cada uno.
  4. Crear una épica con tres subactividades, ligar la épica al hito M1 y comprobar que las tres aparecen en M1 como heredadas y que el avance de M1 cuenta las cuatro.
  5. Ligar una de las subactividades también al hito M2 y comprobar que aparece en ambos y suma al avance de los dos.
  6. Intentar ligar un work item de `BIR` a un hito de `BIS` y ver el rechazo.
  7. Mover una subactividad fuera de la épica y comprobar que deja de estar en M1.
  8. Como `member` participante, intentar pasar un «Bug» de «Nuevo» a «Cerrado» y ver el rechazo; hacerlo por el flujo permitido y ver `work_item.closed` en los eventos.
  9. Crear una dependencia `blocks` e intentar cerrar el work item bloqueado; ver el rechazo.
  10. Como `viewer`, comprobar que se ve todo y no se puede modificar nada.
  11. Crear un objetivo de equipo con un resultado clave derivado, ligar el hito M1 de `BIR` y work items de `BIS`; comprobar que ambos proyectos quedan ligados y que el objetivo muestra los hitos contribuyentes.
  12. Cumplir M1 y comprobar el evento `milestone.completed` y el nuevo avance del resultado clave.
  13. Abrir el mismo work item en dos pestañas, guardar en las dos y ver el conflicto de versión en la segunda.
  14. Consultar la auditoría y ver los cambios anteriores.

## Decisiones de la revisión (2026-10-23)
Tomadas por el agente a petición del responsable de producto, priorizando rendimiento y seguridad:
- **Numeración:** RF-1 a RF-138 correlativos, en lugar de RF-P1, RF-M1… del borrador, como en las specs anteriores.
- **Estado del proyecto:** el borrador mezclaba salud (`en_curso`, `en_riesgo`, `fuera_de_curso`) con ciclo de vida (`cerrado`). Se separan (RF-22, RF-23).
- **Borrado:** el borrador borraba en cascada proyectos, hitos y work items. Eso rompería el ledger inmutable de la gamificación y la auditoría. No hay borrado físico de proyectos (se archivan) ni de hitos ligados (se cancelan); un work item solo se borra si no tiene hijos (RF-31, RF-59, RF-85).
- **D2 — herencia de hitos:** calculada al consultar, no guardada en cada nivel (decisión acordada 3).
- **D3 — jerarquía:** se exige el resultado (subárbol y ancestros en una petición, RF-28 y RF-79, con límites de profundidad 10 y 5) y los umbrales de rendimiento; el plan elige entre tabla de cierre y consulta recursiva. Recomendación para el plan: consulta recursiva sobre el padre con índices, porque con el tamaño de diseño basta y evita mantener una tabla redundante que puede desincronizarse; se pasa a tabla de cierre solo si las pruebas de rendimiento no cumplen.
- **D4 — campos personalizados:** mismas reglas y tipos que los atributos de usuario de la spec 003 (RF-13, RF-17) para no tener dos modelos; el almacenamiento lo decide el plan, siguiendo el de 003 salvo que los filtros no cumplan los umbrales.
- **D5 — jerarquía de proyectos:** mismo patrón que los work items (D3).
- **D6 — pesos en resultados clave:** sin pesos en esta iteración; se deja fuera de alcance en lugar de crear una columna sin uso.
- **Valores derivados:** fórmula explícita (RF-80 a RF-82). Las horas de un hito usan horas propias para no contar dos veces padre e hijos (RF-55).
- **Visibilidad:** todo lo que no es plantilla lo ven todos los usuarios activos (equipo interno de 20–50 personas). Los permisos de escritura usan los roles y el ámbito de equipo de 003, más los papeles de responsable y participante. No hay roles por proyecto.
- **Flujos:** un tipo sin transiciones definidas las permite todas; con alguna, solo las definidas. El administrador puede forzar cualquier transición (auditada).
- **Dependencias:** `blocks` impide cerrar mientras el bloqueante esté abierto; `precedes` es informativa. Se prohíben ciclos en `blocks` y `precedes`.
- **Eventos:** también los de reapertura, y registro en la misma transacción con entrega al menos una vez (patrón outbox), para que la gamificación no pierda ni invente puntos.
- **Concurrencia:** control de versión optimista en proyectos, hitos, work items y objetivos (RF-73, RF-74).
- **Descripciones:** texto enriquecido saneado en el servidor, sin HTML ni scripts, con límite de tamaño (RF-70, RF-71).
- **Identificador de proyecto:** formato fijo e inmutable, porque forma parte de las referencias `KAI-42` (RF-19, RF-21).
- **Asignaciones:** no se asigna trabajo nuevo a usuarios inactivos, dados de baja o `viewer`; las existentes se conservan (RF-25, RF-68, RF-69). Esto resuelve lo que la spec 003 dejaba a esta.
- **Metodología:** pasa a ser un catálogo propio en lugar de un texto libre, para poder colgar de ella las fases.
- **Resultados clave y work items:** un work item ligado cuenta por sí mismo (cerrado o no), sin heredar a sus descendientes, para que el avance de un resultado clave no cambie al reorganizar el árbol.

## Edición en tabla (2026-10-28)
A petición del responsable de producto, con el diseño «Tablas editables» como referencia:
- **Una sola forma de editar y crear:** las tablas sustituyen a las listas con formulario por fila y a los formularios de alta encima de la lista (RF-139, RF-140). Las fichas siguen existiendo para la descripción, las relaciones y los campos que no caben en una celda.
- **Sin dependencias nuevas:** la paginación, el orden y los filtros ya los hace la API, así que no hace falta TanStack Table; las celdas, la fila rápida y la navegación con teclado son componentes propios sobre la tabla del tema.
- **Guardado por celda:** los `PATCH` reemplazan todos los campos editables, así que la celda lee el elemento antes de guardar y solo cambia su campo; el control de versión sigue protegiendo contra escrituras simultáneas (RF-73, RF-142).
- **Permisos por fila:** el listado de proyectos devuelve en cada fila el mismo `can` que la ficha (cambio aditivo de la API), para no ofrecer celdas que la API rechazaría (RF-141).

## Alta rápida en móvil (2026-10-28, Issue #10)
La fila rápida solo creaba al pulsar Enter en el nombre: tras elegir equipo, tipo o fase había que volver al nombre, y en el móvil no había forma de crear. Se crea con Enter desde cualquier campo de la fila o con un botón «Crear» visible (RF-146), que se ve también en pantallas estrechas (RF-148). De las dos opciones del issue (botón o Enter global) se toman el botón y el Enter dentro de la fila, porque un Enter global chocaría con el de las celdas (RF-143).

## Pesos (2026-10-28)
A petición del responsable de producto:
- **Peso por work item, con valor por defecto en el tipo** (RF-149 a RF-151). El valor del tipo se copia al crear, así un cambio en el catálogo no altera el avance de lo que ya existe.
- **El peso sustituye a las horas como ponderación del avance** (RF-153, RF-154). Las horas quedan como información y siguen sumándose (RF-55, RF-80).
- **Peso 0 para tareas de seguimiento:** se registran sin mover el avance del proyecto.
- **XP:** el peso viaja en `work_item.closed` (RF-155); cómo se convierte en XP lo decide la spec de gamificación.

## Dudas abiertas
- Ninguna bloqueante. El tamaño de diseño (500 proyectos, 50 000 work items) es una estimación del agente; confírmalo o corrígelo al aprobar.
