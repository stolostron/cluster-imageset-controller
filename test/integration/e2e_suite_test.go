package e2e_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/go-logr/zapr"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	hivev1 "github.com/openshift/hive/apis/hive/v1"
	imagesetcontroller "github.com/stolostron/cluster-imageset-controller/pkg/controller"
)

const (
	eventuallyTimeout  = 60 // seconds
	eventuallyInterval = 1  // seconds
)

func TestIntegration(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Integration Suite")
}

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(hivev1.AddToScheme(scheme))
}

var (
	testEnv       *envtest.Environment
	restConfig    *rest.Config
	ctx           context.Context
	cancel        context.CancelFunc
	mgr           ctrl.Manager
	dynamicClient dynamic.Interface
)

var _ = ginkgo.BeforeSuite(func() {
	ginkgo.By("bootstrapping test environment")

	var err error

	// install CRDs and start a local kube-apiserver
	testEnv = &envtest.Environment{
		ErrorIfCRDPathMissing: true,
		CRDDirectoryPaths: []string{
			filepath.Join(".", "..", "..", "hack", "test"),
		},
	}
	cfg, err := testEnv.Start()
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	gomega.Expect(cfg).ToNot(gomega.BeNil())
	restConfig = cfg

	ctx, cancel = context.WithCancel(context.Background())
	mgr, err = ctrl.NewManager(restConfig, ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     ":8090",
		Port:                   9443,
		HealthProbeBindAddress: ":8091",
		LeaderElection:         false,
		LeaderElectionID:       "dfe33d85.open-cluster-management.io",
	})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	dynamicClient, err = dynamic.NewForConfig(restConfig)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	zapLog, _ := zap.NewDevelopment()
	options := &imagesetcontroller.ImagesetOptions{
		Log:           zapr.NewLogger(zapLog),
		Interval:      60,
		GitRepository: "https://github.com/stolostron/acm-hive-openshift-releases.git",
		GitBranch:     "release-2.6",
		GitPath:       "clusterImageSets",
		Channel:       "fast",
	}
	restMapper, err := apiutil.NewDynamicRESTMapper(restConfig, apiutil.WithLazyDiscovery)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	iCtrl := imagesetcontroller.NewClusterImageSetController(mgr.GetClient(), restMapper, options)
	err = mgr.Add(iCtrl)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	go startCtrlManager(mgr)
})

var _ = ginkgo.AfterSuite(func() {
	if cancel != nil {
		cancel()
	}
})

func startCtrlManager(mgr ctrl.Manager) {
	err := mgr.Start(ctrl.SetupSignalHandler())
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
}
