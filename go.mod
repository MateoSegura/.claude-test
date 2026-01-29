module github.com/MateoSegura/.claude-test

go 1.24.0

require (
	github.com/MateoSegura/claudesdk-go v0.0.0
	golang.org/x/sync v0.19.0
	gopkg.in/yaml.v3 v3.0.1
)

replace github.com/MateoSegura/claudesdk-go => ./claudesdk-go
