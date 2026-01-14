# 6.6
##Important
* px-backup deployments are now using a random admin password shown on `px-deploy connect` login

##Fixes
* px-deploy vsphere-init now using proper network settings
* clusterpair (async-dr / migration) now works on vsphere deployments (still need aws credentials & DR_BUCKET)

##Improvements
* PX Ent Version 3.5.1
* PX-Backup 2.10.1
* OCP 4.19.21
* update pxbbq version
* include px cli tool
* clouddrives will now have a deployment uuid label by default, used for more robust destroy function
* implement aws-load-balancer-controller on aws for proper service type LB creation
* new training-backup template
* rewrite snoop without stap
* expose wetty with load balancer and DNS
* stop/start cluster by number

# 6.5.1
 * integrate missing PR

# 6.5

##Fixes
 * Fix multiple AWS key warning display

##Improvements
 * PX Ent Version 3.4
 * OCP 4.19
 * PX-Backup 2.9.1
 * ocp-kubevirt now using new RWX block
 * update base linux image to rocky 9.5
 * Add cronjob for monitoring AWS costs
 * Simplify pxctl script
 * Simplify Grafana deployment

# 6.4.5

## Fixes
* add tf-managed AWS GuardDuty endpoint
* enforce AWS imdsv2
* re-work training
* fix minor issues

# 6.4.4

## Improvements
 * updates on documentation

# 6.4.3

## Improvements
 * re-work creation of clusterpair
 * you can now run 'storkctl perform failover...'
 * bump px version
 * updates on documentation

# 6.4.2

## Fixes
 * workaround for px-backup mongodb deployment issue

# 6.4.1

## Improvements
 * bump px-backup to 2.8.4
 * bump OCP to 4.18.8
 * bump k8s to 1.31.8
 * bump EKS/GKE/AKS to 1.31
 
## Fixes
 * create StorageProfile on OCP-V 
 * metro-dr now has fixed etcd version (:latest tag was broken on quay.io)
 * improve px install reliability
 * fix grafana on OCP

# 6.4

## Improvements
 * bump px-backup to 2.8.3
 * bump px version to 3.2.2
 * update pxbbq template
 * change px-deploy github repo

## Fixes
 * fix loss of network connectivity when restoring kubevirt vm
 * support metro-dr template running more than 2 clusters (still only 2 in metro dr)

# 6.3.1

## Fixes
 * Fix Grafana namespace

# 6.3

## Improvements
 * Before each create, assets, scripts and templates are synced from running container
 * ocp-kubevirt now deploys Ubuntu VM instead of FreeBSD
 * Always check latest version before deploying

## Fixes
 * Optimisations in ocp-kubevirt
 * Add NFS ports for OCP

## Removal
 * Outdated assets

# 6.2.2.1

## Fixes
 * Fix petclinic yaml typo

# 6.2.2

## Improvements
 * Bump EKS to 1.31
 * Bump Portworx to 3.2.1
 * Bump GKE to 1.30
 * Bump AKS to 1.30
 * Enable OCP console plugin
 * Add support for OCP 4.16
 * Add Rancher support (Beta)
 * Reduce deployment time for PX-Backup
 * Automatically capture logs for troubleshooting
 * Bump PX-Central to 2.7.3

## Fixes
 * Fix Grafana on Kubernetes and OCP
 * Fix issue destroying deployments with large numbers of disks

# 6.2.1

## Improvements
 * Add Ceph template

## Fixes
 * Fix FreeBSD pymongo install

# 6.2

## Improvements
 * Bump Portworx to 3.1.2
 * Bump PX-Central to 2.7.1
 * Add native Apple Silicon support
 * Improve release workflow
 * Add lock flag
 * Add parameter for cluster-specific number of nodes
 * Display age of active AWS keys after each provision

## Fixes
 * Use default credentials for all Terraform actions (to allow key rotation)

# 6.1

## Improvements
 * Bump Kubernetes to 1.28.9
 * Bump Portworx to 3.1.1
 * Bump PX-Central to 2.7.0
 * Bump various versions

## Fixes
 * Prevent crash when nodegroup deletion fails

# 6.0.2

## Improvements
 * Add healthchecks for pxbbq
 * Improve parameter validation

## Fixes
 * Metro now works on OCP
 * Fix strange curl issue on openshift-install
 * Fix metro race condition

# 6.0.1

## Improvements
 * OCP4 documentation

## Fixes
 * Switch to new Kubernetes yum repo
 * Clean up vSphere clouddrives

# 6.0

## Improvements
 * Completed migration to Terraform
 * Introduce kubevirt-ocp template
 * Bump Kubernetes to 1.26.4
 * Bump Portworx to 3.0.4
 * Add PX-BBQ
 * Documentation
 * Add tags to EBS volumes
 * Use AWS node roles for EBS provisioning

## Fixes
 * Add environment variable AWS_ADD_EKS_IAM_ROLE to fix UI visibility of EKS cluster (see known Issues on EKS)
 * Metro template now works with OCP
 
## Removal
 * auto_destroy parameter
 * quiet parameter

# 5.3

## Fixes
 * Fix intermittent vSphere provisioning issue
 * Remove PDS backups before destroying

## Improvements
 * Migrate GCP to Terraform
 * Add PX-BBQ asset
 * Reduce Kubernetes provision time
 * Basic Kubevirt template

# 5.2

## Fixes
 * Fix issue with destroying large deployments 
 * Correct Rocky URL for vSphere
 * Fix AKS tagging
 * Validation of values in defaults.yml
 * Update OpenSSH to fix scp issues on MacOS
 * Fix auto shutdown so VMs are powered off

## Improvements
 * install.sh no longer runs in a container, so is much faster
 * Delete old assets
 * Improve provisioning parallelisation
 * Add eks_version parameter
 * Add destroy --clear flag
 * Add multicloud migration template
 * SSH root password authentication is disabled by default - provide your SSH public key by setting ssh_pub_key defaults.yml
 * Symlink .terraform directory so create/destroy much faster on MacOS

# 5.1

## Fixes
 * Kubernetes and Portworx image pre-pull now uses containerd
 * Fix Apple Silicon platform build issue
 * Fix metro template deadlock
 * Fix install problem with operator 23.5.0

## Improvements
 * Re-added Azure support, now Terraform based
 * Replace parameter aws_tags with parameter tags (now valid for AWS & Azure)
 * Add Github action trigger builds on commit and release
 * Update install.sh to pull prebuild image

# 5.0.1

## Fixes
 * Bump golang version

# 5.0

## Improvements
 * Migrate AWS to Terraform
 * Move from CentOS 7 to Rocky Linux 8
 * Bump Kubernetes to 1.24.13
 * Bump Portworx to 2.13.5
 * Bump PX-Central to 2.4.2
 
## Fixes
 * Update templates to use external objectstore

# 4.17.1

## Improvements
 * Add vsphere_datacenter

## Fixes
 * awstf AMI for all regions

# 4.17

## Improvements
 * awstf EKS Terraform implemented

## Fixes
 * Fix petclinic replication and io_profile

# 4.16

## Improvements
 * Lots of awstf updates
 * Add containerd to support Kubernetes 1.25
 * Add OCP4 support for awstf
 * Petclinic now always in its own namespace
 * Bump PX-Central to 2.3.2
 * Bump OCP version to 4.10.37

## Fixes
 * Fix intermittent vSphere provisioning bug

# 4.15

## Improvements
 * Lots of awstf updates
 * Refactor training
 * Remove OCP3 support
 * Add performance Grafana dashboard
 * Add gke_version parameter

# 4.14

## Improvements
 * Bump PX-Central to 2.3.0
 * Bump Portworx to 2.11.3
 * Add new cloud awstf - migrate AWS support to Terraform (testing)
 * Add nginx asset

# 4.13.4

## Improvements
 * Bump Flannel to 0.19.2

# 4.13.3

## Improvements
 * Add pxc to .bashrc
 * Bump Portworx to 2.10.3
 * Bump PX-Central to 2.2.1

## Fixes
 * Fix vagrant-vsphere provisioning race condition

# 4.13.2

## Fixes
 * Specify vagrant-vsphere plugin version to fix provision bug

# 4.13.1

## Fixes
 * Bump Vagrant to 2.2.19 to fix build bug

# 4.13

## Improvements
 * Add nomad as a platform
 * Bump PX-Central to 2.2.0
 * Bump Portworx to 2.10.3

## Fixes
 * Find soon-to-be-deprecated CentOS 7 AMI

# 4.12.1

## Improvements
 * Install Helm on EKS
 * Bump Portworx to 2.10.1

## Fixes
 * kubectl/eksctl incompatibility
 * Grafana on EKS

# 4.12

## Improvements
 * Add `kubectl pxc pxctl`
 * Update PX-Central to 2.1.2
 * Update Portworx operator to 1.6.1
 * Bump Kubernetes to 1.21.11
 * Bump Portworx to 2.10.0

## Fixes
 * EKS IAM provisioning
