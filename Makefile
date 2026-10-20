# Gates (see CLAUDE.md): lint, test, docker-build.
API_DIR := services/api
ANALYZER_DIR := services/analyzer
VENV := $(ANALYZER_DIR)/.venv
PYTHON ?= $(if $(wildcard $(VENV)/bin/python),$(abspath $(VENV)/bin/python),python3)

.PHONY: setup lint test docker-build lint-go lint-py test-go test-py

setup:
	python3 -m venv $(VENV)
	$(VENV)/bin/pip install -q -e '$(ANALYZER_DIR)[dev]'

lint: lint-go lint-py

lint-go:
	@out=$$(cd $(API_DIR) && gofmt -l .); if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi
	cd $(API_DIR) && go vet ./...

lint-py:
	cd $(ANALYZER_DIR) && $(PYTHON) -m ruff check . && $(PYTHON) -m ruff format --check .

test: test-go test-py

test-go:
	cd $(API_DIR) && go test ./...

test-py:
	cd $(ANALYZER_DIR) && $(PYTHON) -m pytest -q

docker-build:
	docker build -t text-analyzer:dev $(ANALYZER_DIR)
