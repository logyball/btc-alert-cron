up:
	docker-compose up -d

down:
	docker-compose down

start:
	go run main.go

build:
	GOOS=darwin GOARCH=amd64 go build -o btccron main.go 