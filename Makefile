.PHONY: build vet test test-race cover bench bench-e2e fuzz cover-func

build:
	go build ./...

vet:
	go vet ./...

test:
	go test ./...

test-race:
	go test -race ./...

cover:
	go test -cover ./...

# micro benchmarks (codec / dispatch / pool / log / config / metrics)
bench:
	go test ./knet/ ./kpool/ ./klog/ ./kconf/ ./kmetrics/ -bench . -benchmem -benchtime=2s

# end-to-end throughput incl. raw-TCP baseline (-count=3, take median)
bench-e2e:
	go test ./knet/ -bench 'RawEchoBaseline|EchoThroughput|MultiConnEcho' -benchmem -benchtime=1s -count=3

# fuzz smoke (30s per target)
fuzz:
	go test ./knet/ -run '^$$' -fuzz=FuzzTLVPackDecode -fuzztime=30s
	go test ./klog/ -run '^$$' -fuzz=FuzzRingBuffer -fuzztime=30s
	go test ./kconf/ -run '^$$' -fuzz=FuzzLoadYAML -fuzztime=30s

# coverage report for the core packages (gate: each >= 70%)
cover-func:
	go test ./knet/ ./kconf/ ./kmetrics/ ./klog/ ./kpool/ -coverprofile=coverage.out
	go tool cover -func=coverage.out
