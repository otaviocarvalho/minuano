BINARY = minuano

.PHONY: build install clean vet test

build:
	go build -o $(BINARY) ./cmd/minuano
	go install ./cmd/minuano

install:
	go install ./cmd/minuano

clean:
	rm -f $(BINARY)

vet:
	go vet ./...

test:
	go test ./...
