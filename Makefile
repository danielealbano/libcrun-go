.PHONY: build test test-unit test-integration benchmark clean

build:
	go build ./...

test-unit:
	go test -v -race ./...

test-integration:
	@TEST_ROOTFS=$$(mktemp -d /tmp/test-rootfs-XXXXXX) && \
	echo "Setting up test rootfs at $$TEST_ROOTFS..." && \
	CONTAINER_ID=$$(docker create busybox:latest /bin/sh) && \
	docker export $$CONTAINER_ID | sudo tar -xf - -C $$TEST_ROOTFS && \
	docker rm $$CONTAINER_ID > /dev/null && \
	sudo chown -R root:root $$TEST_ROOTFS && \
	echo "Running integration tests..." && \
	sudo TEST_ROOTFS=$$TEST_ROOTFS go test -v -tags=integration ./... ; \
	EXIT_CODE=$$? ; \
	echo "Cleaning up $$TEST_ROOTFS..." && \
	sudo rm -rf $$TEST_ROOTFS ; \
	exit $$EXIT_CODE

test: test-unit

benchmark:
	@TEST_ROOTFS=$$(mktemp -d /tmp/test-rootfs-XXXXXX) && \
	echo "Setting up test rootfs at $$TEST_ROOTFS..." && \
	CONTAINER_ID=$$(docker create busybox:latest /bin/sh) && \
	docker export $$CONTAINER_ID | sudo tar -xf - -C $$TEST_ROOTFS && \
	docker rm $$CONTAINER_ID > /dev/null && \
	sudo chown -R root:root $$TEST_ROOTFS && \
	echo "Running benchmarks..." && \
	sudo TEST_ROOTFS=$$TEST_ROOTFS go test -tags=integration -bench=. -benchtime=1x -run=^$$ ./... ; \
	EXIT_CODE=$$? ; \
	echo "Cleaning up $$TEST_ROOTFS..." && \
	sudo rm -rf $$TEST_ROOTFS ; \
	exit $$EXIT_CODE

clean:
	go clean ./...
