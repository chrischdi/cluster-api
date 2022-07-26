package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimecatalog "sigs.k8s.io/cluster-api/exp/runtime/catalog"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	"sigs.k8s.io/cluster-api/exp/runtime/server"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	catalog  = runtimecatalog.New()
	setupLog = ctrl.Log.WithName("setup")

	// Flags.
	profilerAddress string
	webhookPort     int
	webhookCertDir  string
	logOptions      = logs.NewOptions()
)

func init() {
	// Register the Runtime Hook types into the catalog.
	_ = runtimehooksv1.AddToCatalog(catalog)
}

// InitFlags initializes the flags.
func InitFlags(fs *pflag.FlagSet) {
	logs.AddFlags(fs, logs.SkipLoggingConfigurationFlags())
	logOptions.AddFlags(fs)

	fs.StringVar(&profilerAddress, "profiler-address", "",
		"Bind address to expose the pprof profiler (e.g. localhost:6060)")

	fs.IntVar(&webhookPort, "webhook-port", 9443,
		"Webhook Server port")

	fs.StringVar(&webhookCertDir, "webhook-cert-dir", "/tmp/k8s-webhook-server/serving-certs/",
		"Webhook cert dir, only used when webhook-port is specified.")
}

func main() {
	InitFlags(pflag.CommandLine)
	pflag.CommandLine.SetNormalizeFunc(cliflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

	if err := logOptions.ValidateAndApply(nil); err != nil {
		setupLog.Error(err, "unable to start extension")
		os.Exit(1)
	}

	// klog.Background will automatically use the right logger.
	ctrl.SetLogger(klog.Background())

	restConfig, err := ctrl.GetConfig()
	if err != nil {
		setupLog.Error(err, "error getting config for the cluster")
		os.Exit(1)
	}

	sc := runtime.NewScheme()
	_ = clusterv1.AddToScheme(sc)

	c, err := client.New(restConfig, client.Options{Scheme: sc})
	if err != nil {
		setupLog.Error(err, "error creating client to the cluster")
		os.Exit(1)
	}

	if profilerAddress != "" {
		klog.Infof("Profiler listening for requests at %s", profilerAddress)
		go func() {
			klog.Info(http.ListenAndServe(profilerAddress, nil))
		}()
	}

	ctx := ctrl.SetupSignalHandler()

	webhookServer, err := server.NewServer(server.Options{
		Catalog: catalog,
		Port:    webhookPort,
		CertDir: webhookCertDir,
	})
	if err != nil {
		setupLog.Error(err, "error creating webhook server")
		os.Exit(1)
	}

	h := handler{c}

	// Register extension handlers.
	if err := webhookServer.AddExtensionHandler(server.ExtensionHandler{
		Hook:           runtimehooksv1.AfterControlPlaneUpgrade,
		Name:           "after-controlplane-upgrade",
		HandlerFunc:    h.DoAfterControlPlaneUpgrade,
		TimeoutSeconds: pointer.Int32(5),
		FailurePolicy:  toPtr(runtimehooksv1.FailurePolicyFail),
	}); err != nil {
		setupLog.Error(err, "error adding handler")
		os.Exit(1)
	}

	setupLog.Info("Starting Runtime Extension server")
	if err := webhookServer.Start(ctx); err != nil {
		setupLog.Error(err, "error running webhook server")
		os.Exit(1)
	}
}

const annotationSkipWorkerMinorVersion = "skip-worker-minor-version"

type handler struct {
	Client client.Client
}

func (h *handler) DoAfterControlPlaneUpgrade(ctx context.Context, request *runtimehooksv1.AfterControlPlaneUpgradeRequest, response *runtimehooksv1.AfterControlPlaneUpgradeResponse) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("AfterControlPlaneUpgrade is called")

	cluster := request.Cluster
	fmt.Println("cluster.Spec.Topology.Version: ", cluster.Spec.Topology.Version)
	fmt.Println("request.KubernetesVersion: ", request.KubernetesVersion)

	if version, skipWorkerMinorAnnotation := request.Cluster.Annotations[annotationSkipWorkerMinorVersion]; skipWorkerMinorAnnotation {
		p, err := patch.NewHelper(&cluster, h.Client)
		if err != nil {
			response.Status = runtimehooksv1.ResponseStatusFailure
			response.Message = err.Error()
			fmt.Println("error patching: ", err)
			return
		}
		// remove annotation
		delete(cluster.Annotations, annotationSkipWorkerMinorVersion)
		// set new version
		cluster.Spec.Topology.Version = version
		if err := p.Patch(ctx, &cluster); err != nil {
			response.Status = runtimehooksv1.ResponseStatusFailure
			response.Message = err.Error()
			fmt.Println("error patching: ", err)
			return
		}

		// Return RetryAfterSeconds so topology controller will do a new reconciliation
		response.RetryAfterSeconds = 5
	}
}

func toPtr(f runtimehooksv1.FailurePolicy) *runtimehooksv1.FailurePolicy {
	return &f
}
