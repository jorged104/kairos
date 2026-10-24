# ADR 0003 — Descripciones como documento JSON con lista blanca

**Estado:** aceptada (2026-10-23) · **Spec:** [004-proyectos](../specs/004-proyectos/spec.md)

## Contexto
Proyectos, hitos, work items y objetivos tienen descripciones con formato básico (RF-70). Las
escribe cualquier participante y las leen todos los usuarios, así que guardar HTML abriría la
puerta a XSS almacenado. RF-71 exige que nunca se guarde ni se devuelva HTML o script ejecutable,
y que los enlaces solo admitan `http`, `https` y `mailto`.

## Decisión
- La descripción se guarda como **documento JSON de TipTap/ProseMirror** (`jsonb`), nunca HTML.
- `internal/richtext` lo valida en el servidor con una **lista blanca** de nodos, marcas y
  atributos (con sus valores), y límites de 256 KiB, 50 000 caracteres y 20 niveles. Todo lo que
  no está en la lista se rechaza con `invalid_rich_text`; los atributos desconocidos solo se
  aceptan si son `null`.
- El documento que se guarda **se reconstruye** a partir del árbol validado, así que nada fuera de
  la lista llega a la BD. Los enlaces solo conservan `href`; `target` y `rel` los pone el visor.
- Se extrae el texto plano a `description_text` para buscar. La auditoría solo guarda
  `description_changed`.
- El frontend lo pinta con TipTap en modo lectura (`RichTextView`), nunca con `v-html`. El editor
  y el visor comparten las extensiones de la lista blanca (`features/richtext/extensions.ts`).

## Alternativas descartadas
- **HTML saneado en el servidor:** obliga a mantener un sanitizador HTML (nueva dependencia) y
  cualquier fallo suyo es XSS; el JSON con lista blanca falla cerrado.
- **Markdown:** necesita un renderizador que también hay que configurar para no emitir HTML, y
  las tablas no son estándar.

## Consecuencias
- Añadir una extensión al editor exige añadirla también a la lista blanca del servidor; si no,
  el guardado falla con `invalid_rich_text`. Los tests de ambos lados lo detectan.
- Las descripciones no se pueden pintar fuera de TipTap sin un renderizador propio del JSON.
