PLUGIN_NAME ?= akerouanton/fluentd-async-logger
PLUGIN_VERSION ?= devel
PLUGIN := $(PLUGIN_NAME):$(PLUGIN_VERSION)
ROOTFS := $(PWD)/plugin/rootfs/
TMP_IMAGE := fluentd-async-logger
TMP_CONTAINER := fluentd-async-logger-rootfs

.PHONY: build
build:
	docker build -t $(TMP_IMAGE) -f Dockerfile.build .
	-docker rm -f $(TMP_CONTAINER)
	docker create --name $(TMP_CONTAINER) $(TMP_IMAGE)
	docker cp $(TMP_CONTAINER):/app/plugin ./

.PHONY: plugin
plugin: build
	-docker plugin disable $(PLUGIN)
	-docker plugin rm $(PLUGIN)
	docker plugin create $(PLUGIN) plugin/

.PHONY: install
install: plugin
	docker plugin enable $(PLUGIN)

.PHONY: release
release: plugin
	docker plugin release $(PLUGIN)

.PHONY: lint
lint:
	docker run --rm -v $(PWD):/app -w /app golangci/golangci-lint:v1.24-alpine golangci-lint run -v
