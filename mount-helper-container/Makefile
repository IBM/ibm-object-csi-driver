.PHONY: coverage
coverage: test
	cat coverage.out | grep -v /fake > cover.out;
	# go tool cover -html=cover.out -o=cover.html
	go tool cover -func=cover.out | fgrep total

ut-coverage: go test -v -timeout 1800s -coverprofile=cover.out ./...
	go tool cover -html=cover.out -o=cover.html
	@./scripts/coverage.sh
	rm cover.html cover.out
	go mod tidy
