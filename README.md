# cluster-imageset-controller

The repository provides a controller that queries the Git repository https://github.com/stolostron/acm-hive-openshift-releases, at set intervals, for new clusterImageSets. The new clusterImageSets are then applied by the controller to the hub cluster of the Red Hat Advanced Cluster Management for Kubernetes (ACM)/Red Hat Multicluster Engine (MCE) environment. This makes available the latest OpenShift images in ACM/MCE for OpenShift deployments.

The acm-hive-openshift-releases Git repository has a cron job that runs every 3 hrs. This cron job queries the install image repository https://quay.io/repository/openshift-release-dev/ocp-release?tab=tags for the latest OpenShift images. A new clusterImageSet YAML is added to the acm-hive-openshift-releases Git repository when a new OpenShift image is discovered. The contents of the acm-hive-openshift-releases Git repository is organized using a directory structure to separate images based on the OCP version and the release channel (`fast/stable/candidate`). The branch of the Git repository is used to define the set of cluster imagesets that are applicable to a particular MCE/ACM release.

By default, this controller synchronizes the clusterImageSets from the Git repository https://github.com/stolostron/acm-hive-openshift-releases, branch `release-2.6`, and using the `fast` channel. These default values could be overriden through properties in the configMap `cluster-image-set-git-repo` in the `open-cluster-management` namespace.

This is a sample of the configMap YAML.

```YAML
apiVersion: v1
kind: ConfigMap
metadata:
  name: cluster-image-set-git-repo
  namespace: open-cluster-management
data:
  gitRepoUrl: https://localhost:10880/p/testrepo2.git
  gitRepoBranch: release-2.6
  gitRepoPath: clusterImageSets
  channel: fast
  insecureSkipVerify: "false"
  caCerts: |
    -----BEGIN CERTIFICATE-----
    MIIFTDCCAzQCCQDUHR2zBw+sDDANBgkqhkiG9w0BAQsFADBoMQswCQYDVQQGEwJV
    vB4YZTsCx9WLCBLqrUQLmYz8OlB2FNAudUwn38C7hyqp0KSU6eKw4cJcljqpxEP2
    AXDDYhRiaIJMdgKh37ewhw==
    -----END CERTIFICATE-----
```

If the Git repository requires authentication, the authentication information could be provided through properties in the secret `cluster-image-set-git-repo` in the `open-cluster-management` namespace.

Here is a sample of a secret that uses basic authentication:

```YAML
apiVersion: v1
kind: Secret
metadata:
  name: cluster-image-set-repo
  namespace: open-cluster-management
type: Opaque
data:
  user: cGhpbGlw
  accessToken: cGFzc3cwcmQ=
```

For authentication using HTTPS client certificates, a secret similar to this could be used:

```YAML
apiVersion: v1
kind: Secret
metadata:
  name: cluster-image-set-repo
  namespace: open-cluster-management
type: Opaque
data:
  clientKey: key1
  clientCert: cert1
```

The controller provides options to override the names of the configMap and secret that contains the configuration information used to access Git repository. For the full list of available options, run:
```
./bin/clusterimageset sync --help
```
