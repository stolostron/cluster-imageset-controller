package imageset

import (
	"context"
	"testing"
	"time"

	"github.com/ghodss/yaml"
	"github.com/go-logr/zapr"
	"github.com/onsi/gomega"
	hivev1 "github.com/openshift/hive/apis/hive/v1"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func TestSyncImageSet(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	restMapper, err := apiutil.NewDynamicRESTMapper(cfg, apiutil.WithLazyDiscovery)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	zapLog, _ := zap.NewDevelopment()
	options := &ImagesetOptions{
		Log:           zapr.NewLogger(zapLog),
		Interval:      60,
		GitRepository: "https://github.com/stolostron/acm-hive-openshift-releases.git",
		GitBranch:     "release-2.6",
		GitPath:       "clusterImageSets",
		Channel:       "fast",
	}

	c := initClient()

	options.GitRepository = "badurl"
	iCtrl := NewImageSetController(c, restMapper, options)
	err = iCtrl.syncImageSet()
	g.Expect(err).To(gomega.HaveOccurred())

	options.GitRepository = "https://github.com/stolostron/acm-hive-openshift-releases.git"
	iCtrl = NewImageSetController(c, restMapper, options)
	err = iCtrl.syncImageSet()
	g.Expect(err).NotTo(gomega.HaveOccurred())

	imagesetList := &hivev1.ClusterImageSetList{}
	err = c.List(context.TODO(), imagesetList, &client.ListOptions{})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(imagesetList.Items).Should(gomega.ContainElements())
}

func TestSetupImageSetController(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	zapLog, _ := zap.NewDevelopment()
	options := &ImagesetOptions{
		Log:           zapr.NewLogger(zapLog),
		Interval:      60,
		GitRepository: "https://github.com/stolostron/acm-hive-openshift-releases.git",
		GitBranch:     "release-2.6",
		GitPath:       "clusterImageSets",
		Channel:       "fast",
	}

	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	ctx := context.TODO()
	controllerFunc := func(manager manager.Manager) {
		err = options.runControllerManager(ctx, manager)
		g.Expect(err).NotTo(gomega.HaveOccurred())
	}
	go controllerFunc(mgr)
	time.Sleep(1 * time.Second)
	ctx.Done()

	go controllerFunc(nil)
	time.Sleep(1 * time.Second)
	ctx.Done()
}

func TestApplyClusterImageSet(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	iCtrl, err := getImageSetController()
	g.Expect(err).NotTo(gomega.HaveOccurred())

	cis := &hivev1.ClusterImageSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "img4.11.0-x86-64-appsub",
		},
		Spec: hivev1.ClusterImageSetSpec{
			ReleaseImage: "quay.io/openshift-release-dev/ocp-release:4.11.0-x86_64-0",
		},
	}

	cis2 := &hivev1.ClusterImageSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "img4.11.0-x86-64-appsub",
		},
		Spec: hivev1.ClusterImageSetSpec{
			ReleaseImage: "quay.io/openshift-release-dev/ocp-release:4.11.0-x86_64",
		},
	}

	bCis, err := yaml.Marshal(cis)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	err = iCtrl.applyClusterImageSetFile(bCis)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	createdCis := &hivev1.ClusterImageSet{}
	err = iCtrl.client.Get(context.TODO(), client.ObjectKeyFromObject(cis), createdCis)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(createdCis.Spec.ReleaseImage).To(gomega.Equal(cis.Spec.ReleaseImage))

	// apply should be skipped since clusterset already exists
	bCis2, err := yaml.Marshal(cis2)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	err = iCtrl.applyClusterImageSetFile(bCis2)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	err = iCtrl.client.Get(context.TODO(), client.ObjectKeyFromObject(cis2), createdCis)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(createdCis.Spec.ReleaseImage).To(gomega.Equal(cis.Spec.ReleaseImage))

	err = iCtrl.client.Get(context.TODO(), client.ObjectKeyFromObject(cis), cis)
	err = iCtrl.updateClusterImageSet(cis, cis2)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	err = iCtrl.client.Get(context.TODO(), client.ObjectKeyFromObject(cis), createdCis)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(createdCis.Spec.ReleaseImage).To(gomega.Equal(cis2.Spec.ReleaseImage))

	// unmarshal error
	badCis := []byte("bad$:xys")
	err = iCtrl.applyClusterImageSetFile(badCis)
	g.Expect(err).To(gomega.HaveOccurred())
}

func TestSyncCommand(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	zapLog, _ := zap.NewDevelopment()
	syncCmd := NewSyncImagesetCommand(zapr.NewLogger(zapLog))
	g.Expect("sync").To(gomega.Equal(syncCmd.Use))
}

func getImageSetController() (*ImageSetController, error) {
	restMapper, err := apiutil.NewDynamicRESTMapper(cfg, apiutil.WithLazyDiscovery)
	if err != nil {
		return nil, err
	}

	zapLog, _ := zap.NewDevelopment()
	options := &ImagesetOptions{
		Log:           zapr.NewLogger(zapLog),
		Interval:      60,
		GitRepository: "https://github.com/stolostron/acm-hive-openshift-releases.git",
		GitBranch:     "release-2.6",
		GitPath:       "clusterImageSets",
		Channel:       "fast",
	}

	client := initClient()
	return NewImageSetController(client, restMapper, options), nil
}

func initClient() client.Client {
	scheme := runtime.NewScheme()

	metav1.AddMetaToScheme(scheme)
	hivev1.AddToScheme(scheme)

	ncb := fake.NewClientBuilder()
	ncb.WithScheme(scheme)
	return ncb.Build()

}
