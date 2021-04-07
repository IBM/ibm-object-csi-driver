module github.com/IBM/satellite-object-storage-plugin

go 1.15

replace (
	k8s.io/api => k8s.io/api v0.0.0-20190516230258-a675ac48af67
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190313205120-d7deff9243b1
	k8s.io/apiserver => k8s.io/apiserver v0.0.0-20190313205120-8b27c41bdbb1
)

require (
	github.com/IBM/ibm-cos-sdk-go v1.6.0
	github.com/IBM/ibmcloud-storage-volume-lib v0.0.2
	github.com/IBM/ibmcloud-volume-interface v1.0.0-beta4
	github.com/container-storage-interface/spec v1.2.0
	github.com/ctrox/csi-s3 v1.1.1
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/kubernetes-csi/drivers v1.0.2
	github.com/mitchellh/go-ps v0.0.0-20170309133038-4fdf99ab2936
	github.com/prometheus/client_golang v1.9.0
	github.com/stretchr/testify v1.6.1
	go.uber.org/zap v1.16.0
	golang.org/x/net v0.0.0-20200707034311-ab3426394381
	google.golang.org/grpc v1.27.0
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
	k8s.io/klog v0.2.0
	k8s.io/klog/v2 v2.8.0
	k8s.io/kubernetes v1.14.2
	k8s.io/utils v0.0.0-20210305010621-2afb4311ab10 // indirect
)
