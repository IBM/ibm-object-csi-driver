test:
	go test -v -timeout 1800s -coverprofile=cover.out ./...
	go tool cover -html=cover.out -o=cover.html

ut-coverage: test
	@./scripts/coverage.sh
	rm cover.html cover.out
	go mod tidy
