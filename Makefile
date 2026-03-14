LOGIC ?= basic
BIN_DIR ?= bin

ENGINE_ARGS ?= --simulations 30 --parallel 5 --seed 50
GAME_ARGS ?= --max-turns 100

P0 ?= $(LOGIC)
P1 ?= opponent

export GOCACHE := $(CURDIR)/tmp/go-build
export GOMAXPROCS := 1

.PHONY: test build-agent build-opponent match match-bin bundle-% clean

test:
	go test ./...

bundle-agent:
	cgmerge --dir agent/$(LOGIC)/cmd --output agent/$(LOGIC)/bundle/bundle.go; \

build-agent: bundle-agent
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(LOGIC) ./agent/$(LOGIC)/bundle

build-opponent:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/opponent ./agent/opponent

match: build-agent build-opponent
	go run ./cmd/match \
		--p0-bin $(BIN_DIR)/$(LOGIC) \
		--p1-bin $(BIN_DIR)/opponent \
		$(ENGINE_ARGS) $(GAME_ARGS)

match-bin:
	go run ./cmd/match \
		--p0-bin $(BIN_DIR)/$(P0) \
		--p1-bin $(BIN_DIR)/$(P1) \
		$(ENGINE_ARGS) $(GAME_ARGS)

clean:
	rm -rf $(BIN_DIR)
