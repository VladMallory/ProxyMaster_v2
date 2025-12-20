.PHONY: run
binary=ProxyMaster_v2
cmd=./cmd/myapp/main.go

run:
	go run $(cmd)