module github.com/weaveworks-liquidmetal/flintlock

go 1.18

replace (
	// We are pinning this for now, as one of our dependencies has this verison
	// as a dependency, and was breaking thanks to another dependency's use of
	// the v1.1.1 version. Ideally we should change this back asap.
	github.com/containernetworking/cni => github.com/containernetworking/cni v0.8.1
	github.com/containernetworking/plugins => github.com/containernetworking/plugins v0.8.1

	github.com/weaveworks-liquidmetal/flintlock/api => ./api
	github.com/weaveworks-liquidmetal/flintlock/client => ./client
)

require (
	github.com/Microsoft/go-winio v0.5.1 // indirect
	github.com/containerd/containerd v1.6.8
	github.com/containerd/typeurl v1.0.2
	github.com/firecracker-microvm/firecracker-go-sdk v0.22.0
	github.com/go-openapi/strfmt v0.21.1 // indirect
	github.com/go-playground/validator/v10 v10.11.0
	github.com/golang/mock v1.6.0
	github.com/google/go-cmp v0.5.8
	github.com/google/uuid v1.3.0 // indirect
	github.com/google/wire v0.5.0
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.11.2
	github.com/klauspost/compress v1.13.6 // indirect
	github.com/oklog/ulid v1.3.1
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.20.0
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.3-0.20211202183452-c5a74bcca799
	github.com/pelletier/go-toml v1.9.5
	github.com/prometheus/client_golang v1.13.0
	github.com/sirupsen/logrus v1.9.0
	github.com/spf13/afero v1.9.2
	github.com/spf13/cobra v1.5.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.12.0
	github.com/vishvananda/netlink v1.1.1-0.20210330154013-f5de75959ad5
	golang.org/x/net v0.0.0-20220624214902-1bab6f366d9e // indirect
	google.golang.org/grpc v1.48.0
	google.golang.org/protobuf v1.28.1
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/diskfs/go-diskfs v1.2.0
	github.com/docker/go-units v0.4.0
	github.com/gorilla/mux v1.8.0
	github.com/grpc-ecosystem/go-grpc-middleware v1.3.0
	github.com/urfave/cli/v2 v2.11.1
	github.com/weaveworks-liquidmetal/flintlock/api v0.0.0-00010101000000-000000000000
	github.com/weaveworks-liquidmetal/flintlock/client v0.0.0-00010101000000-000000000000
	github.com/yitsushi/file-tailor v1.0.0
	sigs.k8s.io/yaml v1.3.0
)

require (
	github.com/Microsoft/hcsshim v0.9.4 // indirect
	github.com/PuerkitoBio/purell v1.1.1 // indirect
	github.com/PuerkitoBio/urlesc v0.0.0-20170810143723-de5bf2ad4578 // indirect
	github.com/asaskevich/govalidator v0.0.0-20200907205600-7a23bdc65eef // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/containerd/cgroups v1.0.3 // indirect
	github.com/containerd/continuity v0.2.2 // indirect
	github.com/containerd/fifo v1.0.0 // indirect
	github.com/containerd/ttrpc v1.1.0 // indirect
	github.com/containernetworking/cni v1.1.1 // indirect
	github.com/containernetworking/plugins v1.1.1 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.2 // indirect
	github.com/docker/go-events v0.0.0-20190806004212-e31b211e4f1c // indirect
	github.com/fsnotify/fsnotify v1.5.4 // indirect
	github.com/go-openapi/analysis v0.19.10 // indirect
	github.com/go-openapi/errors v0.19.8 // indirect
	github.com/go-openapi/jsonpointer v0.19.3 // indirect
	github.com/go-openapi/jsonreference v0.19.3 // indirect
	github.com/go-openapi/loads v0.19.5 // indirect
	github.com/go-openapi/runtime v0.19.22 // indirect
	github.com/go-openapi/spec v0.19.8 // indirect
	github.com/go-openapi/swag v0.19.9 // indirect
	github.com/go-openapi/validate v0.19.11 // indirect
	github.com/go-playground/locales v0.14.0 // indirect
	github.com/go-playground/universal-translator v0.18.0 // indirect
	github.com/go-stack/stack v1.8.0 // indirect
	github.com/gofrs/uuid v3.3.0+incompatible // indirect
	github.com/gogo/googleapis v1.4.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/subcommands v1.0.1 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/leodido/go-urn v1.2.1 // indirect
	github.com/magiconair/properties v1.8.6 // indirect
	github.com/mailru/easyjson v0.7.1 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.2-0.20181231171920-c182affec369 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/moby/locker v1.0.1 // indirect
	github.com/moby/sys/mountinfo v0.5.0 // indirect
	github.com/moby/sys/signal v0.6.0 // indirect
	github.com/nxadm/tail v1.4.8 // indirect
	github.com/opencontainers/runc v1.1.2 // indirect
	github.com/opencontainers/runtime-spec v1.0.3-0.20210326190908-1c3f411f0417 // indirect
	github.com/opencontainers/selinux v1.10.1 // indirect
	github.com/pelletier/go-toml/v2 v2.0.1 // indirect
	github.com/pierrec/lz4 v2.3.0+incompatible // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pkg/xattr v0.4.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.37.0 // indirect
	github.com/prometheus/procfs v0.8.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/spf13/cast v1.5.0 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/subosito/gotenv v1.3.0 // indirect
	github.com/ulikunitz/xz v0.5.6 // indirect
	github.com/vishvananda/netns v0.0.0-20210104183010-2eb08e3e575f // indirect
	github.com/xrash/smetrics v0.0.0-20201216005158-039620a65673 // indirect
	go.mongodb.org/mongo-driver v1.7.5 // indirect
	go.opencensus.io v0.23.0 // indirect
	golang.org/x/crypto v0.0.0-20220411220226-7b82a4e95df4 // indirect
	golang.org/x/mod v0.4.2 // indirect
	golang.org/x/sync v0.0.0-20220601150217-0de741cfad7f // indirect
	golang.org/x/sys v0.0.0-20220715151400-c0bba94af5f8 // indirect
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/tools v0.1.5 // indirect
	golang.org/x/xerrors v0.0.0-20220517211312-f3a8303e98df // indirect
	google.golang.org/genproto v0.0.0-20220805133916-01dd62135a58 // indirect
	gopkg.in/djherbis/times.v1 v1.2.0 // indirect
	gopkg.in/ini.v1 v1.66.4 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)
