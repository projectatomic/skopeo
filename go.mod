module github.com/containers/skopeo

go 1.12

require (
	github.com/containers/buildah v1.8.4
	github.com/containers/image v1.5.2-0.20190821161828-0cc0e97405db
	github.com/containers/storage v1.13.2
	github.com/docker/docker v0.0.0-20180522102801-da99009bbb11
	github.com/go-check/check v0.0.0-20180628173108-788fd7840127
	github.com/opencontainers/go-digest v1.0.0-rc1
	github.com/opencontainers/image-spec v1.0.2-0.20190823105129-775207bd45b6
	github.com/opencontainers/image-tools v0.0.0-20170926011501-6d941547fa1d
	github.com/opencontainers/runtime-spec v1.0.0 // indirect
	github.com/pborman/uuid v0.0.0-20160209185913-a97ce2ca70fa // indirect
	github.com/pkg/errors v0.8.1
	github.com/sirupsen/logrus v1.4.2
	github.com/stretchr/testify v1.4.0
	github.com/syndtr/gocapability v0.0.0-20180916011248-d98352740cb2
	github.com/urfave/cli v1.20.0
	github.com/xeipuuv/gojsonschema v1.1.0 // indirect
	go4.org v0.0.0-20190218023631-ce4c26f7be8e // indirect
	golang.org/x/tools v0.0.0-20180917221912-90fa682c2a6e // indirect
	k8s.io/client-go v0.0.0-20181219152756-3dd551c0f083 // indirect
)

replace github.com/docker/libtrust => github.com/containers/libtrust v0.0.0-20190913040956-14b96171aa3b

replace github.com/containers/image => github.com/lsm5/image v1.5.2-0.20190916194126-2529ee26cce3
