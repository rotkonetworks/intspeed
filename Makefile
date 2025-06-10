.PHONY: build clean install

BINARY_CLI=intspeed
BINARY_SERVER=intspeed-server
BUILD_DIR=build

build:
	@mkdir -p $(BUILD_DIR)
	@echo "Building CLI..."
	@go build -ldflags "-s -w" -o $(BUILD_DIR)/$(BINARY_CLI) ./cmd/cli
	@echo "Building server..."
	@go build -ldflags "-s -w" -o $(BUILD_DIR)/$(BINARY_SERVER) ./cmd/server

clean:
	@rm -rf $(BUILD_DIR)

install: build
	@echo "Installing..."
	@sudo cp $(BUILD_DIR)/$(BINARY_CLI) /usr/local/bin/
	@sudo cp $(BUILD_DIR)/$(BINARY_SERVER) /usr/local/bin/
