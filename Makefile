IMAGE_PREFIX?=opsmx11
APP?=argo-isd-metric-provider-job
IMAGE_TAG?=latest
DOCKER_PUSH?=false
CURRENT_DIR=$(shell pwd)
DIST_DIR=${CURRENT_DIR}/dist


.PHONY: build
## build: builds the application
build: clean
	@echo "Building..."
	@CGO_ENABLED=0 go build -v -o ${DIST_DIR}/${APP} *.go


.PHONY: clean
## clean: cleans the binary
clean:
	@echo "Cleaning the binary"
	rm -rf ${DIST_DIR}
	

.PHONY: test
## test: runs go test with default values
test:
	go test -v -count=1 -race ./...


.PHONY: setup
## setup: setup go modules
setup:
		go mod tidy \
		&& go mod vendor


.PHONY: image
## image: builds the docker image
image:
	DOCKER_BUILDKIT=1 docker build -t ${IMAGE_PREFIX}/${APP}:${IMAGE_TAG}  .
	@if [ "${DOCKER_PUSH}" = "true" ] ; then docker push ${IMAGE_PREFIX}/${APP}:${IMAGE_TAG} ; fi


.PHONY: lint
## lint: runs the linter
lint: setup
	golangci-lint run


.PHONY: help
## help: prints this help message
help:
	@echo "Usage:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'