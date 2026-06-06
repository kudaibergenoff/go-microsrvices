# Microservices Platform

Учебно-продакшен-подобная платформа микросервисов на Go: API Gateway, gRPC-сервисы, Keycloak, Kafka, observability-стек и деплой в Docker Compose / Kubernetes.

## Содержание

- [Архитектура](#архитектура)
- [Стек технологий](#стек-технологий)
- [Сервисы и порты](#сервисы-и-порты)
- [Быстрый старт](#быстрый-старт)
- [API](#api)
- [Аутентификация и RBAC](#аутентификация-и-rbac)
- [Kafka-события](#kafka-события)
- [Observability](#observability)
- [Разработка](#разработка)
- [Kubernetes и Helm](#kubernetes-и-helm)
- [CI/CD](#cicd)
- [Структура проекта](#структура-проекта)

## Архитектура

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │ REST
       ▼
┌─────────────────────────────────────────────────────────┐
│              API Gateway (Fiber) :8080                   │
│  Rate limit · CORS · JWT (Keycloak) · OpenTelemetry      │
└──┬──────┬──────┬──────┬──────┬──────┬───────────────────┘
   │ gRPC │ gRPC │ gRPC │ gRPC │ gRPC │ gRPC
   ▼      ▼      ▼      ▼      ▼      ▼
 Auth   User  Product Order  Media  Notification
   │      │      │      │      │         │
   ▼      ▼      ▼      ▼      ▼         ▼
 AuthDB UserDB ProdDB OrdDB  MinIO    MailHog
   │      │               │               ▲
   └──────┴───────────────┴───────────────┘
                    Kafka
```

**Паттерны:**
- API Gateway — единая точка входа (REST → gRPC)
- Event-driven — асинхронные события через Kafka
- Saga — распределённые транзакции при создании/отмене заказов
- OIDC — аутентификация через Keycloak

## Стек технологий

| Категория | Технологии |
|-----------|------------|
| Язык | Go 1.23+ |
| HTTP | Fiber |
| RPC | gRPC, Protocol Buffers |
| БД | PostgreSQL 16 |
| Auth | Keycloak 24 (OIDC/JWT) |
| Messaging | Apache Kafka 3.9 |
| Storage | MinIO (S3-compatible) |
| Email | MailHog (dev) |
| Metrics | Prometheus, Grafana |
| Tracing | Jaeger, OpenTelemetry |
| Logging | Loki, Promtail |
| Deploy | Docker Compose, Kubernetes, Helm |

## Сервисы и порты

### Приложения

| Сервис | HTTP | gRPC | Описание |
|--------|------|------|----------|
| gateway | 8080 | — | REST API, middleware, маршрутизация |
| auth-service | 8081 | 50051 | Регистрация, логин, refresh token |
| user-service | 8082 | 50052 | Профиль пользователя, CRUD (admin) |
| notification-service | 8083 | 50056 | Email-уведомления (Kafka + gRPC) |
| media-service | 8084 | 50055 | Upload/delete файлов (MinIO) |
| product-service | 8085 | 50053 | CRUD товаров, резервирование склада |
| order-service | 8086 | 50054 | Заказы, saga, Kafka events |

### Инфраструктура

| Сервис | Порт | URL |
|--------|------|-----|
| Keycloak | 8180 | http://localhost:8180 |
| Kafka | 9092 | localhost:9092 |
| Kafka UI | 8090 | http://localhost:8090 |
| MinIO API | 9000 | http://localhost:9000 |
| MinIO Console | 9001 | http://localhost:9001 |
| MailHog UI | 8025 | http://localhost:8025 |
| Prometheus | 9090 | http://localhost:9090 |
| Grafana | 3000 | http://localhost:3000 (admin/admin) |
| Jaeger UI | 16686 | http://localhost:16686 |
| Loki | 3100 | http://localhost:3100 |

### Базы данных (PostgreSQL)

| БД | Порт (host) |
|----|-------------|
| auth-db | 5433 |
| user-db | 5434 |
| product-db | 5435 |
| order-db | 5436 |
| keycloak-db | — (только внутри compose) |

## Быстрый старт

### Требования

- Docker и Docker Compose
- Make (опционально)
- Go 1.23+ (для локальной разработки)
- protoc + protoc-gen-go, protoc-gen-go-grpc (для генерации proto)

### Запуск

```bash
# Сборка и запуск всех сервисов
make up

# или напрямую
docker compose up -d --build
```

Первый запуск Keycloak может занять несколько минут (healthcheck с `start_period: 300s`).

### Проверка

```bash
# Health check gateway
curl http://localhost:8080/health

# Регистрация пользователя
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"123456","name":"Test User"}'

# Список товаров
curl http://localhost:8080/api/products
```

### Остановка

```bash
make down        # остановить контейнеры
make clean       # остановить и удалить volumes
```

## API

Базовый URL: `http://localhost:8080`

### Auth (публичные)

| Метод | Путь | Описание |
|-------|------|----------|
| POST | `/api/auth/register` | Регистрация |
| POST | `/api/auth/login` | Вход |
| POST | `/api/auth/refresh` | Обновление access token |
| POST | `/api/auth/validate` | Проверка токена |

**Register / Login** — ответ:

```json
{
  "user_id": 1,
  "token": "<access_token>",
  "refresh_token": "<refresh_token>"
}
```

### Users (требуется `Authorization: Bearer <token>`)

| Метод | Путь | Доступ | Описание |
|-------|------|--------|----------|
| GET | `/api/users/me` | user | Профиль текущего пользователя |
| PUT | `/api/users/me` | user | Обновление профиля |
| PUT | `/api/users/me/password` | user | Смена пароля |
| POST | `/api/users/me/avatar` | user | Загрузка аватара (multipart) |
| DELETE | `/api/users/me` | user | Удаление аккаунта |
| GET | `/api/users?page=1&limit=10` | admin | Список пользователей |
| GET | `/api/users/:id` | admin | Пользователь по ID |
| PUT | `/api/users/:id` | admin | Обновление пользователя |
| DELETE | `/api/users/:id` | admin | Удаление пользователя |

### Products (публичные)

| Метод | Путь | Описание |
|-------|------|----------|
| POST | `/api/products` | Создание товара |
| GET | `/api/products` | Список товаров |
| GET | `/api/products/search?q=...` | Поиск |
| GET | `/api/products/:id` | Товар по ID |
| PUT | `/api/products/:id` | Обновление |
| DELETE | `/api/products/:id` | Удаление |

### Orders (публичные)

| Метод | Путь | Описание |
|-------|------|----------|
| POST | `/api/orders` | Создание заказа (saga) |
| GET | `/api/orders?user_id=1` | Заказы пользователя |
| GET | `/api/orders/:id` | Заказ по ID |
| PUT | `/api/orders/:id/status` | Обновление статуса |

Статусы заказа: `pending`, `confirmed`, `cancelled`, `completed`.

### Media (публичные)

| Метод | Путь | Описание |
|-------|------|----------|
| POST | `/api/media/upload` | Upload файла (form-data: `file`, `folder`) |
| DELETE | `/api/media/:key` | Удаление файла |

Поддерживаемые типы: JPEG, PNG, GIF, WebP. Максимальный размер: 10 MB.

### Notifications (публичные)

| Метод | Путь | Описание |
|-------|------|----------|
| POST | `/api/notifications/email` | Отправка email |
| POST | `/api/notifications/order` | Уведомление о заказе |

### Системные

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/health` | Health check gateway |
| GET | `/metrics` | Prometheus metrics |

Postman-коллекции: `microservices.json`, `postman_collection.json`.

## Аутентификация и RBAC

Аутентификация реализована через **Keycloak** (realm `microservices`):

- Gateway проверяет JWT через OIDC (`go-oidc`)
- Роли извлекаются из `realm_access.roles`
- Роли: `user` (по умолчанию), `admin`

**Keycloak Admin Console:** http://localhost:8180 — `admin` / `admin`

**Rate limiting на gateway:** 100 запросов / 60 сек на IP (настраивается через `RATE_LIMIT_MAX`, `RATE_LIMIT_WINDOW`).

Пример запроса с токеном:

```bash
TOKEN="<access_token>"

curl http://localhost:8080/api/users/me \
  -H "Authorization: Bearer $TOKEN"
```

## Kafka-события

| Topic | Producer | Consumer | Описание |
|-------|----------|----------|----------|
| `user.registered` | auth-service | notification-service, user-service | Welcome email, синхронизация |
| `order.events` | order-service | — | order.created, confirmed, cancelled, completed |

### Saga (Order Service)

1. Проверка существования товара (gRPC → product-service)
2. Резервирование склада (`ReserveStock`)
3. Создание заказа в БД
4. При ошибке — компенсация (`ReleaseStock`)
5. При отмене confirmed-заказа — возврат склада

## Observability

| Инструмент | Назначение |
|------------|------------|
| **Prometheus** | Метрики сервисов (`/metrics`), Kafka consumer lag |
| **Grafana** | Дашборды: Microservices Overview, Service Logs |
| **Jaeger** | Distributed tracing (gateway → gRPC, Kafka) |
| **Loki + Promtail** | Централизованные логи контейнеров (JSON) |

OpenTelemetry интегрирован в gateway, auth-service и user-service с propagation через gRPC и Kafka.

## Разработка

### Генерация protobuf

```bash
make proto
```

Контракты лежат в `proto/`:

- `auth.proto`, `user.proto`, `product.proto`, `order.proto`, `media.proto`, `notification.proto`

### Полезные команды

```bash
make build          # docker compose build
make logs           # логи всех сервисов
make auth-logs      # логи auth-service
make user-logs      # логи user-service
make gateway-logs   # логи gateway
make tidy           # go mod tidy для основных сервисов
```

### Конфигурация

Каждый сервис использует `.env` файл (уже настроен для Docker Compose). Основные переменные gateway:

```env
APP_PORT=8080
AUTH_GRPC_HOST=auth-service
AUTH_GRPC_PORT=50051
KEYCLOAK_URL=http://keycloak:8080
KEYCLOAK_REALM=microservices
JAEGER_ENDPOINT=jaeger:4318
RATE_LIMIT_MAX=100
RATE_LIMIT_WINDOW=60
```

## Kubernetes и Helm

Манифесты: `k8s/base/`  
Helm chart: `helm/microservices/`  
Подробная документация: [`docs/k8s-guideline.md`](docs/k8s-guideline.md)

```bash
# Применить манифесты
make k8s-apply

# Статус
make k8s-status

# Helm
make helm-install
make helm-upgrade
make helm-uninstall

# Сборка образов для k8s
make k8s-build REGISTRY=localhost:5000 TAG=latest
```

## CI/CD

GitHub Actions (`.github/workflows/ci.yml`):

1. **Lint** — golangci-lint для всех 7 сервисов
2. **Unit Tests** — `go test -race` с coverage
3. **Integration Tests** — docker compose up + curl (register, products)
4. **Docker Build** — multi-stage build с GHA cache

## Структура проекта

```
.
├── auth-service/           # Аутентификация, Keycloak, Kafka producer
├── user-service/           # Пользователи, Kafka consumer, аватары
├── gateway-service/        # API Gateway (REST → gRPC)
├── product-service/        # Каталог товаров, склад
├── order-service/          # Заказы, saga, Kafka events
├── media-service/          # Файлы (MinIO)
├── notification-service/   # Email (SMTP/MailHog)
├── proto/                  # Protobuf-контракты
├── keycloak/               # Realm import
├── prometheus/             # Prometheus config
├── grafana/                # Dashboards и provisioning
├── loki/                   # Loki + Promtail config
├── k8s/base/               # Kubernetes manifests
├── helm/microservices/     # Helm chart
├── docs/                   # Документация
├── docker-compose.yml
├── Makefile
└── TODO.md                 # Roadmap (выполнен)
```

## Лицензия

MIT.
