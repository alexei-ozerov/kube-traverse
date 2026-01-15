BINARY_NAME=kt

build:
	go build -o dist/${BINARY_NAME} ./cmd

run:
	go run ./cmd 

clean:
	go clean
	rm -rf dist
	rm 2026*.log
