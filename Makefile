.PHONY: build

vendor:
	@go mod vendor

dependencies:
	@go mod download

fmt:
	@go fmt .

vet:
	@go vet .

build: clean vendor fmt vet
	@go build -mod=vendor -o bin/terraform-provider-coralogix .

clean:
	@rm -f bin/terraform-provider-coralogix

all: build