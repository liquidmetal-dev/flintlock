TEST_E2E_DIR := test/e2e

.PHONY: test-e2e ## Run e2e tests
test-e2e: ## Run e2e tests
	go test -timeout 30m -p 1 -v -tags=e2e ./test/e2e/...

.PHONY: compile-e2e
compile-e2e: # Test e2e compilation
	go test -c -o /dev/null -tags=e2e ./test/e2e
