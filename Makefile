.PHONY: build run test bench lint clean docker-build docker-up docker-down

BINARY := ratelimiter
DOCKER_COMPOSE := deployments/docker-compose.yml

build:
	go build -o bin/$(BINARY) ./cmd/limiter

run: build
	./bin/$(BINARY) -config configs/rules.yaml

test:
	go test -v -race ./...

bench:
	go test -bench=. -benchmem ./bench/...

lint:
	go vet ./...

clean:
	rm -rf bin/

docker-build:
	docker build -t $(BINARY) .

docker-up:
	docker compose -f $(DOCKER_COMPOSE) up --build -d

docker-down:
	docker compose -f $(DOCKER_COMPOSE) down -v

integration:
	go test -v -race -tags=integration ./integration/...
