# Microservices Project — Roadmap

## 1. Инфраструктура

### 1.1 Observability (Prometheus + Grafana)
- [x] Добавить Prometheus для сбора метрик со всех сервисов (prometheus:9090, 3 targets: up)
- [x] Добавить Grafana с дашбордами (grafana:3000, dashboard "Microservices Overview" — 8 панелей)
- [x] Экспортировать метрики из каждого Go-сервиса (prometheus/client_golang, /metrics endpoint)
- [x] Метрики Kafka (consumer lag, throughput)

### 1.2 Distributed Tracing (Jaeger)
- [x] Добавить Jaeger в docker-compose (jaeger:16686, OTLP на 4318)
- [x] Интегрировать OpenTelemetry SDK в каждый сервис (gateway, auth-service, user-service)
- [x] Трейсинг через gateway → auth-service (gRPC spans подтверждены)
- [x] Трейсинг через gateway → user-service (gRPC otelgrpc interceptor)
- [x] Propagation trace context через gRPC (otelgrpc.NewClientHandler / NewServerHandler)
- [x] Propagation trace context через Kafka

### 1.3 Centralized Logging (Loki + Grafana)
- [x] Добавить Loki + Promtail в docker-compose (loki:3100, promtail с docker_sd)
- [x] Собирать логи всех контейнеров (14 сервисов в Loki)
- [x] Дашборд в Grafana для просмотра логов (dashboard "Service Logs" — 3 панели)
- [x] Structured logging (JSON) во всех сервисах (gateway, auth, user)

### 1.4 Rate Limiting на Gateway
- [x] Rate limiter middleware (по IP — Fiber limiter, FixedWindow)
- [x] Конфигурируемые лимиты (RATE_LIMIT_MAX, RATE_LIMIT_WINDOW через env)
- [x] Ответ 429 Too Many Requests с Retry-After header

---

## 2. Новые сервисы

### 2.1 Notification Service
- [x] Kafka consumer — слушает events (user.registered)
- [x] Отправка email (SMTP через MailHog, порт 8025 — UI для просмотра писем)
- [x] Шаблоны уведомлений (Welcome email с HTML)
- [x] gRPC API для отправки уведомлений вручную

### 2.2 File/Media Service
- [x] MinIO (S3-совместимое хранилище) в docker-compose (minio:9000, консоль :9001)
- [x] Upload/download файлов через REST API (POST /api/media/upload, DELETE /api/media/:key)
- [x] Загрузка аватарок пользователей (folder=avatars, валидация типов, лимит 10MB)
- [x] Интеграция с user-service (сохранение avatar_url)

### 2.3 Product/Order Service (e-commerce)
- [x] Product service — CRUD товаров, категории, поиск (product-service:8085, product-db:5435)
- [x] Order service — создание заказов, статусы, история (order-service:8086, order-db:5436)
- [x] Saga pattern для распределённых транзакций
- [x] Kafka events: order.created, order.confirmed, order.cancelled, order.completed (topic: order.events)

---

## 3. Функциональность

### 3.1 Auth Middleware на Gateway
- [x] Middleware проверки токена на каждый защищённый роут
- [x] Автоматический вызов ValidateToken
- [x] Прокидывание user_id и email в контекст запроса
- [x] Публичные роуты без проверки (login, register)

### 3.2 Refresh Token
- [x] Эндпоинт POST /api/auth/refresh
- [x] Хранение refresh token (httpOnly cookie или body)
- [x] Ротация refresh token при использовании

### 3.3 RBAC (Role-Based Access Control)
- [x] Извлечение ролей из Keycloak JWT (realm_access.roles)
- [x] Middleware проверки ролей на gateway
- [x] Admin-only эндпоинты (управление пользователями, и др.)
- [x] Назначение ролей через Keycloak Admin API

### 3.4 User Service CRUD
- [x] GET /api/users/me — профиль текущего пользователя
- [x] PUT /api/users/me — обновление профиля
- [x] PUT /api/users/me/password — смена пароля (через Keycloak)
- [x] DELETE /api/users/me — удаление аккаунта
- [x] GET /api/users (admin) — список пользователей с пагинацией

---

## 4. DevOps

### 4.1 CI/CD (GitHub Actions)
- [x] Линтинг (golangci-lint) на каждый PR
- [x] Unit тесты
- [x] Integration тесты с docker-compose
- [x] Сборка Docker образов и push в registry

### 4.2 Docker оптимизация
- [x] Multi-stage build caching (go mod download слой)
- [x] .dockerignore файлы для уменьшения контекста
- [x] Health checks для всех сервисов

### 4.3 Kubernetes
- [x] Kubernetes манифесты (Deployment, Service, ConfigMap, Secret)
- [x] Helm chart
- [x] Ingress controller
- [x] Horizontal Pod Autoscaler
