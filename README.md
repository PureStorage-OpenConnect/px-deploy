# What

This will deploy one or more clusters in the cloud, with optional post-install tasks defined by template.

> [!CAUTION]
> The intended purpose of PX-Deploy is to provision demo and lab clusters as quickly as possible and then destroy them when the session is complete. Nothing should be considered suitable for production.

# Supported platforms

## Container
 * Kubernetes
 * Openshift 4 (only on AWS at this time)
 * EKS (only makes sense on AWS)
 * AKS (only makes sense on Azure)
 * GKE (only makes sense on GCP)

## Cloud
 * AWS
 * Azure
 * GCP
 * vSphere

## Install & Update

0. You need a Unix-like OS. This can be Linux, MacOS or WSL if you are on Windows. Do not use Powershell or Git Bash.

1. Install and enable a container runtime such as Docker. On MacOS with Apple Silicon, it is recommended you use [Docker Desktop](https://www.docker.com/products/docker-desktop/).

2. Install the [Git CLI](https://github.com/git-guides/install-git).

3. Install bash-completion (optional), eg:
```
$ brew install bash-completion
```
You will need to restart your shell.

4. Run the install / update script:
```
curl https://raw.githubusercontent.com/PureStorage-OpenConnect/px-deploy/master/install.sh | bash
```
This will:
 * pull the Docker image from Github
 * create and populate `$HOME/.px-deploy`
 * provide instructions to configure `.bash_profile`/`.zshrc`

It may take 5-10 minutes to complete, longer with a slow Internet connection.

Update your `.bash_profile` or `.zshrc` as directed. Source them or login again. Validate it is complete with `px-deploy -h`.

If on a Mac, install the cron job as directed. This will enable notifications twice a day advising of the ongoing costs of running deployments.

Review the various cloud settings in `~/.px-deploy/defaults.yml`.

5. Configure Cloud specific settings

[Notes for AWS](#notes-for-aws)

[Notes for Azure](#notes-for-azure)

[Notes for GCP](#notes-for-gcp)

[Notes for vSphere](#notes-for-vsphere)

[Notes for OCP4 + AWS](https://github.com/purestorage-openconnect/px-deploy/tree/master/docs/cloud/aws/ocp4.md)

### Notes for Updating

* Run the same steps to update your px-deploy installation

* Please don't change the default scripts / templates as they will be overwritten during update. Feel free to create your own scripts / templates.

* Your `defaults.yml` will no longer be replaced. After updating please check if the new release supports newer Kubernetes / Portworx versions and update it manually.
* You'll find this information in `$HOME/.px-deploy/defaults.yml.VERSION`

* When updating from version < 5.0.0, please destroy all AWS / AWSTF Cloud based deployments in advance. Version 5 cannot destroy these deployments any longer.

* Update your `.bashrc` or `.zshrc` with the new px-deploy alias.

## Use

This will provision a VPC and some other objects, and deploy into it from the template:
```
px-deploy create --name=my-deployment --template=clusterpair
```

Connect via SSH:
```
px-deploy connect --name my-deployment
```

Execute a command:
```
px-deploy connect --name my-deployment "storkctl get clusterpair"
```

Stop instances (to reduce AWS costs):
```
px-deploy stop --name my-deployment
```

Start instances:
```
px-deploy start --name my-deployment
```

Tear down the deployment:
```
px-deploy destroy --name my-deployment
```

# NOTES

The deployments can be listed:
```
$ px-deploy list
DEPLOYMENT  CLOUD  REGION         PLATFORM  TEMPLATE  CLUSTERS  NODES  CREATED
foo         aws    eu-west-1      k8s       px               1      3  2020-02-11T16:14:06Z
bar         gcp    europe-north1  gcp       <none>           1      3  2020-02-04T09:50:11Z
```

The templates can be listed:
```
$ px-deploy templates
NAME                         DESCRIPTION
async-dr                     Deploys 2 clusters with Portworx, sets up and configures a cluster pairing,
                             configures an async DR schedule with a loadbalancer in front of the setup.
backup-restore               Deploys a Kubernetes cluster, Minio S3 storage, Petclinic Application and
                             Backup/Restore config
elk                          Deploys an ELK stack of 3 master nodes, 3 data nodes and 2 coordinator nodes, as per
                             https://docs.portworx.com/portworx-install-with-kubernetes/application-install-with-kubernetes/elastic-search-and-kibana/
harbor                       Deploys a single K8s cluster with Portworx and Harbor (https://goharbor.io/)
metro                        Deploys 2 K8s clusters in AWS with a stretched Portworx cluster. It configures
                             Metro, a GUI and Petclinic, ready for a manual failover demo
migration                    Deploys 2 clusters with Portworx, sets up and configures a cluster pairing, and
                             deploys a set of apps and a migration template.
px-backup                    A single Kubernetes cluster with Portworx and PX-Backup via Helm installed
px-fio-example               An example fio benchmark on a gp2 disk and a Portworx volume on a gp2 disk
px-vs-storageos-fio-example  An example fio benchmark on a gp2 disk, a Portworx volume on a gp2 disk, and a
                             StorageOS volume on a gp2 disk
px                           A single Kubernetes cluster with Portworx installed
storageos                    A single Kubernetes cluster with StorageOS installed
training                     Deploys training clusters
```

Generate kubeconfig files so you can run kubectl from your laptop:
```
$ px-deploy kubeconfig --name exampleDeployment
$ kubectl get nodes --kubeconfig $HOME/.px-deploy/kubeconfig/exampleDeployment.1
NAME                                           STATUS   ROLES    AGE   VERSION
ip-192-168-5-19.eu-west-1.compute.internal     Ready    <none>   11m   v1.21.5-eks-9017834
ip-192-168-56-14.eu-west-1.compute.internal    Ready    <none>   11m   v1.21.5-eks-9017834
ip-192-168-74-141.eu-west-1.compute.internal   Ready    <none>   11m   v1.21.5-eks-9017834
```
Note this is currently only tested with EKS.

The `defaults.yml` file sets a number of deployment variables:
 * `aws_ebs` - a list of EBS volumes to be attached to each worker node. This is a space-separated list of type:size pairs, for example: `"gp2:30 standard:20"` will provision a gp2 volume of 30 GB and a standard volume of 20GB
 * `aws_region` - AWS region
 * `aws_type` - the AWS machine type for each node
 * `aws_access_key_id` - your AWS API access key
 * `aws_secret_access_key` - your AWS API secret access key
 * `eks_version` - EKS k8s Version
 * `cloud` - the cloud on which to deploy (aws, gcp, or vsphere)
 * `ssh_pub_key` - ssh public key which will be added for root access on each node
 * `clusters` - the number of clusters to deploy
 * `k8s_version` - the version of Kubernetes to deploy
 * `tags` - a list of tags to be applied to each node. This is a comma-separate list of name=value pairs, for example: `"Owner=Bob,Purpose=Demo"`
 * `stop_after` - stop the intances after this many hours
 * `post_script` - script to run on each master after deployment, output will go to stdout
 * `nodes` - the number of worker nodes on each cluster
 * `lock` - prevents deployment from being deleted by `px-deploy destroy ...`. use `px-deploy unlock -n ...` to unlock 
 * `platform` - can be set to either k8s, none, ocp4, eks or gke
 * `azure_type` - the Azure machine type for each node
 * `azure_disks` - similar to aws_ebs, for example: `"Standard_LRS:49 Premium_LRS:50"` 
 * `azure_client_id` - Azure Client ID
 * `azure_client_secret` - Azure Access Secret
 * `azure_subscription_id` - Azure Subscription ID
 * `azure_tenant_id` - Azure Tenant ID
 * `aks_version` - AKS k8s Version
 * `px_version` - the version of Portworx to install
 * `gcp_project` - GCP project
 * `gcp_disks` - similar to aws_ebs, for example: `"pd-standard:20 pd-ssd:30"`
 * `gcp_region` - GCP region
 * `gcp_type` - the GCP machine type for each node
 * `gcp_zone` - GCP zone
 * `gke_version` - GKE k8s Version
 * `vsphere_host` - endpoint
 * `vsphere_compute_resource` - compute resource
 * `vsphere_user` - user with which to provision VMs
 * `vsphere_password` - password
 * `vsphere_template` - full path to CentOS 7 template
 * `vsphere_datacenter` - vcenter datacenter
 * `vsphere_datastore` - datastore prefix
 * `vsphere_folder` - folder for vSphere VMs
 * `vsphere_disks` - similar to aws_ebs, for example: `"20 30"` (NOTE: these are not provisioned as static block devices, but they are used as clouddrives)
 * `vsphere_network` - vSwitch or dvPortGroup for cluster ex: Team-SE-120
 * `vsphere_memory` - RAM in GB
 * `vsphere_cpu` - number of vCPUs
 * `vsphere_repo` - repository to pull vsphere template
 * `ocp4_domain` - domain that has been delegated to route53
 * `ocp4_version` - eg `4.3.0`
 * `ocp4_pull_secret` - the pull secret `'{"auths" ... }'`

There are two ways to override these variables. The first is to specify a template with the `--template=...` parameter. For example:
```
$ cat templates/px-fio-example.yml
description: An example fio benchmark on a gp2 disk and a Portworx volume on a gp2 disk
scripts: ["install-px", "px-wait", "px-fio-example"]
clusters: 1
nodes: 3
cloud: aws
aws_ebs: "gp2:150 gp2:150"
post_script: cat
auto_destroy: true
env:
  px_suffix: "s=/dev/nvme1n1"
  cat: "/tmp/output"
```

More on `scripts` below.

The second way to override the defaults is to specify on the command line. See `px-deploy create -h` for a full list. For example, to deploy petclinic into the `foo` deployment:
```
px-deploy create --name=foo --clusters=5 --template=petclinic --nodes=6
```

This example is a mixture of both methods. The template is applied, then the command line parameters are applied, so not only is the template overriding the defaults, but also the parameters are overriding the template.

`scripts` is a list of scripts to be executed on each master node. For example:
```
$ cat ~/.px-deploy/scripts/petclinic
# Install petclinic on each cluster
kubectl apply -f /assets/petclinic.yml
```

These variables are passed to the script:
 * `$nodes`
 * `$clusters`
 * `$px_version`
 * `$k8s_version`

You can also select a different defaults file with:
```
$ DEFAULTS=/path/to/other/defaults.yml px-deploy create ...
```

All files in `~/.px-deploy/assets` will be copied to `/assets` on the master nodes. They are then available to be used by the script, as above.

In addition to these, there are some more variables available:
 * `$cluster` - cluster number

There is also an optional cluster object. At the moment, all it can be used for is defining cluster-specific scripts. These will be executed after the scripts above. For example:
```
cluster:
- id: 1
  scripts: ["script-1", "script-2"]
- id: 2
  scripts: ["script-3", "script-4"]
```

`post_script` is a script that will be run on each master node after all of the scripts have completed, and the output will go to stdout. The default is to display the external IP address of master-1, but it could be used to show benchmark outputs, for example.

Last, environment variables can be define in templates or defaults.yml, and these are also available to scripts:
```
$ cat templates/metro.yml
...
env:
  licenses: "XXXX-XXXX-XXXX-XXXX-XXXX-XXXX-XXXX-XXXX XXXX-XXXX-XXXX-XXXX-XXXX-XXXX-XXXX-XXXX"
```
Enviroment variables can also be defined on the command line:
```
px-deploy create -n foo -t migration -e install_apps=true,foo=bar
```

The `install-px` script looks for an environment variable called `cloud_drive`. If it exists, it will deploy Portworx using a clouddrive rather than looking for all attached devices. Note that this is a requirement for Openshift 4. For example:
```
px-deploy create -n foo -t px --cloud aws -e cloud_drive=type%3Dgp2%2Csize%3D150
px-deploy create -n foo -t px --cloud aws --platform eks -e cloud_drive=type%3Dgp2%2Csize%3D150
px-deploy create -n bar -t px --cloud aws --platform ocp4 -e cloud_drive=type%3Dgp2%2Csize%3D150
px-deploy create -n baz -t px --cloud gcp --platform gke -e cloud_drive="type%3Dpd-standard%2Csize%3D150"
px-deploy create -n foo -t px --cloud azure --platform aks -e cloud_drive="type%3DStandard_LRS%2Csize%3D150"
```
# Notes for AWS

If you are using AWS and you have not accepted the Rocky Linux 8 terms, browse to https://aws.amazon.com/marketplace/pp/prodview-2otariyxb3mqu

Edit your `defaults.yml` to set `aws_access_key_id` and `aws_secret_access_key`.

# Notes for Azure

1. Get access to Azure cli (either you install or use Cloud Shell on Azure UI)

2. Edit your `defaults.yml` to set `azure_tenant_id`, `azure_subscription_id`, `azure_client_id`, `azure_client_secret`.

```
$ az account list
```

value of `id` is your `azure_subscription_id` value

value of `tenant` is your `azure_tenant_id` value

3. create application secret

```
$ az ad sp create-for-rbac --role="Contributor" --scopes="/subscriptions/[your azure_subscription_id]"
```

value of `appId` is your `azure_client_id`

value of `password` is your `azure_client_secret`

4. accept RockyLinux EULA

```
$ az vm image terms accept --urn "resf:rockylinux-x86_64:8-base:8.9.20231119" 
```

# Notes for GCP

**tl;dr**

- setup a project

- create service account with roles compute.admin,container.admin,iam.serviceAccountUser

- download json key file for service account and move it as **`gcp.json`** into your ~/.px-deploy folder

- set gcp_project in ~/.px-deploy/defaults.yml

**Detailed:**

1. install CLI and ensure it is configured: https://cloud.google.com/sdk/docs/quickstarts

2. you can copy & paste the following commands. just ensure to replace all values shown in [brackets] with your specific values - without the [brackets]

3. set env variable for project / service account (example here **`px-deploy-proj`** and **`px-deploy-sa`**)

```
PX_DEPLOY_PROJECT="[px-deploy-proj]"

SERVICE_ACCOUNT="[px-deploy-sa]"
```

4. create a new project and set it as default

```
gcloud projects create ${PX_DEPLOY_PROJECT}

gcloud config set project ${PX_DEPLOY_PROJECT}
```

5. get your billing account id 

```
gcloud beta billing accounts list
```

Output will look like this:

```
ACCOUNT_ID            NAME                OPEN  MASTER_ACCOUNT_ID

654321-ABCDEF-123456  My Billing Account  True
```
6. link your project to billing account id  (here **`654321-ABCDEF-123456`**)

```
gcloud beta billing projects link ${PX_DEPLOY_PROJECT} --billing-account [654321-ABCDEF-123456]
```

7. create service account

```
gcloud iam service-accounts create ${SERVICE_ACCOUNT}
```

8. assign roles to service-account
 
```
gcloud projects add-iam-policy-binding ${PX_DEPLOY_PROJECT} \
   --member="serviceAccount:${SERVICE_ACCOUNT}@${PX_DEPLOY_PROJECT}.iam.gserviceaccount.com" \
   --role="roles/compute.admin"

gcloud projects add-iam-policy-binding ${PX_DEPLOY_PROJECT} \
   --member="serviceAccount:${SERVICE_ACCOUNT}@${PX_DEPLOY_PROJECT}.iam.gserviceaccount.com" \
   --role="roles/container.admin"

gcloud projects add-iam-policy-binding ${PX_DEPLOY_PROJECT} \
   --member="serviceAccount:${SERVICE_ACCOUNT}@${PX_DEPLOY_PROJECT}.iam.gserviceaccount.com" \
   --role="roles/iam.serviceAccountUser"  
```

9. download service account json key file (here **`gcp.json`**)

```
gcloud iam service-accounts keys create gcp.json \
    --iam-account=${SERVICE_ACCOUNT}@${PX_DEPLOY_PROJECT}.iam.gserviceaccount.com
```

10. move **`gcp.json`** into your ~/.px-deploy/ folder

```
mv gcp.json ~/.px-deploy/
```

11. enable compute & kubernetes API

```
gcloud services enable container.googleapis.com

gcloud services enable compute.googleapis.com
```

12. edit your `~/.px-deploy/defaults.yml` to add 

```
gcp_project: "[px-deploy-proj]"
```

# Notes for vSphere

Before you can start deploying in vSphere, a local VM template must be installed. The command
```
$ px-deploy vsphere-init
```
will read the vsphere variables from `defaults.yml` (please check in advance if they are correct) and provision a template at the path defined in `vsphere_template`.

# Known Issues 

## Known Issues on EKS

If you see on AWS UI an error message like this: `Your current IAM principal doesn’t have access to Kubernetes objects on this cluster.` the IAM user being used to create the deployment isn't the same IAM user running the AWS UI. 

One reason could be that access to AWS UI is authenticated using SSO. 

To fix the isse find out the user/role being used by AWS UI (e.g. by running `aws sts get-caller-identity` on AWS UI CloudShell) and set one of the following ENV variables in ~/.px-deploy/defaults.yml (in my example its the `OktaSSOAdminUser` Role / `developerUser1` user)

```
env:
    AWS_ADD_EKS_IAM_ROLE: "OktaSSOAdminUser"
    AWS_ADD_EKS_IAM_USER: "developerUser1"
```

More information can be found here https://docs.aws.amazon.com/eks/latest/userguide/view-kubernetes-resources.html#view-kubernetes-resources-permissions

