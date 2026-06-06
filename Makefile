.PHONY: proto build up down restart logs auth-logs user-logs gateway-logs clean tidy \
	k8s-apply k8s-delete k8s-status k8s-logs k8s-restart k8s-build k8s-deploy k8s-clean \
	helm-install helm-upgrade helm-uninstall helm-template

# Generate protobuf code for all services
proto:
	@echo "Generating proto files..."
	protoc --go_out=./auth-service/proto/auth --go_opt=paths=source_relative \
		--go-grpc_out=./auth-service/proto/auth --go-grpc_opt=paths=source_relative \
		-I./proto proto/auth.proto
	protoc --go_out=./user-service/proto/user --go_opt=paths=source_relative \
		--go-grpc_out=./user-service/proto/user --go-grpc_opt=paths=source_relative \
		-I./proto proto/user.proto
	protoc --go_out=./gateway-service/proto/auth --go_opt=paths=source_relative \
		--go-grpc_out=./gateway-service/proto/auth --go-grpc_opt=paths=source_relative \
		-I./proto proto/auth.proto
	protoc --go_out=./gateway-service/proto/user --go_opt=paths=source_relative \
		--go-grpc_out=./gateway-service/proto/user --go-grpc_opt=paths=source_relative \
		-I./proto proto/user.proto
	protoc --go_out=./gateway-service/proto/product --go_opt=paths=source_relative \
		--go-grpc_out=./gateway-service/proto/product --go-grpc_opt=paths=source_relative \
		-I./proto proto/product.proto
	protoc --go_out=./gateway-service/proto/order --go_opt=paths=source_relative \
		--go-grpc_out=./gateway-service/proto/order --go-grpc_opt=paths=source_relative \
		-I./proto proto/order.proto
	protoc --go_out=./gateway-service/proto/media --go_opt=paths=source_relative \
		--go-grpc_out=./gateway-service/proto/media --go-grpc_opt=paths=source_relative \
		-I./proto proto/media.proto
	protoc --go_out=./gateway-service/proto/notification --go_opt=paths=source_relative \
		--go-grpc_out=./gateway-service/proto/notification --go-grpc_opt=paths=source_relative \
		-I./proto proto/notification.proto
	protoc --go_out=./product-service/proto/product --go_opt=paths=source_relative \
		--go-grpc_out=./product-service/proto/product --go-grpc_opt=paths=source_relative \
		-I./proto proto/product.proto
	protoc --go_out=./order-service/proto/order --go_opt=paths=source_relative \
		--go-grpc_out=./order-service/proto/order --go-grpc_opt=paths=source_relative \
		-I./proto proto/order.proto
	protoc --go_out=./order-service/proto/product --go_opt=paths=source_relative \
		--go-grpc_out=./order-service/proto/product --go-grpc_opt=paths=source_relative \
		-I./proto proto/product.proto
	protoc --go_out=./order-service/proto/notification --go_opt=paths=source_relative \
		--go-grpc_out=./order-service/proto/notification --go-grpc_opt=paths=source_relative \
		-I./proto proto/notification.proto
	protoc --go_out=./user-service/proto/media --go_opt=paths=source_relative \
		--go-grpc_out=./user-service/proto/media --go-grpc_opt=paths=source_relative \
		-I./proto proto/media.proto
	protoc --go_out=./media-service/proto/media --go_opt=paths=source_relative \
		--go-grpc_out=./media-service/proto/media --go-grpc_opt=paths=source_relative \
		-I./proto proto/media.proto
	protoc --go_out=./notification-service/proto/notification --go_opt=paths=source_relative \
		--go-grpc_out=./notification-service/proto/notification --go-grpc_opt=paths=source_relative \
		-I./proto proto/notification.proto
	@echo "Proto generation complete!"

# Build all services
build:
	docker compose build

# Start all services
up:
	docker compose up -d

# Stop all services
down:
	docker compose down

# Restart all services
restart:
	docker compose restart

# View logs for all services
logs:
	docker compose logs -f

# View auth service logs
auth-logs:
	docker compose logs -f auth-service

# View user service logs
user-logs:
	docker compose logs -f user-service

# View gateway logs
gateway-logs:
	docker compose logs -f gateway

# Clean up volumes and containers
clean:
	docker compose down -v

# Tidy go modules
tidy:
	cd auth-service && go mod tidy
	cd user-service && go mod tidy
	cd gateway-service && go mod tidy

# ============================================================
# Kubernetes
# ============================================================

NAMESPACE   = microservices
K8S_DIR     = k8s/base
HELM_DIR    = helm/microservices
REGISTRY    ?= localhost:5000
TAG         ?= latest
SERVICES    = auth-service user-service gateway-service product-service order-service media-service notification-service

# Apply all k8s manifests (in dependency order)
k8s-apply:
	kubectl apply -f $(K8S_DIR)/namespace.yaml
	kubectl apply -f $(K8S_DIR)/configmap.yaml
	kubectl apply -f $(K8S_DIR)/secrets.yaml
	kubectl apply -f $(K8S_DIR)/databases.yaml
	kubectl apply -f $(K8S_DIR)/kafka.yaml
	kubectl apply -f $(K8S_DIR)/keycloak.yaml
	kubectl apply -f $(K8S_DIR)/services.yaml
	kubectl apply -f $(K8S_DIR)/ingress.yaml
	kubectl apply -f $(K8S_DIR)/hpa.yaml

# Delete all k8s resources
k8s-delete:
	kubectl delete -f $(K8S_DIR)/ --ignore-not-found

# Show status of all resources in namespace
k8s-status:
	@echo "=== Pods ==="
	@kubectl get pods -n $(NAMESPACE)
	@echo "\n=== Services ==="
	@kubectl get svc -n $(NAMESPACE)
	@echo "\n=== Deployments ==="
	@kubectl get deployments -n $(NAMESPACE)
	@echo "\n=== HPA ==="
	@kubectl get hpa -n $(NAMESPACE)
	@echo "\n=== Ingress ==="
	@kubectl get ingress -n $(NAMESPACE)
	@echo "\n=== PVC ==="
	@kubectl get pvc -n $(NAMESPACE)

# Stream logs for a service: make k8s-logs SVC=auth-service
k8s-logs:
	kubectl logs -f -l app=$(SVC) -n $(NAMESPACE)

# Rolling restart a service: make k8s-restart SVC=auth-service
k8s-restart:
	kubectl rollout restart deployment $(SVC) -n $(NAMESPACE)

# Build and tag Docker images for all services
k8s-build:
	@for svc in $(SERVICES); do \
		echo "Building $$svc..."; \
		docker build -t $(REGISTRY)/$$svc:$(TAG) ./$$svc/; \
	done

# Build + push images to registry
k8s-push: k8s-build
	@for svc in $(SERVICES); do \
		echo "Pushing $$svc..."; \
		docker push $(REGISTRY)/$$svc:$(TAG); \
	done

# Full deploy: build images + apply manifests
k8s-deploy: k8s-build k8s-apply

# Delete everything including persistent volumes
k8s-clean:
	kubectl delete namespace $(NAMESPACE) --ignore-not-found

# ============================================================
# Helm
# ============================================================

# Install helm chart
helm-install:
	helm install microservices $(HELM_DIR) -n $(NAMESPACE) --create-namespace

# Upgrade helm release
helm-upgrade:
	helm upgrade microservices $(HELM_DIR) -n $(NAMESPACE)

# Uninstall helm release
helm-uninstall:
	helm uninstall microservices -n $(NAMESPACE)

# Render templates without applying (dry-run)
helm-template:
	helm template microservices $(HELM_DIR)
