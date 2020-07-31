.PHONY: setup
setup:
	mkdir -p ./bin

.PHONY: compile
compile: setup  ## Compiles the binary
	go build -o ./bin/${SERVICE_NAME} ./cmd/xds-client

default: compile