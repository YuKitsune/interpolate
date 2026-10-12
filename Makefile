BINARY := interpolate
BUILD_DIR := bin
PREFIX ?= $(HOME)/.local

.PHONY: build test install clean

build:
	go build -o $(BUILD_DIR)/$(BINARY) .

test:
	go test ./...

install: build
	mkdir -p $(DESTDIR)$(PREFIX)/bin
	install -m 0755 $(BUILD_DIR)/$(BINARY) $(DESTDIR)$(PREFIX)/bin/$(BINARY)

clean:
	rm -rf $(BUILD_DIR)
	rm -f $(DESTDIR)$(PREFIX)/bin/$(BINARY)
