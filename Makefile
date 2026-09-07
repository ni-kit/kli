export GOPROXY ?= https://proxy.golang.org,direct

install:
	go install ./cmd/kli