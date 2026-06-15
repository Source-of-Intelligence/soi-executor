module github.com/Source-of-Intelligence/soi-executor

go 1.25.0

require (
	github.com/Source-of-Intelligence/soi-vos v1.0.0
	github.com/fsnotify/fsnotify v1.10.1
	github.com/tetratelabs/wazero v1.12.0
)

require golang.org/x/sys v0.44.0 // indirect

replace github.com/Source-of-Intelligence/soi-vos => ../soi-vos
