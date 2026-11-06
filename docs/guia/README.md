# Guía de Kairos

Cómo instalar, configurar y mantener Kairos al día.

| Guía | Para quién |
|---|---|
| [Instalación local](instalacion-local.md) | Quien quiere probar Kairos o desarrollar en su máquina |
| [Instalación en un servidor](instalacion-servidor.md) | Quien quiere dejar Kairos en marcha para un equipo u organización |
| [Configuración](configuracion.md) | Referencia de las variables de entorno y de lo que se ajusta desde la pantalla Sistema |
| [Actualización](actualizacion.md) | Pasar a una versión nueva, con copia de seguridad y vuelta atrás |

## Piezas de una instalación

```
navegador ─► proxy inverso con HTTPS ─► web (nginx: frontend + proxy) ─► api (Go) ─► PostgreSQL
                                                                           └─► S3 (opcional, archivos)
                                                                           └─► SMTP (opcional, correo)
```

- **web**: sirve el frontend y reenvía `/api`, `/ws` y `/health` a la API.
- **api**: un único binario sin estado. Lee su configuración de variables de entorno.
- **PostgreSQL 17**: guarda todos los datos, también los archivos de las páginas si no hay S3.
- **migrate**: aplica las migraciones de la base de datos. Nunca se ejecutan solas al arrancar la API.

Otros documentos del repositorio:
- [`AGENTS.md`](../../AGENTS.md): reglas para trabajar en el código.
- [`docs/specs/`](../specs/): qué hace cada funcionalidad y por qué.
- [`docs/adr/`](../adr/): decisiones de arquitectura.
- [`deploy/preview/README.md`](../../deploy/preview/README.md): el entorno de previsualización que se despliega en cada push a `main`.
