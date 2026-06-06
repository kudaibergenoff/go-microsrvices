# Kubernetes Guideline — Microservices Platform

## Оглавление

1. [Архитектура проекта](#1-архитектура-проекта)
2. [Основные концепции Kubernetes](#2-основные-концепции-kubernetes)
3. [Структура манифестов проекта](#3-структура-манифестов-проекта)
4. [Namespace](#4-namespace)
5. [ConfigMap и Secrets](#5-configmap-и-secrets)
6. [Deployments](#6-deployments)
7. [Services (сетевое взаимодействие)](#7-services-сетевое-взаимодействие)
8. [Ingress](#8-ingress)
9. [Persistent Volumes](#9-persistent-volumes)
10. [Horizontal Pod Autoscaler (HPA)](#10-horizontal-pod-autoscaler-hpa)
11. [Kafka в Kubernetes](#11-kafka-в-kubernetes)
12. [Keycloak в Kubernetes](#12-keycloak-в-kubernetes)
13. [Helm Chart](#13-helm-chart)
14. [Команды для работы](#14-команды-для-работы)
15. [Troubleshooting](#15-troubleshooting)
16. [Best Practices](#16-best-practices)

---

## 1. Архитектура проекта

```
┌─────────────────────────────────────────────────────────────────┐
│                        INGRESS (NGINX)                         │
│  api.microservices.local  │  keycloak.microservices.local       │
└──────────┬────────────────┴────────────────┬────────────────────┘
           │                                 │
           ▼                                 ▼
┌─────────────────┐                ┌──────────────────┐
│     Gateway     │                │     Keycloak     │
│   (порт 8080)   │                │   (порт 8080)    │
└───┬───┬───┬───┬─┘                └──────────────────┘
    │   │   │   │
    │   │   │   │  gRPC
    ▼   ▼   ▼   ▼
┌──────┐┌──────┐┌────────┐┌───────┐┌───────┐┌──────────────┐
│ Auth ││ User ││Product ││ Order ││ Media ││ Notification │
│ :8081││ :8082││  :8085 ││ :8086 ││ :8084 ││    :8083     │
└──┬───┘└──┬───┘└───┬────┘└───┬───┘└───┬───┘└──────┬───────┘
   │       │        │         │        │           │
   ▼       ▼        ▼         ▼        ▼           ▼
┌──────┐┌──────┐┌────────┐┌───────┐┌───────┐  ┌────────┐
│AuthDB││UserDB││ProdDB  ││OrdDB  ││ MinIO │  │MailHog │
└──────┘└──────┘└────────┘└───────┘└───────┘  └────────┘
                     │         │
                     ▼         ▼
               ┌─────────────────┐
               │  Kafka (:29092) │
               └─────────────────┘
```

**Сервисы проекта:**

| Сервис | HTTP | gRPC | Реплики | HPA |
|--------|------|------|---------|-----|
| gateway | 8080 | — | 2 | 2-10 |
| auth-service | 8081 | 50051 | 2 | 2-5 |
| user-service | 8082 | 50052 | 2 | 2-5 |
| notification-service | 8083 | 50056 | 1 | нет |
| media-service | 8084 | 50055 | 1 | нет |
| product-service | 8085 | 50053 | 2 | 2-5 |
| order-service | 8086 | 50054 | 2 | 2-5 |

---

## 2. Основные концепции Kubernetes

### Что такое Kubernetes?

Kubernetes (k8s) — это платформа оркестрации контейнеров. Она решает задачи:
- **Запуск контейнеров** — автоматическое размещение на нодах кластера
- **Масштабирование** — увеличение/уменьшение количества экземпляров
- **Самовосстановление** — перезапуск упавших контейнеров
- **Service discovery** — автоматический DNS для сервисов
- **Балансировка нагрузки** — распределение трафика между подами
- **Rolling updates** — обновление без даунтайма

### Ключевые объекты

```
Pod          — минимальная единица, один или несколько контейнеров
Deployment   — управляет подами: сколько реплик, как обновлять
Service      — стабильный сетевой endpoint для группы подов
ConfigMap    — конфигурация (переменные окружения, файлы)
Secret       — конфиденциальные данные (пароли, ключи)
Ingress      — маршрутизация внешнего HTTP-трафика
PVC          — запрос на persistent storage
HPA          — автомасштабирование по метрикам
Namespace    — логическая изоляция ресурсов
```

### Жизненный цикл Pod

```
Pending → ContainerCreating → Running → Terminating → Terminated
                                 │
                                 ├─ CrashLoopBackOff (ошибка запуска)
                                 ├─ OOMKilled (нехватка памяти)
                                 └─ Error (ошибка приложения)
```

---

## 3. Структура манифестов проекта

```
k8s/
└── base/                        # Основные манифесты
    ├── namespace.yaml           # Namespace "microservices"
    ├── configmap.yaml           # Конфигурация всех сервисов
    ├── secrets.yaml             # Пароли БД, Keycloak, MinIO
    ├── services.yaml            # Deployments + Services (6 микросервисов + gateway)
    ├── databases.yaml           # PostgreSQL для каждого сервиса
    ├── kafka.yaml               # Kafka broker
    ├── keycloak.yaml            # Keycloak + его БД + realm config
    ├── ingress.yaml             # NGINX Ingress правила
    └── hpa.yaml                 # Autoscaler-ы

helm/
└── microservices/               # Helm-альтернатива (параметризированные шаблоны)
    ├── Chart.yaml
    ├── values.yaml
    └── templates/
```

### Порядок применения манифестов

Манифесты нужно применять в определённом порядке, потому что одни ресурсы зависят от других:

```bash
# 1. Namespace (всё остальное живёт внутри него)
kubectl apply -f k8s/base/namespace.yaml

# 2. ConfigMap + Secrets (нужны Deployments для env-переменных)
kubectl apply -f k8s/base/configmap.yaml
kubectl apply -f k8s/base/secrets.yaml

# 3. Базы данных (сервисы подключаются к ним при старте)
kubectl apply -f k8s/base/databases.yaml

# 4. Инфраструктура (Kafka, Keycloak — сервисы зависят от них)
kubectl apply -f k8s/base/kafka.yaml
kubectl apply -f k8s/base/keycloak.yaml

# 5. Микросервисы (зависят от БД, Kafka, Keycloak)
kubectl apply -f k8s/base/services.yaml

# 6. Ingress (маршрутизация на уже запущенные сервисы)
kubectl apply -f k8s/base/ingress.yaml

# 7. HPA (автоскейлинг для уже существующих Deployments)
kubectl apply -f k8s/base/hpa.yaml
```

Или всё сразу (Kubernetes разрешит зависимости, но поды могут перезапуститься пока зависимости не готовы):

```bash
kubectl apply -f k8s/base/
```

---

## 4. Namespace

**Файл:** `k8s/base/namespace.yaml`

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: microservices
```

### Зачем нужен Namespace?

- **Изоляция** — ресурсы разных проектов не пересекаются
- **Управление доступом** — RBAC-политики на уровне namespace
- **Квоты** — можно ограничить CPU/RAM на весь namespace
- **Удобство** — `kubectl delete namespace microservices` удалит ВСЁ

### Работа с namespace

```bash
# Все команды по умолчанию работают в namespace "default"
# Чтобы работать с нашим namespace, добавляем -n:
kubectl get pods -n microservices

# Или переключаем контекст на постоянной основе:
kubectl config set-context --current --namespace=microservices

# Теперь -n не нужен:
kubectl get pods
```

---

## 5. ConfigMap и Secrets

### ConfigMap

**Файл:** `k8s/base/configmap.yaml`

ConfigMap хранит **неконфиденциальную** конфигурацию. В проекте это:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: services-config
  namespace: microservices
data:
  # Адреса баз данных
  AUTH_DB_HOST: "auth-db"
  USER_DB_HOST: "user-db"
  # ...

  # Адреса gRPC сервисов
  AUTH_GRPC_HOST: "auth-service"
  AUTH_GRPC_PORT: "50051"
  # ...

  # Kafka
  KAFKA_BROKERS: "kafka:29092"

  # Keycloak, MinIO, SMTP, Jaeger
```

**Как Deployment использует ConfigMap:**

```yaml
envFrom:
  - configMapRef:
      name: services-config   # Все ключи ConfigMap → env-переменные
```

Каждый ключ ConfigMap (`AUTH_DB_HOST`, `KAFKA_BROKERS` и т.д.) становится переменной окружения внутри контейнера. Приложение читает их через `os.Getenv("KAFKA_BROKERS")`.

### Secrets

**Файл:** `k8s/base/secrets.yaml`

Secrets хранят **конфиденциальные** данные. Значения кодируются в **base64** (НЕ шифрование!):

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: db-secrets
  namespace: microservices
type: Opaque
data:
  AUTH_DB_USER: cG9zdGdyZXM=         # "postgres" в base64
  AUTH_DB_PASSWORD: cGFzc3dvcmQ=     # "password" в base64
```

**Кодирование/декодирование base64:**

```bash
# Закодировать
echo -n "mypassword" | base64
# → bXlwYXNzd29yZA==

# Декодировать
echo "bXlwYXNzd29yZA==" | base64 -d
# → mypassword
```

**Как Deployment использует Secrets:**

```yaml
envFrom:
  - secretRef:
      name: db-secrets       # Все ключи Secret → env-переменные
```

> **Важно:** в продакшене используйте внешние хранилища секретов (Vault, AWS Secrets Manager, Sealed Secrets). Base64 в YAML — это НЕ безопасно.

---

## 6. Deployments

**Файл:** `k8s/base/services.yaml`

Deployment описывает **что запускать** и **как управлять** подами.

### Анатомия Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: auth-service                    # Имя Deployment
  namespace: microservices
  labels:
    app: auth-service                   # Метка для группировки
spec:
  replicas: 2                          # Количество подов
  selector:
    matchLabels:
      app: auth-service                # Какие поды принадлежат этому Deployment
  template:                            # Шаблон пода
    metadata:
      labels:
        app: auth-service              # Метка пода (должна совпадать с selector)
    spec:
      containers:
        - name: auth-service
          image: auth-service:latest    # Docker-образ
          imagePullPolicy: IfNotPresent # Не тянуть если уже есть локально
          ports:
            - containerPort: 8081      # HTTP
            - containerPort: 50051     # gRPC
          envFrom:
            - configMapRef:
                name: services-config  # Конфигурация
            - secretRef:
                name: db-secrets       # Секреты
          resources:
            requests:                  # Минимум ресурсов (для планировщика)
              cpu: 50m                 # 0.05 ядра CPU
              memory: 64Mi             # 64 МБ RAM
            limits:                    # Максимум ресурсов (жёсткий лимит)
              cpu: 200m               # 0.2 ядра CPU
              memory: 256Mi            # 256 МБ RAM
```

### Ключевые поля

| Поле | Что делает | Пример |
|------|-----------|--------|
| `replicas` | Сколько экземпляров запустить | `2` |
| `selector.matchLabels` | По какой метке найти "свои" поды | `app: auth-service` |
| `image` | Docker-образ для контейнера | `auth-service:latest` |
| `imagePullPolicy` | Когда качать образ | `IfNotPresent`, `Always`, `Never` |
| `resources.requests` | Гарантированный минимум ресурсов | `cpu: 50m, memory: 64Mi` |
| `resources.limits` | Жёсткий потолок ресурсов | `cpu: 200m, memory: 256Mi` |

### Единицы измерения ресурсов

```
CPU:
  1    = 1 ядро CPU
  500m = 0.5 ядра (m = millicores, тысячные доли)
  50m  = 0.05 ядра

Memory:
  Ki = кибибайт (1024 байт)
  Mi = мебибайт (1024 Ki)
  Gi = гибибайт (1024 Mi)
  64Mi  = 64 мебибайт
  2Gi   = 2 гибибайта
```

### Стратегии обновления

```yaml
spec:
  strategy:
    type: RollingUpdate          # Обновлять поды постепенно (по умолчанию)
    rollingUpdate:
      maxSurge: 1                # Создать макс. 1 "лишний" под при обновлении
      maxUnavailable: 0          # Не допускать недоступных подов
```

- **RollingUpdate** — новые поды создаются, старые удаляются по одному (zero downtime)
- **Recreate** — сначала удалить все старые, потом создать новые (есть downtime)

---

## 7. Services (сетевое взаимодействие)

**Файл:** `k8s/base/services.yaml`

Service создаёт **стабильный сетевой endpoint** для группы подов. Поды умирают и создаются, но Service всегда доступен по одному адресу.

### Анатомия Service

```yaml
apiVersion: v1
kind: Service
metadata:
  name: auth-service              # DNS-имя внутри кластера
  namespace: microservices
spec:
  selector:
    app: auth-service             # Какие поды обслуживать
  ports:
    - name: http
      port: 8081                  # Порт Service (на него приходят запросы)
      targetPort: 8081            # Порт контейнера (куда перенаправляются)
    - name: grpc
      port: 50051
      targetPort: 50051
```

### DNS внутри кластера

Kubernetes автоматически создаёт DNS-записи для каждого Service:

```
# Формат: <service-name>.<namespace>.svc.cluster.local
auth-service.microservices.svc.cluster.local

# Внутри того же namespace можно короче:
auth-service
```

Поэтому в ConfigMap адреса сервисов — это просто их имена:

```yaml
AUTH_GRPC_HOST: "auth-service"      # → auth-service.microservices.svc.cluster.local
KAFKA_BROKERS: "kafka:29092"        # → kafka.microservices.svc.cluster.local:29092
```

### Типы Service

```
ClusterIP (по умолчанию) — доступен только внутри кластера
NodePort                 — доступен снаружи через порт ноды (30000-32767)
LoadBalancer             — создаёт внешний балансировщик (в облаке)
ExternalName             — алиас для внешнего DNS-имени
```

В этом проекте все Service — **ClusterIP**. Внешний доступ идёт через Ingress.

---

## 8. Ingress

**Файл:** `k8s/base/ingress.yaml`

Ingress — это **HTTP-маршрутизатор** на входе в кластер. Принимает внешние запросы и направляет их на нужные Service.

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: gateway-ingress
  namespace: microservices
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
spec:
  ingressClassName: nginx              # Какой Ingress Controller использовать
  rules:
    - host: api.microservices.local    # Виртуальный хост
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: gateway          # Перенаправить на Service "gateway"
                port:
                  number: 8080

    - host: keycloak.microservices.local
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: keycloak
                port:
                  number: 8080
```

### Как это работает

```
Браузер → api.microservices.local
       → Ingress Controller (NGINX pod в кластере)
       → Ingress Rule: host=api.microservices.local → gateway:8080
       → Service "gateway" → Pod gateway-xxx
```

### Настройка локального DNS

Для работы с локальными доменами добавьте в `/etc/hosts`:

```
127.0.0.1  api.microservices.local
127.0.0.1  keycloak.microservices.local
127.0.0.1  mail.microservices.local
```

### Установка Ingress Controller

```bash
# Для Minikube:
minikube addons enable ingress

# Для Docker Desktop / другого кластера:
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.12.0/deploy/static/provider/cloud/deploy.yaml
```

---

## 9. Persistent Volumes

**Файл:** `k8s/base/databases.yaml`

### Проблема

Поды — **эфемерны**. Когда под умирает, все данные внутри контейнера теряются. Для баз данных это недопустимо.

### Решение — PVC (PersistentVolumeClaim)

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: auth-db-pvc
  namespace: microservices
spec:
  accessModes:
    - ReadWriteOnce             # Только один под может писать
  resources:
    requests:
      storage: 1Gi              # Запрашиваем 1 ГБ диска
```

**Использование в Deployment:**

```yaml
spec:
  containers:
    - name: auth-db
      image: postgres:16-alpine
      volumeMounts:
        - name: auth-db-storage
          mountPath: /var/lib/postgresql/data    # Куда монтировать внутри контейнера
  volumes:
    - name: auth-db-storage
      persistentVolumeClaim:
        claimName: auth-db-pvc                  # Ссылка на PVC
```

### PVC в проекте

| PVC | Размер | Для чего |
|-----|--------|---------|
| auth-db-pvc | 1Gi | PostgreSQL auth-service |
| user-db-pvc | 1Gi | PostgreSQL user-service |
| product-db-pvc | 1Gi | PostgreSQL product-service |
| order-db-pvc | 1Gi | PostgreSQL order-service |
| keycloak-db-pvc | 1Gi | PostgreSQL Keycloak |
| minio-pvc | 5Gi | MinIO object storage |

### Access Modes

```
ReadWriteOnce (RWO) — один под читает/пишет (для баз данных)
ReadOnlyMany  (ROX) — много подов только читают
ReadWriteMany (RWX) — много подов читают/пишут (нужен NFS/Ceph)
```

---

## 10. Horizontal Pod Autoscaler (HPA)

**Файл:** `k8s/base/hpa.yaml`

HPA автоматически меняет количество реплик Deployment на основе метрик (CPU, RAM).

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: gateway-hpa
  namespace: microservices
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: gateway                          # Какой Deployment скейлить
  minReplicas: 2                           # Минимум подов
  maxReplicas: 10                          # Максимум подов
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70           # Скейлить если CPU > 70%
    - type: Resource
      resource:
        name: memory
        target:
          type: Utilization
          averageUtilization: 80           # Скейлить если RAM > 80%
```

### Как работает

```
CPU использование: 30% → 2 реплики (min)
CPU использование: 75% → HPA добавляет реплики
CPU использование: 95% → Возможно до 10 реплик
CPU использование: 20% → HPA уменьшает до min (2)
```

### Требования

HPA требует **Metrics Server** в кластере:

```bash
# Minikube:
minikube addons enable metrics-server

# Другие кластеры:
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
```

### Проверка HPA

```bash
kubectl get hpa -n microservices
# NAME           REFERENCE           TARGETS          MINPODS   MAXPODS   REPLICAS
# gateway-hpa   Deployment/gateway  45%/70%, 30%/80%  2         10        2
```

---

## 11. Kafka в Kubernetes

**Файл:** `k8s/base/kafka.yaml`

Kafka используется для **асинхронного обмена событиями** между сервисами.

### Топики в проекте

| Топик | Продьюсер | Консьюмер |
|-------|-----------|-----------|
| `user.registered` | user-service | notification-service |
| `order.events` | order-service | notification-service |

### Настройка Kafka в k8s

```yaml
env:
  - name: KAFKA_LISTENERS
    value: "INTERNAL://0.0.0.0:29092,EXTERNAL://0.0.0.0:9092"
  - name: KAFKA_ADVERTISED_LISTENERS
    value: "INTERNAL://kafka:29092,EXTERNAL://localhost:9092"
  - name: KAFKA_LISTENER_SECURITY_PROTOCOL_MAP
    value: "INTERNAL:PLAINTEXT,EXTERNAL:PLAINTEXT"
  - name: KAFKA_INTER_BROKER_LISTENER_NAME
    value: "INTERNAL"
```

**Два листенера:**
- `INTERNAL://kafka:29092` — для сервисов **внутри** кластера (по DNS-имени `kafka`)
- `EXTERNAL://localhost:9092` — для доступа **снаружи** (для отладки)

---

## 12. Keycloak в Kubernetes

**Файл:** `k8s/base/keycloak.yaml`

Keycloak — IAM-система (управление пользователями и авторизацией).

### Компоненты

```
keycloak-db (PostgreSQL) → Keycloak сервер → Realm "microservices"
                                               ├── Client: auth-service
                                               └── Roles: user, admin
```

### Realm Configuration

Keycloak настраивается через ConfigMap с JSON realm-конфигурацией. Это позволяет декларативно задать:
- Клиентов (applications)
- Роли
- Настройки токенов (TTL: 3600 секунд)

---

## 13. Helm Chart

**Директория:** `helm/microservices/`

Helm — это **пакетный менеджер** для Kubernetes. Вместо захардкоженных YAML-ов используются шаблоны с переменными.

### Зачем Helm, если есть plain manifests?

| Plain YAML | Helm |
|------------|------|
| Жёстко захардкоженные значения | Параметры в `values.yaml` |
| Копипаст для разных окружений | Один чарт, разные values-файлы |
| Ручное применение | `helm install / upgrade / rollback` |
| Нет версионирования | Версии релизов с возможностью отката |

### Структура Helm Chart

```
helm/microservices/
├── Chart.yaml           # Метаданные чарта (имя, версия)
├── values.yaml          # Значения по умолчанию
└── templates/
    ├── _helpers.tpl     # Переиспользуемые шаблоны
    ├── gateway.yaml     # Шаблон gateway Deployment + HPA
    ├── app-services.yaml # Шаблон для всех микросервисов
    ├── configmap.yaml   # Шаблон ConfigMap
    └── ingress.yaml     # Шаблон Ingress
```

### Пример: values.yaml

```yaml
global:
  namespace: microservices
  image:
    pullPolicy: IfNotPresent

gateway:
  replicas: 2
  image: gateway:latest
  port: 8080
  hpa:
    minReplicas: 2
    maxReplicas: 10
    cpuTarget: 70

services:
  - name: auth-service
    replicas: 2
    image: auth-service:latest
    httpPort: 8081
    grpcPort: 50051
```

### Команды Helm

```bash
# Установить чарт
helm install my-app ./helm/microservices/

# Обновить (после изменения values/templates)
helm upgrade my-app ./helm/microservices/

# Откатить к предыдущей версии
helm rollback my-app 1

# Удалить
helm uninstall my-app

# Посмотреть сгенерированные манифесты (dry-run)
helm template my-app ./helm/microservices/

# Установить с кастомными values
helm install my-app ./helm/microservices/ -f values-prod.yaml
```

---

## 14. Команды для работы

### Просмотр ресурсов

```bash
# Все поды
kubectl get pods -n microservices

# Подробно (с нодой и IP)
kubectl get pods -n microservices -o wide

# Все ресурсы
kubectl get all -n microservices

# Deployments
kubectl get deployments -n microservices

# Services
kubectl get svc -n microservices

# Ingress
kubectl get ingress -n microservices

# PVC
kubectl get pvc -n microservices

# HPA (с текущими метриками)
kubectl get hpa -n microservices

# Events (полезно для отладки)
kubectl get events -n microservices --sort-by=.metadata.creationTimestamp
```

### Логи

```bash
# Логи пода
kubectl logs <pod-name> -n microservices

# Логи в реальном времени (follow)
kubectl logs -f <pod-name> -n microservices

# Логи предыдущего контейнера (если перезапустился)
kubectl logs <pod-name> -n microservices --previous

# Логи всех подов сервиса
kubectl logs -l app=auth-service -n microservices
```

### Отладка

```bash
# Детальная информация о поде (события, статус, причины ошибок)
kubectl describe pod <pod-name> -n microservices

# Детальная информация о Deployment
kubectl describe deployment auth-service -n microservices

# Зайти внутрь контейнера (shell)
kubectl exec -it <pod-name> -n microservices -- /bin/sh

# Проверить DNS изнутри пода
kubectl exec -it <pod-name> -n microservices -- nslookup auth-service

# Проброс порта на localhost (для отладки)
kubectl port-forward svc/auth-service 8081:8081 -n microservices
# Теперь http://localhost:8081 → auth-service в кластере
```

### Управление

```bash
# Применить манифесты
kubectl apply -f k8s/base/

# Удалить ресурсы
kubectl delete -f k8s/base/services.yaml

# Перезапустить Deployment (rolling restart)
kubectl rollout restart deployment auth-service -n microservices

# Масштабировать вручную
kubectl scale deployment auth-service --replicas=3 -n microservices

# Статус rollout
kubectl rollout status deployment auth-service -n microservices

# История rollout
kubectl rollout history deployment auth-service -n microservices

# Откат к предыдущей версии
kubectl rollout undo deployment auth-service -n microservices
```

---

## 15. Troubleshooting

### Pod не запускается

```bash
# 1. Проверить статус
kubectl get pods -n microservices
# STATUS: CrashLoopBackOff, ImagePullBackOff, Pending, Error

# 2. Посмотреть события
kubectl describe pod <pod-name> -n microservices

# 3. Посмотреть логи
kubectl logs <pod-name> -n microservices
```

**Частые проблемы:**

| Статус | Причина | Решение |
|--------|---------|---------|
| `ImagePullBackOff` | Образ не найден | Проверьте имя образа, `imagePullPolicy`, доступность registry |
| `CrashLoopBackOff` | Приложение падает | Смотрите логи (`kubectl logs`) |
| `Pending` | Нет ресурсов на нодах | Увеличьте ноды или уменьшите requests |
| `OOMKilled` | Превышен лимит памяти | Увеличьте `resources.limits.memory` |
| `CreateContainerConfigError` | Не найден ConfigMap/Secret | Проверьте что ConfigMap/Secret созданы |

### Сервис не отвечает

```bash
# 1. Проверить endpoints Service
kubectl get endpoints auth-service -n microservices
# Если ENDPOINTS пусто — selector не совпадает с labels подов

# 2. Проверить доступность из другого пода
kubectl exec -it <pod-name> -n microservices -- wget -qO- http://auth-service:8081/health

# 3. Port-forward для локальной проверки
kubectl port-forward svc/auth-service 8081:8081 -n microservices
curl http://localhost:8081/health
```

### Ingress не работает

```bash
# 1. Проверить Ingress Controller
kubectl get pods -n ingress-nginx

# 2. Проверить Ingress resource
kubectl describe ingress gateway-ingress -n microservices

# 3. Проверить /etc/hosts
cat /etc/hosts | grep microservices

# 4. Проверить что backend Service существует и имеет endpoints
kubectl get svc gateway -n microservices
kubectl get endpoints gateway -n microservices
```

---

## 16. Best Practices

### Resource Management

```yaml
# ВСЕГДА указывайте requests и limits
resources:
  requests:          # "Мне нужно минимум столько"
    cpu: 50m
    memory: 64Mi
  limits:            # "Не давайте мне больше"
    cpu: 200m
    memory: 256Mi
```

**Правило:** `limits` обычно в 2-4 раза больше `requests`.

### Health Checks (рекомендация к добавлению)

В текущем проекте probes не настроены. Рекомендуется добавить:

```yaml
containers:
  - name: auth-service
    # Готов ли под принимать трафик?
    readinessProbe:
      httpGet:
        path: /health
        port: 8081
      initialDelaySeconds: 5
      periodSeconds: 10

    # Жив ли под? (перезапустить если нет)
    livenessProbe:
      httpGet:
        path: /health
        port: 8081
      initialDelaySeconds: 15
      periodSeconds: 20

    # Для медленного старта (Keycloak, тяжёлые сервисы)
    startupProbe:
      httpGet:
        path: /health
        port: 8081
      failureThreshold: 30
      periodSeconds: 10
```

### Безопасность

```yaml
# Security Context — ограничение привилегий контейнера
securityContext:
  runAsNonRoot: true           # Не запускать от root
  readOnlyRootFilesystem: true # Файловая система только для чтения
  allowPrivilegeEscalation: false
```

### Labels и Annotations

```yaml
metadata:
  labels:
    app: auth-service          # Обязательно — для selector
    version: v1                # Для canary/blue-green deploys
    team: backend              # Для фильтрации
  annotations:
    description: "Authentication microservice"
```

### Многоокружность (dev/staging/prod)

Для разных окружений рекомендуется использовать **Kustomize overlays** или **Helm values**:

```
# Вариант 1: Kustomize
k8s/
├── base/                    # Общие манифесты
├── overlays/
│   ├── dev/                 # Переопределения для dev
│   │   └── kustomization.yaml
│   ├── staging/
│   │   └── kustomization.yaml
│   └── prod/
│       └── kustomization.yaml

# Вариант 2: Helm
helm/microservices/
├── values.yaml              # Дефолтные значения
├── values-dev.yaml          # Dev overrides
├── values-staging.yaml      # Staging overrides
└── values-prod.yaml         # Production overrides
```

### Порядок миграции к production

1. Заменить `imagePullPolicy: IfNotPresent` на `Always` + конкретные теги (не `latest`)
2. Добавить health probes (liveness, readiness, startup)
3. Использовать внешний Secrets Manager вместо plaintext secrets
4. Настроить NetworkPolicies (ограничить трафик между сервисами)
5. Добавить PodDisruptionBudgets (гарантия доступности при обслуживании нод)
6. Настроить resource quotas на уровне namespace
7. Использовать Kustomize overlays или Helm values для разных окружений
