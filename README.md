# Big Brother CCTV - Core API (Go)

Backend central y API REST de alto rendimiento desarrollado en **Go (Golang)** con Gin y GORM para PostgreSQL.

## Características
- Autenticación JWT stateless (`/api/auth/login`, `/api/auth/register`).
- Soporte para token interno (`INTERNAL_TOKEN`) para agentes y cámaras físicas sin login de usuario.
- Gestión y configuración de cámaras en tiempo real (`/api/cameras`).
- Búsqueda de cámaras por nombre (`/api/cameras/search?name=...`).
- Gestión de usuarios y perfil actual (`/api/users`, `/api/users/me`).
- Proxy de subida de imágenes y vídeos para CDN / content manager (`/api/upload/image`, `/api/upload/video`).
- Health checks compatibles (`/actuator/health`).
- CORS configurado para clientes web y credenciales.

## Variables de Entorno
| Variable | Descripción | Valor por Defecto |
|---|---|---|
| `PORT` | Puerto HTTP del servidor | `8080` |
| `DB_HOST` | Host de PostgreSQL | `localhost` |
| `DB_PORT` | Puerto de PostgreSQL | `5432` |
| `DB_USER` | Usuario de PostgreSQL | `admin` |
| `DB_PASSWORD` | Contraseña de PostgreSQL | `admin` |
| `DB_NAME` | Base de datos | `bigbrother` |
| `DB_SSLMODE` | Modo SSL de conexión a DB | `disable` |
| `JWT_SECRET` | Clave secreta HMAC-SHA256 | `8Q4n5v1Qw8v9wF3n...` |
| `INTERNAL_TOKEN` | Token para comunicación con agente cámara | `internal-token-dev` |
| `CONTENT_MANAGER_URL` | URL del servicio content manager | `http://content-manager:8181` |

## Comandos Rápidos
```bash
# Compilar binario
go build -o bin/server ./cmd/server

# Ejecutar en desarrollo
go run ./cmd/server

# Ejecutar tests
go test -v ./...
```