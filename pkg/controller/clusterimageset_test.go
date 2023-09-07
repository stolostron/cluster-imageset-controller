package clusterimageset

import (
	"context"
	"testing"
	"time"

	"github.com/ghodss/yaml"
	"github.com/go-logr/zapr"
	"github.com/onsi/gomega"
	hivev1 "github.com/openshift/hive/apis/hive/v1"
	"github.com/stolostron/cluster-imageset-controller/test/integration/util"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func TestSyncImageSet(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	zapLog, _ := zap.NewDevelopment()
	options := &ImagesetOptions{
		Log:       zapr.NewLogger(zapLog),
		Interval:  60,
		ConfigMap: "cluster-image-set-git-repo",
	}

	c := initClient()

	iCtrl := NewClusterImageSetController(c, options)

	configMap := getConfigMap("badurl", "release-2.6", "clusterImageSets", "fast")
	err := c.Create(context.TODO(), configMap)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	err = iCtrl.syncClusterImageSet(true)
	g.Expect(err).To(gomega.HaveOccurred())

	// Create dummy cluster imageset that will NOT be deleted by cleanup routine
	cis := &hivev1.ClusterImageSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dummy-img4.11.0-x86-64-appsub",
		},
		Spec: hivev1.ClusterImageSetSpec{
			ReleaseImage: "quay.io/openshift-release-dev/ocp-release:4.11.0-x86_64-0",
		},
	}
	err = c.Create(context.TODO(), cis)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	// Create dummy cluster imageset with channel label that will be deleted by cleanup routine
	cis2 := &hivev1.ClusterImageSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dummy2-img4.11.0-x86-64-appsub",
		},
		Spec: hivev1.ClusterImageSetSpec{
			ReleaseImage: "quay.io/openshift-release-dev/ocp-release:4.11.0-x86_64-0",
		},
	}
	cis2.SetLabels(map[string]string{util.ChannelLabel: "fast"})
	err = c.Create(context.TODO(), cis2)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	// Create configmap with valid url
	err = c.Delete(context.TODO(), configMap)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	iCtrl = NewClusterImageSetController(c, options)
	iCtrl.lastCommitID = "fakeCommit"
	err = iCtrl.syncClusterImageSet(true)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	imagesetList := &hivev1.ClusterImageSetList{}
	err = c.List(context.TODO(), imagesetList, &client.ListOptions{})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(imagesetList.Items).Should(gomega.ContainElements())

	// Dummy imageset should NOT be deleted
	err = c.Get(context.TODO(), client.ObjectKeyFromObject(cis), cis)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	// Dummy imageset should be deleted
	err = c.Get(context.TODO(), client.ObjectKeyFromObject(cis2), cis2)
	g.Expect(err).To(gomega.HaveOccurred())
	g.Expect(errors.IsNotFound(err)).To(gomega.BeTrue())
}

func TestSetupImageSetController(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	zapLog, _ := zap.NewDevelopment()
	options := &ImagesetOptions{
		Log:        zapr.NewLogger(zapLog),
		Interval:   60,
		MetricAddr: ":8387",
		ConfigMap:  "cluster-image-set-git-repo",
	}

	ctx := context.TODO()
	controllerFunc := func(manager manager.Manager) {
		err := options.runControllerManager(ctx, manager)
		g.Expect(err).NotTo(gomega.HaveOccurred())
	}

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
			Name:   "img4.11.0-x86-64-appsub",
			Labels: map[string]string{"visible": "true"},
		},
		Spec: hivev1.ClusterImageSetSpec{
			ReleaseImage: "quay.io/openshift-release-dev/ocp-release:4.11.0-x86_64-0",
		},
	}

	// ReleaseImage changed
	cis2 := &hivev1.ClusterImageSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "img4.11.0-x86-64-appsub",
			Labels: map[string]string{"visible": "true"},
		},
		Spec: hivev1.ClusterImageSetSpec{
			ReleaseImage: "quay.io/openshift-release-dev/ocp-release:4.11.0-x86_64",
		},
	}

	// Visible label changed
	cis3 := &hivev1.ClusterImageSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "img4.11.0-x86-64-appsub",
			Labels: map[string]string{"visible": "false"},
		},
		Spec: hivev1.ClusterImageSetSpec{
			ReleaseImage: "quay.io/openshift-release-dev/ocp-release:4.11.0-x86_64",
		},
	}

	bCis, err := yaml.Marshal(cis)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	_, err = iCtrl.applyClusterImageSetFile(bCis)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	createdCis := &hivev1.ClusterImageSet{}
	err = iCtrl.client.Get(context.TODO(), client.ObjectKeyFromObject(cis), createdCis)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(createdCis.Spec.ReleaseImage).To(gomega.Equal(cis.Spec.ReleaseImage))

	// apply should update cluster image set since release image changed
	bCis2, err := yaml.Marshal(cis2)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	_, err = iCtrl.applyClusterImageSetFile(bCis2)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	err = iCtrl.client.Get(context.TODO(), client.ObjectKeyFromObject(cis2), createdCis)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(createdCis.Spec.ReleaseImage).To(gomega.Equal(cis2.Spec.ReleaseImage))

	// apply should update cluster image set since visible label changed
	bCis3, err := yaml.Marshal(cis3)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	_, err = iCtrl.applyClusterImageSetFile(bCis3)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	err = iCtrl.client.Get(context.TODO(), client.ObjectKeyFromObject(cis3), createdCis)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(createdCis.GetLabels()["visible"]).To(gomega.Equal(cis3.GetLabels()["visible"]))

	// unmarshal error
	badCis := []byte("bad$:xys")
	_, err = iCtrl.applyClusterImageSetFile(badCis)
	g.Expect(err).To(gomega.HaveOccurred())
}

func TestSyncCommand(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	zapLog, _ := zap.NewDevelopment()
	syncCmd := NewSyncImagesetCommand(zapr.NewLogger(zapLog))
	g.Expect("sync").To(gomega.Equal(syncCmd.Use))
}

func getImageSetController() (*ClusterImageSetController, error) {
	zapLog, _ := zap.NewDevelopment()
	options := &ImagesetOptions{
		Log:       zapr.NewLogger(zapLog),
		Interval:  60,
		ConfigMap: "cluster-image-set-git-repo",
	}

	client := initClient()
	return NewClusterImageSetController(client, options), nil
}

func getConfigMap(gitRepoUrl, gitRepoBranch, gitRepoPath, channel string) *corev1.ConfigMap {
	data := map[string]string{
		"gitRepoUrl":    gitRepoUrl,
		"gitRepoBranch": gitRepoBranch,
		"gitRepoPath":   gitRepoPath,
		"channel":       channel,
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-image-set-git-repo",
			Namespace: "multicluster-engine",
		},
		Data: data,
	}
}

func getDefaultConfigMap() *corev1.ConfigMap {
	return getConfigMap("https://github.com/stolostron/acm-hive-openshift-releases.git", "release-2.6", "clusterImageSets", "fast")
}

func getSecret(name string, user, accessToken, key, cert []byte) *corev1.Secret {
	data := map[string][]byte{
		UserID:      user,
		AccessToken: accessToken,
		ClientKey:   key,
		ClientCert:  cert,
	}

	if name == "" {
		name = "cluster-image-set-git-repo"
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "multicluster-engine",
		},
		Data: data,
	}
}

func initClient() client.Client {
	scheme := runtime.NewScheme()

	metav1.AddMetaToScheme(scheme)
	hivev1.AddToScheme(scheme)
	corev1.AddToScheme(scheme)

	ncb := fake.NewClientBuilder()
	ncb.WithScheme(scheme)
	return ncb.Build()

}
