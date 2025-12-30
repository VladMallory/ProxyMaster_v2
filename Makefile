.PHONY: run
binary=ProxyMaster_v2
cmdMacosAndLinux=./cmd/myapp/main.go
cmdWindows=.\cmd\myapp\main.go

run:
	@clear
	@go run $(cmdMacosAndLinux)

windows:
	go run $(cmdWindows)

 run2:
	go run ./cmd/testGetUserInfo/testMain.go

# docker
# натив
docker-build:
	docker build -t proxymaster_v2 .

docker: docker-build
	docker run --env-file .env proxymaster_v2

# эмуляция под linux
docker-build-linux:
	docker build --platform linux/amd64 -t proxymaster_v2 .

docker-linux: docker-build-linux
	docker run --platform linux/amd64 --env-file .env proxymaster_v2 

# Проверки и прочее
gosec:
	@clear
	gosec ./...

list:
	@clear
	golangci-lint run