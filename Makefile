RUN_DIR  := run
BIN      := $(RUN_DIR)/many-opinions-go
PID_FILE := $(RUN_DIR)/.server.pid
LOG_FILE := $(RUN_DIR)/server.log

# Read PORT from .env, fall back to 8000
PORT := $(shell grep -s '^PORT=' .env | cut -d= -f2 | tr -d ' ')
ifeq ($(PORT),)
  PORT := 8000
endif

IMAGE    := many-opinions-go
CONTAINER := many-opinions-go

.PHONY: help install lint build start stop restart status logs health format run clean docker-build docker-run docker-stop

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

install: ## Install dependencies
	go mod download
	go mod tidy
	@echo "Done. Copy .env.example to .env and set your API keys."

lint: ## Run Go vet
	go vet ./...

format: ## Run Go fmt
	go fmt ./...

build: ## Build the server binary
	@mkdir -p $(RUN_DIR)
	go build -o $(BIN) .

start: build ## Start server in background
	@if [ -f $(PID_FILE) ] && kill -0 $$(cat $(PID_FILE)) 2>/dev/null; then \
		echo "Server already running (PID $$(cat $(PID_FILE)))"; \
	else \
		$(BIN) -transport sse -port $(PORT) >> $(LOG_FILE) 2>&1 & \
		echo $$! > $(PID_FILE); \
		sleep 1; \
		if kill -0 $$(cat $(PID_FILE)) 2>/dev/null; then \
			echo "Server started (PID $$(cat $(PID_FILE))) on port $(PORT)"; \
		else \
			echo "Server failed to start. Check $(LOG_FILE)"; \
			rm -f $(PID_FILE); \
			exit 1; \
		fi; \
	fi

stop: ## Stop server
	@if [ -f $(PID_FILE) ]; then \
		kill $$(cat $(PID_FILE)) 2>/dev/null && echo "Server stopped" || echo "Server not running"; \
		rm -f $(PID_FILE); \
	else \
		echo "No PID file found"; \
	fi

restart: stop start ## Restart server

status: ## Show server status
	@if [ -f $(PID_FILE) ] && kill -0 $$(cat $(PID_FILE)) 2>/dev/null; then \
		echo "Running (PID $$(cat $(PID_FILE)), port $(PORT))"; \
	else \
		echo "Stopped"; \
		rm -f $(PID_FILE) 2>/dev/null; \
	fi

logs: ## Tail server logs
	@tail -f $(LOG_FILE)

run: ## Run server in foreground (stdio transport configuration)
	go run .

clean: stop ## Stop server and remove logs/pid/binary
	rm -rf $(RUN_DIR)

docker-build: ## Build Docker image
	docker build -t $(IMAGE) .

docker-run: ## Run Docker container in background
	@if docker ps -a --format '{{.Names}}' | grep -Eq '^$(CONTAINER)$$'; then \
		echo "Removing existing container $(CONTAINER)"; \
		docker rm -f $(CONTAINER) >/dev/null; \
	fi
	@if [ -f .env ]; then \
		docker run -d --name $(CONTAINER) --env-file .env -p $(PORT):19801 $(IMAGE); \
	else \
		docker run -d --name $(CONTAINER) -p $(PORT):19801 $(IMAGE); \
	fi

docker-stop: ## Stop and remove Docker container
	@docker rm -f $(CONTAINER) >/dev/null 2>&1 || true

docker-start: docker-build docker-run ## Build image and run container
