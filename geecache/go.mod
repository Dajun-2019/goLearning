module example

go 1.18

require geecache v0.0.0

require (
	github.com/golang/protobuf v1.5.0 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
)

// require google.golang.org/protobuf v1.31.0 // indirect

replace geecache => ./geecache
