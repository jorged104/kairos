# spec-generator — instalación

La skill es una carpeta autocontenida con dos ficheros: `SKILL.md` (las
instrucciones, con frontmatter YAML) y `spec-template.md` (la plantilla).
El contenido es agnóstico de la herramienta; solo cambia dónde se coloca.

## Claude Code

Ya está instalada en este proyecto. Para tenerla en todos tus proyectos:

```bash
cp -R .claude/skills/spec-generator ~/.claude/skills/
```

Se activa sola cuando pides una spec, o a mano con `/spec-generator`.

## opencode

Mismo formato de skill, distinta ruta. Lo ideal es un symlink
`.opencode/skill/spec-generator` → `.claude/skills/spec-generator`.
Si tu sistema no maneja symlinks (Windows sin permisos), copia la carpeta:

```bash
mkdir -p .opencode/skill && cp -R .claude/skills/spec-generator .opencode/skill/
```

Global en vez de por proyecto: `~/.config/opencode/skill/`.

## Mantenimiento

`SKILL.md` es la única fuente de verdad: enlaza o referencia esta carpeta
desde otras herramientas en vez de duplicarla. Edita solo ese fichero. Si en algún
sitio tienes una copia en vez de un enlace, recuerda sincronizarla.