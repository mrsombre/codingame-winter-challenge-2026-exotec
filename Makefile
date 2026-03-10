LOGIC ?= basic
BIN_DIR ?= bin

.PHONY: test build-agent clean

test:
	env GOCACHE=/tmp/go-build go test ./...

build-agent:
	mkdir -p $(BIN_DIR)
	env GOCACHE=/tmp/go-build go build -o $(BIN_DIR)/$(LOGIC) ./agent/$(LOGIC)

clean:
	rm -rf $(BIN_DIR)
