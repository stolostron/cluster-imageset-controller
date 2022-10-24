package imageset

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/ghodss/yaml"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"

	"github.com/go-logr/logr"
	hivev1 "github.com/openshift/hive/apis/hive/v1"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	clientgoscheme.AddToScheme(scheme)
	hivev1.AddToScheme(scheme)
	//+kubebuilder:scaffold:scheme
}

func NewSyncImagesetCommand(logger logr.Logger) *cobra.Command {
	o := NewImagesetOptions(logger)

	ctx := context.TODO()

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Start controller to sync cluster imagesets from a Git repository",
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
	Log           logr.Logger
	MetricAddr    string
	ProbeAddr     string
	Interval      int
	GitRepository string
	GitBranch     string
	GitPath       string
	Channel       string
}

// NewWorkloadAgentOptions returns the flags with default value set
func NewImagesetOptions(logger logr.Logger) *ImagesetOptions {
	return &ImagesetOptions{Log: logger}
}

func (o *ImagesetOptions) AddFlags(cmd *cobra.Command) {
	flags := cmd.Flags()
	// This command only supports reading from config
	flags.IntVar(&o.Interval, "sync-interval", 60,
		"Interval in seconds when cluster imagesets are sync with the Git repository.")
	flags.StringVar(&o.GitRepository, "git-repository", "https://github.com/stolostron/acm-hive-openshift-releases.git", "Git repository to sync the cluster imagesets from.")
	flags.StringVar(&o.GitBranch, "git-branch", "release-2.6", "Branch of the Git repository.")
	flags.StringVar(&o.GitPath, "git-path", "clusterImageSets", "Path in the Git repository.")
	flags.StringVar(&o.Channel, "channel", "fast", "Name of channel to sync cluster imagesets from.")
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

	restMapper, err := apiutil.NewDynamicRESTMapper(config, apiutil.WithLazyDiscovery)
	if err != nil {
		return err
	}

	iCtrl := NewImageSetController(mgr.GetClient(), restMapper, o)
	if err := mgr.Add(iCtrl); err != nil {
		return err
	}

	o.Log.Info("starting manager")

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up health check, err: %w", err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up ready check, err: %w", err)
	}

	return mgr.Start(ctrl.SetupSignalHandler())
}

type ImageSetController struct {
	client        client.Client
	restMapper    meta.RESTMapper
	log           logr.Logger
	interval      int
	channel       string
	gitRepository string
	gitBranch     string
	gitPath       string
}

func NewImageSetController(c client.Client, r meta.RESTMapper, o *ImagesetOptions) *ImageSetController {
	return &ImageSetController{
		client:        c,
		restMapper:    r,
		log:           o.Log,
		channel:       o.Channel,
		interval:      o.Interval,
		gitRepository: o.GitRepository,
		gitBranch:     o.GitBranch,
		gitPath:       o.GitPath,
	}
}

func (r *ImageSetController) Start(ctx context.Context) error {
	go wait.Until(func() {
		err := r.syncImageSet()
		if err != nil {
			r.log.Error(err, "error syncing cluster imagesets")
		}
	}, time.Duration(r.interval)*time.Second, ctx.Done())

	return nil
}

func (r *ImageSetController) syncImageSet() error {
	r.log.Info("sync cluster imageset")

	tempDir, err := ioutil.TempDir(os.TempDir(), "cluster-imageset-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	if err := r.cloneGitRepo(tempDir); err != nil {
		return err
	}

	if err := r.applyImageSetsFromClonedGitRepo(tempDir); err != nil {
		return err
	}

	return nil
}

func (r *ImageSetController) cloneGitRepo(destDir string) error {
	r.log.Info(fmt.Sprintf("cloning Git repository:%s, branch:%v to directory:%s", r.gitRepository, r.gitBranch, destDir))

	options := &git.CloneOptions{
		URL:               r.gitRepository,
		SingleBranch:      true,
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
		ReferenceName:     plumbing.NewBranchReferenceName(r.gitBranch),
	}

	_, err := git.PlainClone(destDir, false, options)
	if err != nil {
		r.log.Error(err, "failed to clone the Git repo")
		return err
	}

	return nil
}

func (r *ImageSetController) applyImageSetsFromClonedGitRepo(destDir string) error {
	resourcePath := filepath.Join(destDir, r.gitPath, r.channel)
	r.log.Info(fmt.Sprintf("applying cluster imagesets from path: %v", resourcePath))

	err := filepath.Walk(resourcePath,
		func(path string, info os.FileInfo, err error) error {
			if !info.IsDir() {
				file, err := ioutil.ReadFile(path)
				if err != nil {
					r.log.Error(err, "failed to read cluster imageset file: "+path)
					return err
				}

				if err := r.applyClusterImageSetFile(file); err != nil {
					r.log.Error(err, "failed to apply cluster imageset file:"+path)
					return err
				}
			}

			return nil
		})

	return err
}

func (r *ImageSetController) applyClusterImageSetFile(file []byte) error {
	clusterset := &hivev1.ClusterImageSet{}
	if err := yaml.Unmarshal(file, clusterset); err != nil {
		return err
	}

	oClusterset := &hivev1.ClusterImageSet{}
	err := r.client.Get(context.TODO(), client.ObjectKeyFromObject(clusterset), oClusterset)
	if err != nil {
		if errors.IsNotFound(err) {
			err = r.createClusterImageSet(clusterset)
		} else {
			r.log.Error(err, "failed to apply resource with error:")
		}
	} else {
		r.log.Info(fmt.Sprintf("cluster imageset(%v) already exists, skipping", clusterset.GetName()))
	}

	return err
}

func (r *ImageSetController) createClusterImageSet(imageset *hivev1.ClusterImageSet) error {
	r.log.Info(fmt.Sprintf("create cluster imageset: %v", imageset))

	if err := r.client.Create(context.TODO(), imageset); err != nil {
		return err
	}

	return nil
}

func (r *ImageSetController) updateClusterImageSet(oImageset, imageset *hivev1.ClusterImageSet) error {

	oImageset.Spec = imageset.Spec
	r.log.V(2).Info(fmt.Sprintf("update cluster imageset: %v", oImageset))

	if err := r.client.Update(context.TODO(), oImageset); err != nil {
		return err
	}

	return nil
}
