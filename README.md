# Big Brother CCTV — Core API (Go)

Backend central y API REST de alto rendimiento desarrollado en **Go (Golang)** con **Gin Web Framework** y **GORM** para PostgreSQL. Gestiona la autenticación, la configuración y topología universal de cámaras, el despacho de eventos Server-Sent Events (SSE) en tiempo real y el ciclo de vida del almacenamiento forense de instantáneas.

---

## 🌟 Características

- **Autenticación y Control de Acceso**:
  - JWT stateless con roles (`admin`, `operator`, `viewer`) para usuarios del panel web.
  - Autenticación por token interno (`INTERNAL_TOKEN`) para agentes edge y el daemon de detección de movimiento sin requerir sesión interactiva.
- **Topología Universal de Cámaras**:
  - CRUD de cámaras en tiempo real (`/api/cameras`) con persistencia en PostgreSQL.
  - Sincronización automática de estado y codecs consultando la API v3 de **MediaMTX** (`http://mediamtx:9997/v3/paths/list`).
  - Compatibilidad directa con cámaras IP (RTSP/HTTP Pull), emisiones externas (RTMP/RTSP Push), agentes USB locales y generadores sintéticos.
- **Configuración Centralizada de Detección de Movimiento**:
  - Endpoint `/api/motion/config` que genera dinámicamente la configuración para el daemon multi-hilo de `motion-detector` (umbrales por cámara, frames y URLs RTSP).
- **Despacho de Eventos en Tiempo Real (SSE)**:
  - Canal Server-Sent Events en `/api/alerts/stream` sin almacenamiento intermedio de respuesta (`X-Accel-Buffering: no`), permitiendo alertas visuales y sonoras instantáneas en el frontend.
  - Heartbeat periódico para mantener la conexión activa a través de proxies e ingress controllers.
- **Gestión Forense de Alertas e Instantáneas**:
  - Ingesta de capturas vía `multipart/form-data` o imagen codificada en `base64`.
  - Almacenamiento persistente en `./data/alerts` servido estáticamente en `/api/alerts/snapshots/`.
  - **Eliminación Física Garantizada**: Al eliminar una alerta individual (`DELETE /api/alerts/:id`) o limpiar el historial completo (`DELETE /api/alerts`), el servidor borra físicamente el archivo del disco para evitar acumulación de espacio residual.

---

## 📡 Endpoints de la API

### Autenticación
- `POST /api/auth/login`: Inicio de sesión de usuario y emisión de token JWT.
- `POST /api/auth/register`: Auto-registro **desactivado por defecto** (`ALLOW_REGISTRATION=true` para activarlo). Siempre crea usuarios `viewer`.
- `POST /api/media/session`: (Bearer) Emite la cookie HttpOnly `bb_media` que autoriza SSE, snapshots y streams.
- `DELETE /api/media/session`: Borra la cookie de medios (logout).
- `GET /api/media/publish-credentials`: (admin/operator) Usuario y clave para publicar desde móviles/OBS en MediaMTX.
- `POST /internal/mediamtx/auth`: Endpoint `authHTTPAddress` de MediaMTX (`authMethod: http`). Fuera de `/api` para que el nginx público no lo exponga.
- `GET /api/media/check`: 204 si la petición trae credenciales de medios válidas. Lo usa nginx (`auth_request`) para `/stream/` y `/whep/`.
- `GET /api/users/me`: Información del usuario autenticado.

### Cámaras
- `GET /api/cameras`: Listado de cámaras enriquecido con telemetría en vivo desde MediaMTX.
- `POST /api/cameras`: Registro de una nueva cámara.
- `POST /api/cameras/heartbeat`: Heartbeat y auto-registro de agentes (Internal Token u operador/admin).
- `PUT /api/cameras/:id`: Modificación de parámetros de captura y umbrales de movimiento.
- `DELETE /api/cameras/:id`: Eliminación de una cámara.
- `GET /api/cameras/search?name=...`: Búsqueda de cámaras por nombre.

### Alertas y Detección de Movimiento
- `POST /api/alerts`: Ingesta de nueva alerta con snapshot forense (solo Internal Token: cabecera `X-Internal-Token`, `token` en form o query).
- `GET /api/alerts/stream`: Flujo en tiempo real Server-Sent Events (SSE). Requiere Bearer o cookie `bb_media`.
- `GET /api/alerts/snapshots/<fichero>`: Capturas. Requiere Bearer o cookie `bb_media`.
- `GET /api/alerts`: Consulta paginada de alertas con filtros de estado y cámara.
- `GET /api/alerts/unread-count`: Contador de alertas pendientes de acuse de recibo.
- `POST|PUT /api/alerts/:id/ack`: Acuse de recibo de una alerta.
- `POST|PUT /api/alerts/ack-all`: Acuse de recibo masivo de todas las alertas pendientes.
- `DELETE /api/alerts/:id`: Eliminación de una alerta y borrado físico de su imagen del disco.
- `DELETE /api/alerts`: Limpieza total del historial y borrado físico de todas las capturas.
- `GET /api/motion/config`: Configuración JSON para el daemon de detección de movimiento (Internal Token).

---

## ⚙️ Variables de Entorno

> Genera secretos con `openssl rand -base64 48`.

| Variable | Descripción | Valor por Defecto |
|---|---|---|
| `APP_ENV` | `production` o `dev`. En `dev` se permiten secretos por defecto (con aviso) | `production` |
| `PORT` | Puerto HTTP del servidor Gin | `8080` |
| `DB_HOST` | Host de PostgreSQL | `localhost` |
| `DB_PORT` | Puerto de PostgreSQL | `5432` |
| `DB_USER` | Usuario de base de datos | `admin` |
| `DB_PASSWORD` | Contraseña de base de datos | `admin` |
| `DB_NAME` | Nombre de la base de datos | `bigbrother` |
| `DB_SSLMODE` | Modo SSL de conexión PostgreSQL | `disable` |
| `JWT_SECRET` | **Obligatorio.** Clave HMAC de los JWT (≥32 caracteres). La API no arranca si falta o es un valor publicado | — |
| `INTERNAL_TOKEN` | **Obligatorio.** Token de agentes y motion-detector (≥24 caracteres). Equivale a rol admin | — |
| `MEDIA_PUBLISH_TOKEN` | Clave para publicar en MediaMTX (`rtsp://publisher:<clave>@host:8554/<cámara>`, `rtmp://host:1935/<cámara>?user=publisher&pass=<clave>`) | — |
| `MEDIA_READ_TOKEN` | Clave de lectura de MediaMTX (motion-detector y nginx de Telescreen) | — |
| `ADMIN_INITIAL_PASSWORD` | Password del usuario `admin` al crear la BD vacía. Si no se define, se genera y se imprime una vez en el log | *(aleatoria)* |
| `ALERT_RETENTION_DAYS` | Días que se conservan alertas y snapshots (0 = sin límite). Limpieza cada hora | `30` |
| `ALERT_MAX_STORAGE_MB` | Espacio máximo de snapshots; al superarlo se borran las alertas más antiguas (0 = sin límite) | `2048` |
| `ALLOW_REGISTRATION` | Habilita `POST /api/auth/register` (crea solo `viewer`) | `false` |
| `MEDIAMTX_API_URL` | URL de la API v3 de MediaMTX | `http://localhost:9997` |
| `MEDIAMTX_RTSP_HOST` | Host RTSP para generar URLs de streaming | `localhost` |

---

## 🚀 Comandos Rápidos

```bash
# Compilar binario en local
go build -o bin/server ./cmd/server

# Ejecutar el servidor
go run ./cmd/server

# Ejecutar pruebas unitarias
go test -v ./...
```
*(Nota: Para entornos locales de desarrollo, asegúrate de tener una instancia de PostgreSQL en marcha, por ejemplo mediante `docker compose up -d postgres`).*