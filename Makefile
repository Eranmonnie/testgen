test-all:
	go test ./... -v
build:
	go build -o testgen cmd/testgen/main.go
path:
	sudo mv testgen /usr/local/bin/