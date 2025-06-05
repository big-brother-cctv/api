# API

This is the main REST API for the Big Brother project, built with Spring Boot.

## Features

- User authentication and registration with JWT
- Camera management (CRUD operations)
- Error handling with custom responses
- Integration with other services in the Big Brother ecosystem

## Requirements

- Java 17+
- Maven 3.8+
- PostgreSQL database

## Getting Started

Clone the repository and navigate to the `api` directory:

```sh
git clone https://github.com/big-brother-cctv/api.git
cd api
```

Install dependencies and build the project:

```sh
./mvnw clean install
```

### Running the API

Start the API locally:

```sh
./mvnw spring-boot:run
```

By default, the API runs at [http://localhost:8080](http://localhost:8080).

### Docker

To build and run the API with Docker:

```sh
docker build -t bigbrother-api .
docker run -p 8080:8080 bigbrother-api
```

## Endpoints

- `POST /api/auth/login` - User login (returns JWT)
- `POST /api/auth/register` - User registration
- `GET /api/cameras` - List all cameras
- `POST /api/cameras` - Create a new camera
- `PUT /api/cameras/{id}` - Update a camera
- `DELETE /api/cameras/{id}` - Delete a camera

## Camera Model

Example of camera fields:

```json
{
  "id": 1,
  "name": "Front Door",
  "device": "/dev/video0",
  "resolution": "1920x1080",
  "fps": "30",
  "postUrl": "http://localhost:8000/upload",
  "codec": "h264",
  "preset": "ultrafast",
  "tune": "zerolatency",
  "buffer": "1000",
  "rotation": "0"
}
```

## Error Handling

Custom error responses are provided, for example:

```json
{
  "timestamp": "2024-06-07T12:34:56.789",
  "message": "Camera name is required",
  "details": "Invalid camera name"
}
```

## License

This project is licensed under the [GNU GPL v3](https://github.com/big-brother-cctv/api/blob/main/LICENSE).