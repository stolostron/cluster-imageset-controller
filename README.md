<# cluster-imageset-controller

The repository provides a controller that queries the Git repository https://github.com/stolostron/acm-hive-openshift-releases, at set intervals, for new clusterImageSets. The new clusterImageSets are then applied by the controller to the hub cluster of the Red Hat Advanced Cluster Management for Kubernetes (ACM)/Red Hat Multicluster Engine (MCE) environment. This makes available the latest OpenShift images in ACM/MCE for OpenShift deployments.

The acm-hive-openshift-releases Git repository has a cron job that runs every 3 hrs. This cron job queries the install image repository https://quay.io/repository/openshift-release-dev/ocp-release?tab=tags for the latest OpenShift images. A new clusterImageSet YAML is added to the acm-hive-openshift-releases Git repository when a new OpenShift image is discovered. The contents of the acm-hive-openshift-releases Git repository is organized using a directory structure to separate images based on the OCP version and the release channel (`fast/stable/candidate`). The branch of the Git repository is used to define the set of cluster imagesets that are applicable to a particular MCE/ACM release.

By default, this controller synchronizes the clusterImageSets from the Git repository https://github.com/stolostron/acm-hive-openshift-releases, branch `release-2.6`, and using the `fast` channel. The controller provides options to override these default values, as well as the time interval for the synchronization. For the full list of available options, run:

```
./bin/clusterimageset sync --help
```
