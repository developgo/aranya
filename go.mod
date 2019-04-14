module arhat.dev/aranya

go 1.12

replace (
	cloud.google.com/go => cloud.google.com/go v0.0.0-20160913182117-3b1ae45394a2
	github.com/Nvveen/Gotty => github.com/Nvveen/Gotty v0.0.0-20120604004816-cd527374f1e5
	github.com/PuerkitoBio/purell => github.com/PuerkitoBio/purell v1.1.1
	github.com/blang/semver => github.com/blang/semver v3.5.1+incompatible
	github.com/boltdb/bolt => github.com/boltdb/bolt v1.3.1
	github.com/containerd/containerd => github.com/containerd/containerd v1.2.6
	github.com/containernetworking/cni => github.com/containernetworking/cni v0.6.0
	github.com/containers/libpod => github.com/containers/libpod v1.1.2
	github.com/cyphar/filepath-securejoin => github.com/cyphar/filepath-securejoin v0.2.2
	github.com/docker/distribution => github.com/docker/distribution v2.7.1-0.20190205005809-0d3efadf0154+incompatible
	github.com/docker/docker => github.com/docker/engine v0.0.0-20190408150954-50ebe4562dfc
	github.com/docker/docker/builder/dockerfile/parser => github.com/moby/buildkit v0.4.0
	github.com/docker/spdystream => github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c
	github.com/elazarl/goproxy => github.com/elazarl/goproxy v0.0.0-20181111060418-2ce16c963a8a
	github.com/emicklei/go-restful => github.com/emicklei/go-restful v2.9.0+incompatible
	github.com/fatih/camelcase => github.com/fatih/camelcase v1.0.0
	github.com/fsnotify/fsnotify => github.com/fsnotify/fsnotify v1.4.7
	github.com/ghodss/yaml => github.com/ghodss/yaml v0.0.0-20180820084758-c7ce16629ff4
	github.com/go-openapi/jsonpointer => github.com/go-openapi/jsonpointer v0.19.0
	github.com/go-openapi/jsonreference => github.com/go-openapi/jsonreference v0.19.0
	github.com/go-openapi/spec => github.com/go-openapi/spec v0.19.0
	github.com/go-openapi/swag => github.com/go-openapi/swag v0.19.0
	github.com/gogo/protobuf => github.com/gogo/protobuf v1.2.1
	github.com/golang/groupcache => github.com/golang/groupcache v0.0.0-20190129154638-5b532d6fd5ef
	github.com/golang/mock => github.com/golang/mock v1.2.0
	github.com/golang/protobuf => github.com/golang/protobuf v1.3.1
	github.com/google/btree => github.com/google/btree v0.0.0-20160524151835-7d79101e329e
	github.com/google/gofuzz => github.com/google/gofuzz v1.0.0
	github.com/googleapis/gnostic => github.com/googleapis/gnostic v0.2.0
	github.com/gorilla/mux => github.com/gorilla/mux v1.7.0
	github.com/gregjones/httpcache => github.com/gregjones/httpcache v0.0.0-20170728041850-787624de3eb7
	github.com/imdario/mergo => github.com/imdario/mergo v0.3.7
	github.com/inconshreveable/mousetrap => github.com/inconshreveable/mousetrap v1.0.0
	github.com/json-iterator/go => github.com/json-iterator/go v1.1.5
	github.com/kisielk/sqlstruct => github.com/kisielk/sqlstruct v0.0.0-20150923205031-648daed35d49
	github.com/kr/fs => github.com/kr/fs v0.1.0
	github.com/lib/pq => github.com/lib/pq v1.0.0
	github.com/lithammer/dedent => github.com/lithammer/dedent v1.1.0
	github.com/mattn/go-shellwords => github.com/mattn/go-shellwords v1.0.5
	github.com/mistifyio/go-zfs => github.com/mistifyio/go-zfs v2.1.1+incompatible
	github.com/modern-go/concurrent => github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd
	github.com/modern-go/reflect2 => github.com/modern-go/reflect2 v1.0.1
	github.com/opencontainers/runc => github.com/opencontainers/runc v1.0.0-rc6
	github.com/opencontainers/runtime-spec => github.com/opencontainers/runtime-spec v1.0.1
	github.com/opencontainers/selinux => github.com/opencontainers/selinux v1.0.0
	github.com/pborman/uuid => github.com/pborman/uuid v0.0.0-20180906182336-adf5a7427709
	github.com/peterbourgon/diskv => github.com/peterbourgon/diskv v2.0.1+incompatible
	github.com/pkg/sftp => github.com/pkg/sftp v1.10.0
	github.com/pquerna/ffjson => github.com/pquerna/ffjson v0.0.0-20181028064349-e517b90714f7
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v0.9.2
	github.com/prometheus/client_model => github.com/prometheus/client_model v0.0.0-20190129233127-fd36f4220a90
	github.com/prometheus/procfs => github.com/prometheus/procfs v0.0.0-20190209105433-f8d8b3f739bd
	github.com/renstrom/dedent => github.com/lithammer/dedent v1.1.0
	github.com/satori/go.uuid => github.com/satori/go.uuid v1.2.0
	github.com/seccomp/libseccomp-golang => github.com/seccomp/libseccomp-golang v0.9.0
	github.com/sigma/go-inotify => github.com/sigma/go-inotify v0.0.0-20181102212354-c87b6cf5033d
	github.com/spf13/afero => github.com/spf13/afero v1.2.1
	github.com/spf13/cobra => github.com/spf13/cobra v0.0.3
	github.com/spf13/pflag => github.com/spf13/pflag v1.0.3
	github.com/stretchr/testify => github.com/stretchr/testify v1.3.0
	github.com/syndtr/gocapability => github.com/syndtr/gocapability v0.0.0-20180916011248-d98352740cb2
	go.uber.org/atomic => go.uber.org/atomic v1.3.2
	go.uber.org/multierr => go.uber.org/multierr v1.1.0
	go.uber.org/zap => go.uber.org/zap v1.9.1
	golang.org/x/net => github.com/golang/net v0.0.0-20190311183353-d8887717615a
	golang.org/x/oauth2 => github.com/golang/oauth2 v0.0.0-20190226205417-e64efc72b421
	golang.org/x/sync => github.com/golang/sync v0.0.0-20190227155943-e225da77a7e6
	google.golang.org/genproto => github.com/google/go-genproto v0.0.0-20190307195333-5fe7a883aa19
	google.golang.org/grpc => github.com/grpc/grpc-go v1.19.1
	gopkg.in/inf.v0 => gopkg.in/inf.v0 v0.9.0
	gopkg.in/yaml.v2 => gopkg.in/yaml.v2 v2.2.2
	sigs.k8s.io/yaml => github.com/kubernetes-sigs/yaml v1.1.0
)

// Kubernetes v1.13.5
replace (
	k8s.io/api => github.com/kubernetes/api v0.0.0-20190222213804-5cb15d344471
	k8s.io/apiextensions-apiserver => github.com/kubernetes/apiextensions-apiserver v0.0.0-20190325193600-475668423e9f
	k8s.io/apimachinery => github.com/kubernetes/apimachinery v0.0.0-20190221213512-86fb29eff628
	k8s.io/apiserver => github.com/kubernetes/apiserver v0.0.0-20190319190228-a4358799e4fe
	k8s.io/client-go => github.com/kubernetes/client-go v2.0.0-alpha.0.0.20190307161346-7621a5ebb88b+incompatible
	k8s.io/cloud-provider => github.com/kubernetes/cloud-provider v0.0.0-20190325195930-a624236cb1f2
	k8s.io/klog => github.com/kubernetes/klog v0.2.0
	k8s.io/kubernetes => github.com/kubernetes/kubernetes v1.13.5
	k8s.io/utils => github.com/kubernetes/utils v0.0.0-20190221042446-c2654d5206da
)

require (
	cloud.google.com/go v0.36.0 // indirect
	github.com/14rcole/gopopulate v0.0.0-20180821133914-b175b219e774 // indirect
	github.com/Microsoft/go-winio v0.4.12 // indirect
	github.com/Microsoft/hcsshim v0.8.6 // indirect
	github.com/Nvveen/Gotty v0.0.0-20120604004816-cd527374f1e5 // indirect
	github.com/PuerkitoBio/purell v1.1.1 // indirect
	github.com/VividCortex/ewma v1.1.1 // indirect
	github.com/appscode/jsonpatch v0.0.0-20190108182946-7c0e3b262f30 // indirect
	github.com/blang/semver v3.5.1+incompatible // indirect
	github.com/boltdb/bolt v1.3.1 // indirect
	github.com/checkpoint-restore/go-criu v0.0.0-20190109184317-bdb7599cd87b // indirect
	github.com/containerd/cgroups v0.0.0-20190226200435-dbea6f2bd416 // indirect
	github.com/containerd/containerd v1.3.0-0.20190212172151-f5b0fa220df8
	github.com/containernetworking/cni v0.6.0 // indirect
	github.com/containernetworking/plugins v0.7.5 // indirect
	github.com/containers/buildah v1.7.1 // indirect
	github.com/containers/image v0.0.0-20190313194849-a911b201c9ed
	github.com/containers/libpod v1.1.2
	github.com/containers/psgo v0.0.0-20190311163040-ee081b685b16 // indirect
	github.com/containers/storage v0.0.0-20190311173742-25923caa130d
	github.com/coreos/go-iptables v0.4.0 // indirect
	github.com/coreos/prometheus-operator v0.26.0 // indirect
	github.com/cri-o/ocicni v0.1.0
	github.com/cyphar/filepath-securejoin v0.2.2 // indirect
	github.com/denisbrodbeck/machineid v1.0.1
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/docker v1.13.1
	github.com/docker/docker-credential-helpers v0.6.1 // indirect
	github.com/docker/docker/builder/dockerfile/parser v1.13.1 // indirect
	github.com/docker/go-connections v0.4.0
	github.com/docker/go-metrics v0.0.0-20181218153428-b84716841b82 // indirect
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7 // indirect
	github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c // indirect
	github.com/elazarl/goproxy v0.0.0-20181111060418-2ce16c963a8a // indirect
	github.com/emicklei/go-restful v2.9.0+incompatible // indirect
	github.com/fatih/camelcase v1.0.0 // indirect
	github.com/fsnotify/fsnotify v1.4.7
	github.com/fsouza/go-dockerclient v1.3.6 // indirect
	github.com/ghodss/yaml v0.0.0-00010101000000-000000000000 // indirect
	github.com/go-logr/logr v0.1.0
	github.com/go-logr/zapr v0.1.0 // indirect
	github.com/go-openapi/jsonpointer v0.18.0 // indirect
	github.com/go-openapi/jsonreference v0.18.0 // indirect
	github.com/go-openapi/spec v0.0.0-20190124011800-53d776530bf7
	github.com/go-openapi/swag v0.18.0 // indirect
	github.com/gogo/protobuf v1.2.0
	github.com/golang/groupcache v0.0.0-20190129154638-5b532d6fd5ef // indirect
	github.com/golang/mock v1.2.0 // indirect
	github.com/golang/protobuf v1.3.1 // indirect
	github.com/google/btree v0.0.0-00010101000000-000000000000 // indirect
	github.com/google/gofuzz v1.0.0 // indirect
	github.com/google/uuid v1.1.1 // indirect
	github.com/googleapis/gnostic v0.2.0 // indirect
	github.com/gorilla/mux v1.7.0
	github.com/gregjones/httpcache v0.0.0-00010101000000-000000000000 // indirect
	github.com/guregu/null v3.4.0+incompatible // indirect
	github.com/hashicorp/go-multierror v1.0.0 // indirect
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/json-iterator/go v1.1.5 // indirect
	github.com/klauspost/compress v1.4.1 // indirect
	github.com/klauspost/cpuid v1.2.0 // indirect
	github.com/klauspost/pgzip v1.2.1 // indirect
	github.com/lib/pq v0.0.0-00010101000000-000000000000 // indirect
	github.com/mattn/go-isatty v0.0.7 // indirect
	github.com/mattn/go-shellwords v1.0.5 // indirect
	github.com/mistifyio/go-zfs v2.1.1+incompatible // indirect
	github.com/modern-go/concurrent v0.0.0-00010101000000-000000000000 // indirect
	github.com/modern-go/reflect2 v0.0.0-00010101000000-000000000000 // indirect
	github.com/mtrmac/gpgme v0.0.0-20170102180018-b2432428689c // indirect
	github.com/opencontainers/runc v1.0.0-rc6 // indirect
	github.com/opencontainers/runtime-spec v1.0.1
	github.com/opencontainers/runtime-tools v0.9.0 // indirect
	github.com/opencontainers/selinux v1.0.0 // indirect
	github.com/openshift/imagebuilder v0.0.0-20190308124740-705fe9255c57 // indirect
	github.com/opentracing/opentracing-go v1.0.2 // indirect
	github.com/operator-framework/operator-sdk v0.5.0
	github.com/ostreedev/ostree-go v0.0.0-20181213164143-d0388bd827cf // indirect
	github.com/pborman/uuid v0.0.0-20180906182336-adf5a7427709 // indirect
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/phayes/freeport v0.0.0-20180830031419-95f893ade6f2
	github.com/pquerna/ffjson v0.0.0-20181028064349-e517b90714f7 // indirect
	github.com/prometheus/client_golang v0.9.3-0.20190127221311-3c4408c8b829 // indirect
	github.com/prometheus/client_model v0.0.0-20190129233127-fd36f4220a90 // indirect
	github.com/prometheus/procfs v0.0.0-20190209105433-f8d8b3f739bd // indirect
	github.com/satori/go.uuid v1.2.0
	github.com/seccomp/containers-golang v0.0.0-20190312124753-8ca8945ccf5f // indirect
	github.com/seccomp/libseccomp-golang v0.9.0 // indirect
	github.com/sigma/go-inotify v0.0.0-20181102212354-c87b6cf5033d // indirect
	github.com/spf13/afero v1.2.1 // indirect
	github.com/spf13/cobra v0.0.3
	github.com/spf13/pflag v1.0.3
	github.com/stretchr/testify v1.3.0
	github.com/syndtr/gocapability v0.0.0-20180916011248-d98352740cb2 // indirect
	github.com/tchap/go-patricia v2.3.0+incompatible // indirect
	github.com/ulikunitz/xz v0.5.6 // indirect
	github.com/ulule/deepcopier v0.0.0-20171107155558-ca99b135e50f // indirect
	github.com/vbatts/tar-split v0.11.1 // indirect
	github.com/vbauerster/mpb v3.3.4+incompatible // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20180127040702-4e3ac2762d5f // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xeipuuv/gojsonschema v1.1.0 // indirect
	go.uber.org/atomic v1.3.2 // indirect
	go.uber.org/multierr v1.1.0 // indirect
	go.uber.org/zap v1.9.1 // indirect
	golang.org/x/net v0.0.0-20190311183353-d8887717615a // indirect
	golang.org/x/oauth2 v0.0.0-20190226205417-e64efc72b421 // indirect
	google.golang.org/genproto v0.0.0-20190307195333-5fe7a883aa19 // indirect
	google.golang.org/grpc v1.19.1
	gopkg.in/inf.v0 v0.0.0-00010101000000-000000000000 // indirect
	gopkg.in/yaml.v2 v2.2.2
	k8s.io/api v0.0.0-20190408172450-b1350b9e3bc2
	k8s.io/apiextensions-apiserver v0.0.0-20190325193600-475668423e9f // indirect
	k8s.io/apimachinery v0.0.0-20190408172355-3115ef20f323
	k8s.io/apiserver v0.0.0-20190319190228-a4358799e4fe // indirect
	k8s.io/client-go v2.0.0-alpha.0.0.20190228174230-b40b2a5939e4+incompatible
	k8s.io/cloud-provider v0.0.0-20190325195930-a624236cb1f2 // indirect
	k8s.io/csi-api v0.0.0-20190325194237-b07135bbe9d0 // indirect
	k8s.io/klog v0.2.0 // indirect
	k8s.io/kube-openapi v0.0.0-20190228160746-b3a7cee44a30
	k8s.io/kubernetes v1.13.5
	k8s.io/utils v0.0.0-20190221042446-c2654d5206da
	sigs.k8s.io/controller-runtime v0.1.10
	sigs.k8s.io/testing_frameworks v0.1.1 // indirect
	sigs.k8s.io/yaml v0.0.0-00010101000000-000000000000 // indirect
)
