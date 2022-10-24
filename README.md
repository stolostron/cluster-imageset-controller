<# cluster-imageset-controller

The repository provides a controller that queries the Git repository https://github.com/stolostron/acm-hive-openshift-releases, at set intervals, for new clusterImageSets. The new clusterImageSets are then applied by the controller to the hub cluster of the Red Hat Advanced Cluster Management for Kubernetes (ACM)/Red Hat Multicluster Engine (MCE)/Open Cluster Management (OCM) environment. This makes available the latest OpenShift images in ACM/OCM/MCE for OpenShift deployments.

The acm-hive-openshift-releases Git repository has a cron job that runs every 60 seconds. This cron job queries the install image repository https://quay.io/repository/openshift-release-dev/ocp-release?tab=tags for the latest OpenShift images. A new clusterImageSet YAML is added to the acm-hive-openshift-releases Git repository when a new OpenShift image is discovered. The contents of the acm-hive-openshift-releases Git repository is organized using a directory structure to separate images based on the OCP version and the release channel (`fast/stable/candidate`).

By default, this controller synchronizes the clusterImageSets from the Git repository https://github.com/stolostron/acm-hive-openshift-releases, branch `release-2.6`, and using the `fast` channel. The controller provides options to override these default values, as well as the time interval for the synchronization. For the full list of available options, run:

```
./bin/imageset sync --help
```
