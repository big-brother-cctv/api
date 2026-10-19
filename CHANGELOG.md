# Changelog

## [0.12.1](https://github.com/big-brother-cctv/api/compare/v0.12.0...v0.12.1) (2026-10-19)


### Bug Fixes

* set camera status to standby when inactive in camera handler ([c84c3f9](https://github.com/big-brother-cctv/api/commit/c84c3f94649923c11a68da8c848a7496fe52aac7))

## [0.12.0](https://github.com/big-brother-cctv/api/compare/v0.11.0...v0.12.0) (2026-10-19)


### Features

* expand API README with comprehensive features, endpoints, and environment variables documentation ([cfda9c6](https://github.com/big-brother-cctv/api/commit/cfda9c6ba36488c3f5ae52105443fa9bee161a38))

## [0.11.0](https://github.com/big-brother-cctv/api/compare/v0.10.0...v0.11.0) (2026-10-18)


### Features

* add unified motion detection configuration endpoint and camera model fields ([4c36d06](https://github.com/big-brother-cctv/api/commit/4c36d06c81d44aaa2181dad3c9e976fb0297cdd7))

## [0.10.0](https://github.com/big-brother-cctv/api/compare/v0.9.0...v0.10.0) (2026-10-18)


### Features

* disable response buffering for SSE alerts and enrich camera status with active paths ([1f75d51](https://github.com/big-brother-cctv/api/commit/1f75d51ebd4fa44db117e9c5427ac9ce8410e65e))

## [0.9.0](https://github.com/big-brother-cctv/api/compare/v0.8.1...v0.9.0) (2026-10-18)


### Features

* add base64 image support and cooldown bypass to alerts handler alongside PUT routes for acknowledgments ([b680002](https://github.com/big-brother-cctv/api/commit/b6800023edcde8d6ade925a997fd6c7d03e8cdc1))

## [0.8.1](https://github.com/big-brother-cctv/api/compare/v0.8.0...v0.8.1) (2026-10-18)


### Bug Fixes

* prevent demoting or disabling the primary admin user and ensure initial admin state on startup ([fde57c4](https://github.com/big-brother-cctv/api/commit/fde57c41c03a5dbbb5a02db2572ca7d8c6398df9))

## [0.8.0](https://github.com/big-brother-cctv/api/compare/v0.7.0...v0.8.0) (2026-10-16)


### Features

* add alert management system with real-time SSE streaming and snapshot storage ([f2c15f3](https://github.com/big-brother-cctv/api/commit/f2c15f363bd71ee605af6dd258ec739e00fc004d))

## [0.7.0](https://github.com/big-brother-cctv/api/compare/v0.6.0...v0.7.0) (2026-10-16)


### Features

* add security headers, rate limiting, role-based access control, and enhanced validation ([222b47c](https://github.com/big-brother-cctv/api/commit/222b47c7e9610e042953e85b7643f1ad1b760b6f))

## [0.6.0](https://github.com/big-brother-cctv/api/compare/v0.5.1...v0.6.0) (2026-10-16)


### Features

* **users:** add role and permissions management with deletion safety protections ([0afb9b2](https://github.com/big-brother-cctv/api/commit/0afb9b2a88709dccfdf87b08583a021af939225d))

## [0.5.1](https://github.com/big-brother-cctv/api/compare/v0.5.0...v0.5.1) (2026-10-16)


### Bug Fixes

* **build:** track cmd/server and correct gitignore rule ([4319fa1](https://github.com/big-brother-cctv/api/commit/4319fa1b674dbddd77168174c60d213632cb2d4f))

## [0.5.0](https://github.com/big-brother-cctv/api/compare/v0.4.0...v0.5.0) (2026-10-16)


### Features

* add release-please workflow and update CI to separate testing and docker validation ([d6a1c99](https://github.com/big-brother-cctv/api/commit/d6a1c9970ea4a4430bd9d03f4262b33fe1c53a8b))
