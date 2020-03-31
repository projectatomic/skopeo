module github.com/containers/skopeo

go 1.12

require (
	github.com/containerd/containerd v1.3.0 // indirect
	github.com/containers/image/v5 v5.3.1
	github.com/containers/ocicrypt v1.0.1
	github.com/containers/storage v1.18.1
	github.com/docker/docker v1.4.2-0.20191219165747-a9416c67da9f
	github.com/dsnet/compress v0.0.1 // indirect
	github.com/go-check/check v0.0.0-20180628173108-788fd7840127
	github.com/google/go-cmp v0.3.1 // indirect
	github.com/opencontainers/go-digest v1.0.0-rc1
	github.com/opencontainers/image-spec v1.0.2-0.20190823105129-775207bd45b6
	github.com/opencontainers/image-tools v0.0.0-20170926011501-6d941547fa1d
	github.com/opencontainers/runtime-spec v1.0.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/russross/blackfriday v2.0.0+incompatible // indirect
	github.com/sirupsen/logrus v1.5.0
	github.com/stretchr/testify v1.5.1
	github.com/syndtr/gocapability v0.0.0-20180916011248-d98352740cb2
	github.com/urfave/cli v1.22.1
	go4.org v0.0.0-20190218023631-ce4c26f7be8e // indirect
	gopkg.in/yaml.v2 v2.2.8
)

replace github.com/containers/image/v5 => github.com/mtrmac/image/v5 v5.4.0
