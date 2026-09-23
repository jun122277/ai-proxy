SHELL := /bin/sh
.DEFAULT_GOAL := help
COMPOSE := docker compose -f deploy/compose/compose.yaml
IMAGE ?= ai-proxy:dev
GOVULNCHECK_VERSION := v1.8.0
TRIVY_IMAGE := aquasec/trivy:0.74.0@sha256:62b1e65e8869bc4b4c6aa4fa2b21595256c7c2f6018a9d9ad61caf87187c1969

.PHONY: help fmt lint test test-unit test-sdk build check vuln run up mock-up down logs smoke image scan

help:
	@echo 'make check       format, vet, race tests, and build (Go required)'
	@echo 'make vuln        scan reachable Go vulnerabilities'
	@echo 'make up/down     start/stop the local gateway (Docker required)'
	@echo 'make mock-up     start gateway and mock upstream for M1 exercises'
	@echo 'make smoke       check health and unimplemented API behavior'
	@echo 'make scan        build and scan the runtime image'

fmt:
	gofmt -w cmd internal tests/sdk

lint:
	@test -z "$$(gofmt -l cmd internal tests/sdk)" || { gofmt -l cmd internal tests/sdk; exit 1; }
	go vet ./...
	cd tests/sdk && go vet -mod=readonly ./...

test: test-unit test-sdk

test-unit:
	go test -race -count=1 -timeout=60s ./...

test-sdk:
	cd tests/sdk && go test -mod=readonly -race -count=1 -timeout=60s ./...

build:
	CGO_ENABLED=0 go build -trimpath -o bin/gateway ./cmd/gateway
	CGO_ENABLED=0 go build -trimpath -o bin/mockllm ./cmd/mockllm

check: lint test build

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

run:
	go run ./cmd/gateway -config config/gateway.example.json

up:
	$(COMPOSE) up --build -d --wait --wait-timeout 60 gateway

mock-up:
	$(COMPOSE) --profile mock up --build -d --wait --wait-timeout 60 gateway mockllm

down:
	$(COMPOSE) --profile mock down --remove-orphans

logs:
	$(COMPOSE) logs -f gateway

smoke:
	sh scripts/smoke.sh

image:
	docker build --tag $(IMAGE) .

scan: image
	mkdir -p test-results
	docker save --output test-results/gateway.tar $(IMAGE)
	docker run --rm -v "$(CURDIR)/test-results:/scans:ro" $(TRIVY_IMAGE) image --input /scans/gateway.tar --scanners vuln --severity HIGH,CRITICAL --exit-code 1 --no-progress
