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
- `POST /api/auth/register`: Registro de nuevos usuarios.
- `GET /api/users/me`: Información del usuario autenticado.

### Cámaras
- `GET /api/cameras`: Listado de cámaras enriquecido con telemetría en vivo desde MediaMTX.
- `POST /api/cameras`: Registro de una nueva cámara.
- `PUT /api/cameras/:id`: Modificación de parámetros de captura y umbrales de movimiento.
- `DELETE /api/cameras/:id`: Eliminación de una cámara.
- `GET /api/cameras/search?name=...`: Búsqueda de cámaras por nombre.

### Alertas y Detección de Movimiento
- `POST /api/alerts`: Ingesta de nueva alerta con snapshot forense (requiere `token` o JWT).
- `GET /api/alerts/stream`: Flujo en tiempo real Server-Sent Events (SSE).
- `GET /api/alerts`: Consulta paginada de alertas con filtros de estado y cámara.
- `GET /api/alerts/unread-count`: Contador de alertas pendientes de acuse de recibo.
- `POST|PUT /api/alerts/:id/ack`: Acuse de recibo de una alerta.
- `POST|PUT /api/alerts/ack-all`: Acuse de recibo masivo de todas las alertas pendientes.
- `DELETE /api/alerts/:id`: Eliminación de una alerta y borrado físico de su imagen del disco.
- `DELETE /api/alerts`: Limpieza total del historial y borrado físico de todas las capturas.
- `GET /api/motion/config`: Configuración JSON para el daemon de detección de movimiento.

---

## ⚙️ Variables de Entorno

| Variable | Descripción | Valor por Defecto |
|---|---|---|
| `PORT` | Puerto HTTP del servidor Gin | `8080` |
| `DB_HOST` | Host de PostgreSQL | `localhost` |
| `DB_PORT` | Puerto de PostgreSQL | `5432` |
| `DB_USER` | Usuario de base de datos | `admin` |
| `DB_PASSWORD` | Contraseña de base de datos | `admin` |
| `DB_NAME` | Nombre de la base de datos | `bigbrother` |
| `DB_SSLMODE` | Modo SSL de conexión PostgreSQL | `disable` |
| `JWT_SECRET` | Clave secreta HMAC para firma de tokens JWT | *(Clave predeterminada)* |
| `INTERNAL_TOKEN` | Token secreto para agentes y microservicios internos | `internal-token-pro` |
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