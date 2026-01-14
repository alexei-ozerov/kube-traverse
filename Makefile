BINARY_NAME=kt

build:
	mkdir -p bin
	go build -o bin/${BINARY_NAME} ./cmd

run:
	go run ./cmd 

clean:
	go clean
	rm ./bin/${BINARY_NAME}
