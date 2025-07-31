.PHONY: all build install clean

APP_NAME := nerdcan
BUILD_DIR := bin
INSTALL_DIR := /usr/local/bin

all: build

build:
	@echo "Building $(APP_NAME)..."
	go mod tidy
	go build -o $(BUILD_DIR)/$(APP_NAME) .

install: build
	@echo "Installing $(APP_NAME) to $(INSTALL_DIR)..."
	sudo mkdir -p $(INSTALL_DIR)
	sudo cp $(BUILD_DIR)/$(APP_NAME) $(INSTALL_DIR)
	@echo "$(APP_NAME) installed successfully!"

clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	@echo "Clean complete."
