generate:
	go generate ./...

install-bob:
	go get -tool github.com/stephenafamo/bob/gen/bobgen-psql@latest

bob:
	go tool github.com/stephenafamo/bob/gen/bobgen-psql

swagger:
	swag init -g internal/handler/handler.go

# Allure Go reporting
# Установка: go install github.com/robotomize/go-allure/cmd/golurectl@latest
# Для просмотра отчётов: brew install allure (macOS) или sudo apt-get install allure (ubuntu)

ALLURE_RESULTS := allure-results
TEST_OUTPUT := test-output.json

install-allure:
	brew install allure
	go install github.com/robotomize/go-allure/cmd/golurectl@latest
	@echo "Allure установлен! golurectl в $$(go env GOPATH)/bin"

test-allure:
	go test -json -cover ./... 2>&1 | tee $(TEST_OUTPUT)

generate-allure: test-allure
	golurectl -l -e -s -a -o $(ALLURE_RESULTS) < $(TEST_OUTPUT)

serve-allure:
	allure serve $(ALLURE_RESULTS)

generate-report: generate-allure
	allure generate $(ALLURE_RESULTS) -o allure-report

clean-allure:
	rm -rf $(ALLURE_RESULTS) allure-report $(TEST_OUTPUT)
