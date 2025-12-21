.PHONY: run
binary=ProxyMaster_v2
cmd=./cmd/myapp/main.go

run:
	clear
	go run $(cmd)

run2:
	clear
	go run ./cmd/testLoginRemna/remna.go

run3:
	clear
	go run ./cmd/test/test1.go