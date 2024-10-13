module sigs.k8s.io/cloud-provider-azure/pkg/azclient/trace

go 1.21.1
toolchain go1.22.5

require (
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.13.0
	go.opentelemetry.io/otel v1.31.0
	go.opentelemetry.io/otel/metric v1.31.0
	go.opentelemetry.io/otel/trace v1.31.0
	k8s.io/klog/v2 v2.130.1
	sigs.k8s.io/cloud-provider-azure/pkg/azclient v0.0.32
)

require (
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.10.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	golang.org/x/net v0.27.0 // indirect
	golang.org/x/text v0.16.0 // indirect
)
