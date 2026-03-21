generate:
	go generate ./...

install-bob:
	go get -tool github.com/stephenafamo/bob/gen/bobgen-psql@latest

bob:
	go tool github.com/stephenafamo/bob/gen/bobgen-psql

swagger:
	swag init -g internal/handler/handler.go
