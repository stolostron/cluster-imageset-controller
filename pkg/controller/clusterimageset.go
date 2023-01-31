package clusterimageset

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/ghodss/yaml"
	"github.com/stolostron/cluster-imageset-controller/test/integration/util"
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/go-logr/logr"
	hivev1 "github.com/openshift/hive/apis/hive/v1"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(hivev1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func NewSyncImagesetCommand(logger logr.Logger) *cobra.Command {
	o := NewImagesetOptions(logger)

	ctx := context.TODO()

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Start controller to sync the clusterImageSets from a Git repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			return o.runControllerManager(ctx, nil)
		},
	}

	o.AddFlags(cmd)

	cmd.FParseErrWhitelist.UnknownFlags = true

	return cmd
}

// AgentOptions defines the flags for workload agent
type ImagesetOptions struct {
	Log        logr.Logger
	MetricAddr string
	ProbeAddr  string
	Interval   int
	ConfigMap  string
	Secret     string
}

// NewWorkloadAgentOptions returns the flags with default value set
func NewImagesetOptions(logger logr.Logger) *ImagesetOptions {
	return &ImagesetOptions{Log: logger}
}

func (o *ImagesetOptions) AddFlags(cmd *cobra.Command) {
	flags := cmd.Flags()
	// This command only supports reading from config
	flags.IntVar(&o.Interval, "sync-interval", 60,
		"Interval in seconds when clusterImageSets are synced with the Git repository.")
	flags.StringVar(&o.ConfigMap, "git-configmap", "cluster-image-set-git-repo", "Configuration info to access the clusterImageSet Git repository.")
	flags.StringVar(&o.Secret, "git-secret", "cluster-image-set-git-repo", "Authentication info to access the clusterImageSet Git repository.")
	flags.StringVar(&o.MetricAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flags.StringVar(&o.ProbeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")

}

func (o *ImagesetOptions) runControllerManager(ctx context.Context, mgr manager.Manager) error {
	flag.Parse()

	config := ctrl.GetConfigOrDie()
	if mgr == nil {
		var err error
		mgr, err = ctrl.NewManager(config, ctrl.Options{
			Scheme:                 scheme,
			MetricsBindAddress:     o.MetricAddr,
			Port:                   9443,
			HealthProbeBindAddress: o.ProbeAddr,
			LeaderElection:         false,
		})

		if err != nil {
			o.Log.Error(err, "unable to start manager")
			return fmt.Errorf("unable to create manager, err: %w", err)
		}
	}

	client, err := client.New(mgr.GetConfig(), client.Options{Scheme: scheme})
	if err != nil {
		o.Log.Error(err, "unable to create kube client.")
		os.Exit(1)
	}
	iCtrl := NewClusterImageSetController(client, o)

	iCtrl.Start()

	o.Log.Info("starting manager")

	return mgr.Start(ctrl.SetupSignalHandler())
}

type ClusterImageSetController struct {
	client       client.Client
	log          logr.Logger
	stopch       chan struct{}
	interval     int
	configMap    string
	secret       string
	lastCommitID string
}

func NewClusterImageSetController(c client.Client, o *ImagesetOptions) *ClusterImageSetController {
	return &ClusterImageSetController{
		client:    c,
		log:       o.Log,
		interval:  o.Interval,
		configMap: o.ConfigMap,
		secret:    o.Secret,
	}
}

func (r *ClusterImageSetController) Start() {
	// do nothing if already started
	if r.stopch != nil {
		return
	}

	r.stopch = make(chan struct{})

	cleanup := true

	go wait.Until(func() {
		err := r.syncClusterImageSet(cleanup)
		if err != nil {
			fmt.Printf("error syncing clusterImageSets: %v", err.Error())
		}

		cleanup = false // Perform cleanup on first run only
	}, time.Duration(r.interval)*time.Second, r.stopch)
}

func (r *ClusterImageSetController) Stop() {
	close(r.stopch)

	r.stopch = nil
}

func (r *ClusterImageSetController) syncClusterImageSet(cleanup bool) error {
	r.log.Info("start syncClusterImageSet")
	defer r.log.Info("done syncClusterImageSet")

	// Check if the last commit ID is different since the previous sync
	if r.lastCommitID != "" {
		lastCommitID, err := r.getLastCommitID()
		if err != nil {
			return err
		}

		if r.lastCommitID == lastCommitID {
			r.log.Info(fmt.Sprintf("previous commit %v is already the most recent, skip sync", lastCommitID))
			return nil
		}
	}

	tempDir, err := ioutil.TempDir(os.TempDir(), "cluster-imageset-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	repo, err := r.cloneGitRepo(tempDir, false)
	if err != nil {
		return err
	}

	imagesetList, err := r.applyImageSetsFromClonedGitRepo(tempDir)
	if err != nil {
		return err
	}

	if cleanup {
		err = r.cleanupClusterImages(imagesetList)
		if err != nil {
			return err
		}
	}

	// Update lastCommitID
	ref, err := repo.Head()
	if err != nil {
		return err
	}
	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return err
	}
	r.lastCommitID = commit.ID().String()

	return nil
}

func (r *ClusterImageSetController) applyImageSetsFromClonedGitRepo(destDir string) ([]string, error) {
	imageSetList := []string{}

	_, _, gitRepoPath, channel, _, _, err := r.getGitRepoConfig()
	if err != nil {
		return nil, err
	}

	resourcePath := filepath.Join(destDir, gitRepoPath, channel)
	r.log.Info(fmt.Sprintf("applying clusterImageSets from path: %v", resourcePath))

	err = filepath.Walk(resourcePath,
		func(path string, info os.FileInfo, err error) error {
			if !info.IsDir() {
				file, err := ioutil.ReadFile(filepath.Clean(path))
				if err != nil {
					r.log.Info("failed to read clusterImageSet file: " + path)
					return err
				}

				imageset, err := r.applyClusterImageSetFile(file)
				if err != nil {
					r.log.Info("failed to apply clusterImageSet file:" + path)
					return err
				}
				imageSetList = append(imageSetList, imageset.GetName())
			}

			return nil
		})

	return imageSetList, err
}

func (r *ClusterImageSetController) applyClusterImageSetFile(file []byte) (*hivev1.ClusterImageSet, error) {
	imageset := &hivev1.ClusterImageSet{}
	if err := yaml.Unmarshal(file, imageset); err != nil {
		return nil, err
	}

	oImageset := &hivev1.ClusterImageSet{}
	err := r.client.Get(context.TODO(), client.ObjectKeyFromObject(imageset), oImageset)
	if err != nil {
		if errors.IsNotFound(err) {
			err = r.createClusterImageSet(imageset)
		} else {
			r.log.Info("failed to create clusterImageSet")
		}
	} else {
		// Check if visible label and release image values changed
		if oImageset.GetLabels()["visible"] != imageset.GetLabels()["visible"] ||
			oImageset.Spec.ReleaseImage != imageset.Spec.ReleaseImage {

			err = r.updateClusterImageSet(oImageset, imageset)
		} else {
			r.log.V(2).Info(fmt.Sprintf("clusterImageSet(%v) already exists, skipping", imageset.GetName()))
			imageset = oImageset
		}
	}

	return imageset, err
}

func (r *ClusterImageSetController) createClusterImageSet(imageset *hivev1.ClusterImageSet) error {
	r.log.Info(fmt.Sprintf("create clusterImageSet: %v", imageset))

	if err := r.client.Create(context.TODO(), imageset); err != nil {
		return err
	}

	return nil
}

func (r *ClusterImageSetController) updateClusterImageSet(oImageset, imageset *hivev1.ClusterImageSet) error {
	oImageset.Spec = imageset.Spec
	oImageset.Labels = imageset.Labels
	r.log.Info(fmt.Sprintf("update clusterImageSet: %v", oImageset))

	if err := r.client.Update(context.TODO(), oImageset); err != nil {
		return err
	}

	return nil
}

func (r *ClusterImageSetController) cleanupClusterImages(currentImageSetList []string) error {
	r.log.Info("cleanup old clusterImageSets")

	imageSets := &hivev1.ClusterImageSetList{}
	err := r.client.List(context.TODO(), imageSets, &client.ListOptions{})
	if err != nil {
		return err
	}

	if len(imageSets.Items) > 0 {
		sort.Strings(currentImageSetList)

		for _, imageSet := range imageSets.Items {
			// Ignore customer's cluster imagesets (without channel label)
			channel, ok := imageSet.GetLabels()[util.ChannelLabel]
			if !ok || channel == "" {
				continue
			}

			i := sort.SearchStrings(currentImageSetList, imageSet.GetName())
			if i >= len(currentImageSetList) || currentImageSetList[i] != imageSet.GetName() {
				r.log.Info(fmt.Sprintf("deleting clusterImageSet: %v", imageSet.GetName()))

				delImageSet := imageSet.DeepCopy()
				if err := r.client.Delete(context.TODO(), delImageSet); err != nil {
					r.log.Info(fmt.Sprintf("failed to delete clusterImageSet: %v", imageSet.GetName()))
					return err
				}
			}
		}
	}

	return nil
}
