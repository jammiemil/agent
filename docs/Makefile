IMAGE = grafana/docs-base:latest
CONTENT_PATH = /hugo/content/docs/agent/latest
PORT = 3002:3002

.PHONY: pull
pull:
	docker pull $(IMAGE)

.PHONY: docs
docs: pull
	docker run --init -v $(shell pwd)/flow:$(CONTENT_PATH)/flow:Z -v $(shell pwd)/sources:$(CONTENT_PATH):Z -p $(PORT) --rm -it $(IMAGE)
