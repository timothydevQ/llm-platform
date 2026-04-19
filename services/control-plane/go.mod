module github.com/timothydevQ/llm-platform/services/control-plane

go 1.22

require (
	github.com/timothydevQ/llm-platform/gen v0.0.0
	google.golang.org/grpc v1.64.0
	modernc.org/sqlite v1.30.1
)

replace github.com/timothydevQ/llm-platform/gen => ../../gen/go
