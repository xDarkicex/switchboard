module github.com/xDarkicex/switchboard

go 1.25.7

replace (
	github.com/xDarkicex/logic => ../logic
	github.com/xDarkicex/memory => ../memory
	github.com/xDarkicex/nanite => ../nanite
	github.com/xDarkicex/nanite/quic => ../nanite/quic
	github.com/xDarkicex/nanite/sse => ../nanite/sse
	github.com/xDarkicex/steady => ../steady
)

require (
	github.com/spf13/cobra v1.10.2
	github.com/xDarkicex/logic v0.0.0-00010101000000-000000000000
	github.com/xDarkicex/nanite v0.0.0
	github.com/xDarkicex/nanite/sse v0.0.0-20260628084004-4a3045a31e5c
	github.com/xDarkicex/steady v0.0.0-00010101000000-000000000000
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	github.com/xDarkicex/memory v1.2.7 // indirect
	golang.org/x/sys v0.45.0 // indirect
)
