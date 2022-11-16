package clusterimageset

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	"gopkg.in/src-d/go-git.v4"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"gopkg.in/src-d/go-git.v4/plumbing"
	gitclient "gopkg.in/src-d/go-git.v4/plumbing/transport/client"
	githttp "gopkg.in/src-d/go-git.v4/plumbing/transport/http"
	corev1 "k8s.io/api/core/v1"
)

const (
	// Secret data for Git authentication
	UserID      = "user"
	AccessToken = "accessToken"
	ClientKey   = "clientKey"
	ClientCert  = "clientCert"

	// Git repo configurations (in configmap)
	GitRepoUrl         = "gitRepoUrl"
	GitRepoBranch      = "gitRepoBranch"
	GitRepoPath        = "gitRepoPath"
	Channel            = "channel"
	CaCerts            = "caCerts"
	InsecureSkipVerify = "insecureSkipVerify"

	// Default values
	DefaultGitRepoUrl    = "https://github.com/stolostron/acm-hive-openshift-releases.git"
	DefaultGitRepoBranch = "backplane-2.2"
	DefaultGitRepoPath   = "clusterImageSets"
	DefaultChannel       = "fast"
)

func (r *ClusterImageSetController) getLastCommitID() (string, error) {
	tempDir, err := ioutil.TempDir(os.TempDir(), "cluster-imageset-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempDir)

	repo, err := r.cloneGitRepo(tempDir, true)
	if err != nil {
		return "", err
	}

	ref, err := repo.Head()
	if err != nil {
		r.log.Info(fmt.Sprintf("failed to get git repo head: %v", err.Error()))
		return "", err
	}

	lastCommit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		r.log.Info(fmt.Sprintf("failed to get the last commit ID: %v", err.Error()))
		return "", err
	}

	return lastCommit.ID().String(), nil
}

func (r *ClusterImageSetController) cloneGitRepo(destDir string, noCheckOut bool) (*git.Repository, error) {
	options, err := r.getHTTPOptions()
	if err != nil {
		return nil, err
	}

	options.NoCheckout = noCheckOut

	r.log.Info(fmt.Sprintf("cloning Git repository:%s, branch:%v to directory:%s, no-checkout:%v", options.URL, options.ReferenceName, destDir, noCheckOut))

	repository, err := git.PlainClone(destDir, false, options)
	if err != nil {
		return repository, err
	}

	return repository, nil
}

func (r *ClusterImageSetController) getHTTPOptions() (*git.CloneOptions, error) {
	gitRepoUrl, gitRepoBranch, _, _, caCerts, insecureSkipVerify, err := r.getGitRepoConfig()
	if err != nil {
		return nil, err
	}

	options := &git.CloneOptions{
		URL:               gitRepoUrl,
		SingleBranch:      true,
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
		ReferenceName:     plumbing.NewBranchReferenceName(gitRepoBranch),
	}

	user, accessToken, clientKey, clientCert, err := r.getGitRepoAuthFromSecret()
	if err != nil {
		return nil, err
	}

	if user != "" && accessToken != "" {
		options.Auth = &githttp.BasicAuth{
			Username: user,
			Password: accessToken,
		}
	}

	installProtocol := false
	clientConfig := &tls.Config{MinVersion: tls.VersionTLS12}

	// skip TLS certificate verification for Git servers with custom or self-signed certs
	if insecureSkipVerify {
		r.log.Info("insecureSkipVerify = true, skipping Git server's certificate verification.")

		clientConfig.InsecureSkipVerify = true
		installProtocol = true
	} else if !strings.EqualFold(caCerts, "") {
		r.log.Info("adding Git server's CA certificate to trust certificate pool")

		// Load the host's trusted certs into memory
		certPool, _ := x509.SystemCertPool()
		if certPool == nil {
			certPool = x509.NewCertPool()
		}

		certChain := getCertChain(caCerts)
		if len(certChain.Certificate) == 0 {
			r.log.Info("no certificate found")
		}

		// Add CA certs from the config map to the cert pool
		// It will not add duplicate certs
		for _, cert := range certChain.Certificate {
			x509Cert, err := x509.ParseCertificate(cert)
			if err != nil {
				return options, err
			}
			r.log.V(4).Info("adding certificate -->" + x509Cert.Subject.String())
			certPool.AddCert(x509Cert)
		}

		clientConfig.RootCAs = certPool
		installProtocol = true
	}

	// If client key pair is provided, make mTLS connection
	if len(clientKey) > 0 && len(clientCert) > 0 {
		r.log.Info("client certificate key pair is provided. Making mTLS connection.")

		clientCertificate, err := tls.X509KeyPair(clientCert, clientKey)
		if err != nil {
			r.log.Info(fmt.Sprintf("failed to get key pair: %v", err.Error()))
			return options, err
		}

		// Add the client certificate in the connection
		clientConfig.Certificates = []tls.Certificate{clientCertificate}

		r.log.Info("client certificate key pair added successfully")
	}

	if installProtocol {
		transportConfig := &http.Transport{
			TLSClientConfig: clientConfig, // #nosec G402
		}

		customClient := &http.Client{
			Transport: transportConfig,  // #nosec G402
			Timeout:   15 * time.Second, // 15 second timeout

			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}

		gitclient.InstallProtocol("https", githttp.NewClient(customClient))
	}

	return options, nil
}

func (r *ClusterImageSetController) getGitRepoAuthFromSecret() (string, string, []byte, []byte, error) {
	username := ""
	accessToken := ""
	clientKey := []byte("")
	clientCert := []byte("")

	secret := &corev1.Secret{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: r.secret, Namespace: "open-cluster-management"}, secret)
	if err != nil {
		if errors.IsNotFound(err) {
			return username, accessToken, clientKey, clientCert, nil
		}

		r.log.Info("unable to get secret for cluster image set Git repo")
		return username, accessToken, clientKey, clientCert, err
	}

	err = yaml.Unmarshal(secret.Data[UserID], &username)
	if err != nil {
		r.log.Info("failed to unmarshal username from the secret.")
		return username, accessToken, clientKey, clientCert, err
	}

	err = yaml.Unmarshal(secret.Data[AccessToken], &accessToken)
	if err != nil {
		r.log.Info("failed to unmarshal accessToken from the secret.")
		return username, accessToken, clientKey, clientCert, err
	}

	clientKey = bytes.TrimSpace(secret.Data[ClientKey])
	clientCert = bytes.TrimSpace(secret.Data[ClientCert])

	if (len(clientKey) == 0 && len(clientCert) > 0) || (len(clientKey) > 0 && len(clientCert) == 0) {
		r.log.Info("for mTLS connection to Git, both clientKey (private key) and clientCert (certificate) are required in the channel secret")
		return username, accessToken, clientKey, clientCert,
			fmt.Errorf("for mTLS connection to Git, both clientKey (private key) and clientCert (certificate) are required in the channel secret")
	}
	if err != nil {
		return username, accessToken, clientKey, clientCert, err
	}

	return username, accessToken, clientKey, clientCert, nil
}

func getCertChain(certs string) tls.Certificate {
	var certChain tls.Certificate
	certPEMBlock := []byte(certs)
	var certDERBlock *pem.Block

	for {
		certDERBlock, certPEMBlock = pem.Decode(certPEMBlock)
		if certDERBlock == nil {
			break
		}

		if certDERBlock.Type == "CERTIFICATE" {
			certChain.Certificate = append(certChain.Certificate, certDERBlock.Bytes)
		}
	}

	return certChain
}

func (r *ClusterImageSetController) getGitRepoConfig() (string, string, string, string, string, bool, error) {
	configMap := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: r.configMap, Namespace: "open-cluster-management"}, configMap)
	if err != nil {
		r.log.Info(fmt.Sprintf("unable to get config map %v, use default values.", r.configMap))
		return DefaultGitRepoUrl, DefaultGitRepoBranch, DefaultGitRepoPath, DefaultChannel, "", false, nil
	}

	gitRepoUrl := configMap.Data[GitRepoUrl]
	if gitRepoUrl == "" {
		gitRepoUrl = DefaultGitRepoUrl
	}

	gitRepoBranch := configMap.Data[GitRepoBranch]
	if gitRepoBranch == "" {
		gitRepoBranch = DefaultGitRepoBranch
	}

	gitRepoPath := configMap.Data[GitRepoPath]
	if gitRepoPath == "" {
		gitRepoPath = DefaultGitRepoPath
	}

	channel := configMap.Data[Channel]
	if channel == "" {
		channel = DefaultChannel
	}

	caCert := configMap.Data[CaCerts]

	bSkipCertVerify := false
	skipCertVerify := configMap.Data[InsecureSkipVerify]
	if skipCertVerify != "" {
		bSkipCertVerify, err = strconv.ParseBool(skipCertVerify)
		if err != nil {
			r.log.Info(fmt.Sprintf("invalid bool value for insecureSkipVerify: %v", err.Error()))
		}
	}

	return gitRepoUrl, gitRepoBranch, gitRepoPath, channel, caCert, bSkipCertVerify, nil
}
