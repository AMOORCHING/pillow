.PHONY: build install test vet clean

build:
	go build -o bin/pillow ./cmd/pillow
	go build -o bin/pillowsensord ./cmd/pillowsensord

install:
	go install ./cmd/pillow
	go install ./cmd/pillowsensord

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -rf bin/
