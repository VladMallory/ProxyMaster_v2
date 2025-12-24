.PHONY: run
binary=ProxyMaster_v2
cmd=./cmd/myapp/main.go

run:
	@go run $(cmd)

run2:
	go run ./cmd/testLoginRemna/remna.go

run3:
	go run ./cmd/testGetClientInfoRemna/test.go

run4:
	go run ./cmd/testTransactions/testMain.go