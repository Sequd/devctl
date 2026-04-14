.PHONY: build run clean tidy install

BINARY_NAME = devctl
GOBIN ?= $(HOME)/go/bin

ifeq ($(OS),Windows_NT)
	BINARY = $(BINARY_NAME).exe
else
	BINARY = $(BINARY_NAME)
endif

CMD_PKG := $(shell find cmd -name 'main.go' -printf '%h\n' 2>/dev/null | head -1)

build: tidy
	go build -o $(BINARY) ./$(CMD_PKG)

run: build
	./$(BINARY)

install: build
	mkdir -p $(GOBIN)
	cp $(BINARY) $(GOBIN)/$(BINARY)
	@echo "Installed to $(GOBIN)/$(BINARY)"

tidy:
	go mod tidy

clean:
	rm -f $(BINARY_NAME) $(BINARY_NAME).exe
