.DEFAULT_GOAL := help

.PHONY: all
all:

.PHONY: build
build: ## Build for local environment
	@go build

.PHONY: release
release: ## Build for multiple OSs, packaging it and upload to GitHub Release
	@bash <(wget -o /dev/null -qO - https://git.io/release-go)

.PHONY: help
help: ## Self-documented Makefile
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| sort \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
