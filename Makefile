.PHONY: build test clean

build:
	@./build.sh

test:
	@docker run --rm -v "$$(pwd)":/src -w /src -e CGO_ENABLED=0 golang:alpine \
		sh -c "go mod tidy && go vet ./... && go test ./..."

clean:
	rm -rf bin/
