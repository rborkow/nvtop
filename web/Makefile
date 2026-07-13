IMAGE ?= ghcr.io/rborkow/nvtop-web
TAG   ?= dev

.PHONY: test run-replay docker-amd64 docker-push-dev

test:
	gofmt -l . && test -z "$$(gofmt -l .)"
	go vet ./...
	go test ./...

run-replay:
	go run ./cmd/nvtop-web --replay internal/collector/testdata/snapshot_890m.json --interval 1s

docker-amd64:
	docker buildx build --platform linux/amd64 -f Dockerfile -t $(IMAGE):$(TAG) --load ..

docker-push-dev:
	docker buildx build --platform linux/amd64 -f Dockerfile -t $(IMAGE):$(TAG) --push ..
