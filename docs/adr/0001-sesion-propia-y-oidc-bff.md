# ADR 0001 — Sesión propia y OIDC con el backend como cliente (BFF)

**Estado:** aceptada (2026-10-22) · **Spec:** [003-usuarios](../specs/003-usuarios/spec.md)

## Contexto
Kairos necesita login local, por un IdP OIDC o ambos (`AUTH_MODE`), y que la autorización
refleje en ≤ 120 s cambios de rol, equipo o estado hechos en Kairos aunque el IdP siga
autenticando a la persona.

## Decisión
- **El backend es el cliente OIDC** (confidencial, Authorization Code + PKCE, `state` de un solo
  uso en BD y `nonce`). El navegador nunca ve tokens del IdP y no se guardan tras el login.
- **Sesión propia:** cookie `kairos_session` `HttpOnly; SameSite=Lax` (y `Secure` fuera de
  desarrollo) con un JWT HS256 que solo lleva `sub` (= `users.id`) y `exp` (8 h fijas).
- **Roles, permisos, ámbito y estado se leen de BD** en cada petición, con una caché por usuario
  de 60 s que se invalida en la propia instancia al cambiar algo.
- **CSRF:** `SameSite=Lax` más comprobación de `Origin`/`Referer` contra `PUBLIC_URL` en los
  métodos que modifican.
- **Identidad separada del usuario:** `user_identities(issuer, subject)`; ni el `sub` ni el email
  son claves foráneas.

## Alternativas descartadas
- **SPA como cliente público con tokens del IdP en el navegador:** expone tokens a XSS, obliga a
  validar en cada petición contra la JWKS y deja la autorización en manos del IdP.
- **Roles en el JWT de sesión:** un cambio no se aplicaría hasta que caducase la sesión.
- **Sesiones en BD con revocación inmediata:** fuera de alcance de la spec; la ventana de 120 s
  y la desactivación cubren el caso. Se puede añadir sin cambiar el contrato de la cookie.

## Consecuencias
- El logout en el IdP se pide con `client_id` (sin `id_token_hint`, porque no se guarda): Keycloak
  muestra una pantalla de confirmación antes de volver a Kairos.
- Rate limit y caché viven en memoria: válidos con una instancia de API. Con varias habrá que
  moverlos a BD o a un almacén compartido.
- En desarrollo, la API en contenedor descubre Keycloak por `keycloak:8080`
  (`OIDC_DISCOVERY_URL`) mientras el issuer sigue siendo `localhost:8180`.
