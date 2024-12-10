.PHONY: all build

BUILD_OUT=bin

all: build

build:
	@echo "Building the application..."
	@mkdir -p $(BUILD_OUT)
	@go build -o $(GOPATH)/bin/mmr cmd/app/main.go
	@echo "Build completed."