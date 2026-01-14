BINARY_NAME=kt

build:
	mkdir -p bin
	go build -o bin/${BINARY_NAME} ./cmd

release_windows:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags '-s -w -X main.version=v0.0.1' -o dist/kt_windows_amd64/kt.exe ./cmd/main.go

run:
	go run ./cmd 

clean:
	go clean
	rm ./bin/${BINARY_NAME}
