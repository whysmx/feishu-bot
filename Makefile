.PHONY: build run clean test docker-build docker-run

# 变量定义
APP_NAME=feishu-bot
BUILD_DIR=bin
DOCKER_IMAGE=feishu-bot:latest

# 构建
build:
	@echo "Building applications..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/bot ./cmd/bot
	go build -o $(BUILD_DIR)/webhook ./cmd/webhook
	go build -o $(BUILD_DIR)/relay ./cmd/relay

# 运行主机器人
run-bot:
	@echo "Starting bot service..."
	go run ./cmd/bot

# 运行webhook服务
run-webhook:
	@echo "Starting webhook service..."
	go run ./cmd/webhook

# 运行命令中继服务
run-relay:
	@echo "Starting command relay service..."
	go run ./cmd/relay

# 清理
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -rf data/logs/*

# 测试
test:
	@echo "Running tests..."
	go test -v ./...

# 获取依赖
deps:
	@echo "Getting dependencies..."
	go mod tidy
	go mod download

# Docker构建
docker-build:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE) .

# Docker运行
docker-run:
	@echo "Running Docker container..."
	docker-compose up -d

# 开发环境设置
dev-setup:
	@echo "Setting up development environment..."
	@mkdir -p data/logs
	@cp .env.example .env
	@echo "Please edit .env file with your configuration"

# 格式化代码
fmt:
	@echo "Formatting code..."
	go fmt ./...

# 代码检查
lint:
	@echo "Running linter..."
	golangci-lint run

# 帮助
help:
	@echo "Available commands:"
	@echo "  build      - Build all applications"
	@echo "  run-bot    - Run bot service"
	@echo "  run-webhook - Run webhook service"
	@echo "  run-relay  - Run command relay service"
	@echo "  clean      - Clean build artifacts"
	@echo "  test       - Run tests"
	@echo "  deps       - Get dependencies"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run - Run with Docker Compose"
	@echo "  dev-setup  - Setup development environment"
	@echo "  fmt        - Format code"
	@echo "  lint       - Run linter"