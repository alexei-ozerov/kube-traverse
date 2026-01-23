BINARY_NAME=kt

build:
	go build -o dist/${BINARY_NAME} ./cmd

run:
	go run ./cmd 

clean:
	go clean
	rm -rf dist
	rm -f ~/.kube/traverse_cache.json # App cache
	rm -f *.log # Kube-logs dumped by kt
	rm -rf logs # App logs
