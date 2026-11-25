# px-deploy User Guide

Installing, configuring and running px-deploy, from first install to tearing a lab down. For how px-deploy works inside (what `create` does step by step, where state lives, how clouds plug in), see the [Theory of Operations](theory-of-operations.md).

## 1. Introduction

px-deploy is an open-source command-line tool that builds Kubernetes and OpenShift lab environments, installs Portworx on them, and tears them down again. It can build several clusters per deployment, which is what the DR, Metro and migration templates rely on.

> [!CAUTION]
> px-deploy is for labs, workshops, training and demos. It aims to stand Portworx up as fast as possible (a plain Kubernetes cluster takes roughly 5 to 7 minutes) and to be destroyed when the session ends. Nothing it builds is suitable for production.

### How it runs, in brief

px-deploy runs on your workstation inside a Docker container, behind a shell function that the installer prints for you. For each deployment it:

1. renders Terraform files and runs Terraform to build the cloud resources (network, VMs, disks);
2. uploads its setup scripts and your `assets` folder to the VMs;
3. lets every VM run its setup at first boot; the first master of each cluster also runs the template's scripts (install Portworx, apps, demos), and `px-deploy status` reports when they are done.

### Supported platforms

| Platform (`platform`) | AWS | Azure | GCP | vSphere | Notes |
|---|---|---|---|---|---|
| Kubernetes (`k8s`) | Yes | Yes | Yes | Yes | Default. About 5 to 7 minutes. |
| None (`none`) | Yes | Yes | Yes | Yes | VMs only, no Kubernetes. |
| OpenShift 4 (`ocp4`) | Yes | No | No | No | About 45 minutes. |
| EKS (`eks`) | Yes | No | No | No | Managed Kubernetes. |
| AKS (`aks`) | No | Yes | No | No | Managed Kubernetes. |
| GKE (`gke`) | No | No | Yes | No | Managed Kubernetes. |
| Rancher (`rancher`) | Yes | No | No | No | Experimental. |

## 2. Prerequisites

- **A Unix-like OS:** Linux, macOS, or WSL on Windows. Do not run px-deploy from PowerShell or Git Bash.
- **A container runtime** such as Docker, installed and running. On Apple Silicon Macs, use Docker Desktop.
- **The Git CLI.**
- **Optional:** shell completion. On macOS:

  ```shell
  brew install bash-completion
  ```

  Restart your shell afterwards.

## 3. Install and update

Run the installer:

```shell
curl https://raw.githubusercontent.com/PureStorage-OpenConnect/px-deploy/master/install.sh | bash
```

It:

1. clones the repository, checks out the current release, and pulls that release's image from `ghcr.io/purestorage-openconnect/px-deploy`;
2. creates `$HOME/.px-deploy` and copies the release's `infra`, `scripts`, `templates`, `assets` and `docs` into it. If you already had those folders, they are kept as `<folder>.<timestamp>` and replaced;
3. copies `defaults.yml` into place only if you don't have one yet, and always writes the release's recommended versions to `$HOME/.px-deploy/versions.yml`;
4. prints the shell function to add to `~/.zshrc` or `~/.bash_profile`.

Then:

```shell
source ~/.zshrc        # or: source ~/.bash_profile
px-deploy -h
```

To update, run the installer again and add the newly printed shell function. Your `defaults.yml` is never replaced: compare it with `versions.yml` and update versions by hand. `px-deploy create` warns when a version in your `defaults.yml` is lower than the recommended one.

`px-deploy create` also refuses to run when a newer px-deploy release exists. Update, or pass `--ignore_version` to continue for now.

### Cost reminder (optional cron job)

`px-deploy.cron` runs `px-deploy_cron.sh` at 08:00 and 16:00. It counts your AWS deployments and shows a dialog with a rough daily cost. It is a reminder only: it does not stop or destroy anything. Setup steps are in [cron.md](cron.md).

## 4. Configuration (`~/.px-deploy/defaults.yml`)

`defaults.yml` holds the defaults for every deployment. A template can override them, and command-line flags override both (see section 6).

To use another settings file for one run, set `DEFAULTS`:

```shell
DEFAULTS=/path/to/other-defaults.yml px-deploy create --name=my-deployment
```

### Global settings

| Key | Meaning | Example |
|---|---|---|
| `cloud` | Where to build: `aws`, `azure`, `gcp` or `vsphere` | `aws` |
| `platform` | `k8s`, `none`, `ocp4`, `rancher`, `eks`, `aks` or `gke` | `k8s` |
| `clusters` | Clusters per deployment | `1` |
| `nodes` | Worker nodes per cluster | `3` |
| `k8s_version` | Kubernetes version | see `versions.yml` |
| `px_version` | Portworx version | see `versions.yml` |
| `ssh_pub_key` | An extra public key added for root on every VM | `"ssh-ed25519 AAAA... you@example"` |
| `tags` | Tags on cloud resources, comma-separated `key=value` | `"Owner=Bob,Purpose=Demo"` |
| `lock` | Refuse `destroy` until `px-deploy unlock` | `false` |
| `env` | Extra variables passed to the setup scripts | see section 6 |

### 4.1 AWS

```yaml
aws_access_key_id: "AKIA................"
aws_secret_access_key: "........................................"
aws_region: "eu-west-1"
aws_type: "t3.large"
aws_ebs: "gp2:50"
eks_version: "1.35"
```

- `aws_ebs`: space-separated `type:size_in_GB` disks attached to each worker, for example `"gp2:30 standard:20"`. It can be left empty when Portworx uses cloud drives (section 7).
- Your account must be subscribed to the Rocky Linux 9 (Official) x86_64 image on AWS Marketplace.

### 4.2 Azure

```yaml
azure_tenant_id: "00000000-0000-0000-0000-000000000000"
azure_subscription_id: "00000000-0000-0000-0000-000000000000"
azure_client_id: "00000000-0000-0000-0000-000000000000"
azure_client_secret: "................................"
azure_region: "uksouth"
azure_type: "Standard_B2ms"
azure_disks: "Standard_LRS:50"
aks_version: "1.35"
```

1. Find your subscription (`id`) and tenant (`tenantId`):

   ```shell
   az account list
   ```

2. Create a service principal with Contributor rights. `appId` is your `azure_client_id`, `password` is your `azure_client_secret`:

   ```shell
   az ad sp create-for-rbac --role="Contributor" --scopes="/subscriptions/<azure_subscription_id>"
   ```

3. Accept the Rocky Linux image terms:

   ```shell
   az vm image terms accept --urn "resf:rockylinux-x86_64:9-base:9.5.20241118"
   ```

### 4.3 GCP

```yaml
gcp_project: "px-deploy-proj"
gcp_region: "europe-north1"
gcp_zone: "b"
gcp_type: "n1-standard-4"
gcp_disks: "pd-standard:50"
gke_version: "1.35"
```

`gcp_zone` is the zone letter within `gcp_region` (`a`, `b` or `c`).

1. Name the project and service account:

   ```shell
   PX_DEPLOY_PROJECT="px-deploy-proj"
   SERVICE_ACCOUNT="px-deploy-sa"
   ```

2. Create the project and link billing:

   ```shell
   gcloud projects create ${PX_DEPLOY_PROJECT}
   gcloud config set project ${PX_DEPLOY_PROJECT}
   gcloud beta billing projects link ${PX_DEPLOY_PROJECT} --billing-account <BILLING_ACCOUNT_ID>
   ```

3. Create the service account and grant it `compute.admin`, `container.admin` and `iam.serviceAccountUser`:

   ```shell
   gcloud iam service-accounts create ${SERVICE_ACCOUNT}
   for role in compute.admin container.admin iam.serviceAccountUser; do
     gcloud projects add-iam-policy-binding ${PX_DEPLOY_PROJECT} \
       --member="serviceAccount:${SERVICE_ACCOUNT}@${PX_DEPLOY_PROJECT}.iam.gserviceaccount.com" \
       --role="roles/${role}"
   done
   ```

4. Save a key as `~/.px-deploy/gcp.json`:

   ```shell
   gcloud iam service-accounts keys create ~/.px-deploy/gcp.json \
     --iam-account=${SERVICE_ACCOUNT}@${PX_DEPLOY_PROJECT}.iam.gserviceaccount.com
   ```

5. Enable the APIs:

   ```shell
   gcloud services enable container.googleapis.com compute.googleapis.com
   ```

### 4.4 vSphere

```yaml
vsphere_host: "vcenter.example.com"
vsphere_user: "administrator@vsphere.local"
vsphere_password: "..."
vsphere_datacenter: "Datacenter1"
vsphere_compute_resource: "Cluster1"
vsphere_resource_pool: "px-deploy"
vsphere_datastore: "datastore1"
vsphere_folder: "px-deploy-vms"
vsphere_template: "pxdeploy-template"
vsphere_network: "VM Network"
vsphere_memory: 8
vsphere_cpu: 2
vsphere_disks: "64"
vsphere_repo: "https://px-deploy.s3.eu-west-1.amazonaws.com/templates/"
```

- **Template:** every deployment clones the VM template named in `vsphere_template`, so it must exist first. Either run `px-deploy vsphere-init`, which imports `template.ova` from `vsphere_repo`, or deploy that OVA from the vCenter UI, which is often faster: see [Manual creation of px-deploy vSphere template](cloud/vsphere/README.md). After each `create`, px-deploy compares your template's version with the published one and tells you when to re-run `vsphere-init`. `px-deploy vsphere-check-template -n <name>` runs the same check on demand.
- **Disks:** `vsphere_disks` (for example `"20 30"`) is not attached when the VMs are built. Portworx creates these disks itself as vSphere cloud drives.

### 4.5 OpenShift 4 on AWS

```yaml
ocp4_domain: "ocp.example.com"
ocp4_version: "4.22.13"
ocp4_credentials_mode: "Mint"
ocp4_pull_secret: '{"auths":{...}}'
```

- `ocp4_domain`: a public domain delegated to Route 53.
- `ocp4_credentials_mode`: the Cloud Credential Operator mode, `Mint` (default) or `Passthrough`.
- More in [OpenShift 4 on AWS](cloud/aws/ocp4.md).

## 5. Commands

| Command | What it does |
|---|---|
| `create` | Build a deployment |
| `list` | List your deployments |
| `status -n <name>` | Show whether each master and node has finished its setup |
| `connect -n <name> [command]` | SSH to the first master as root, or run one command there |
| `kubeconfig -n <name>` | Copy each cluster's kubeconfig to `~/.px-deploy/kubeconfig/` |
| `stop` / `start -n <name> [-c <cluster>]` | Stop or start the VMs (AWS only; not for `ocp4` or `eks`) |
| `destroy -n <name>` | Tear the deployment down |
| `unlock -n <name>` | Remove a deployment's lock so it can be destroyed |
| `templates` | List the templates |
| `history [-n <id>]` | Show past `create` runs, or the full settings of one |
| `version` | Print the px-deploy version |
| `vsphere-init` | Import the vSphere VM template |
| `vsphere-check-template -n <name>` | Compare your vSphere template with the published one |
| `completion` | Print shell completion code |

### create

```shell
px-deploy create --name=my-demo
px-deploy create --name=foo --clusters=2 --nodes=6 --template=migration -e install_apps=true
```

Useful flags: `-n/--name`, `-t/--template`, `-C/--cloud`, `-p/--platform`, `-c/--clusters`, `-N/--nodes`, `-r/--region`, `-e/--env` (comma-separated `key=value`), `--lock`, `--no_sync` (section 6), `--ignore_version`, and `-d/--dry_run` (render the Terraform files and run the plan without building anything; AWS and Azure only; remove the local files afterwards with `px-deploy destroy -n <name>`). Names may use lower-case letters, digits, `-`, `_` and `.`; without a name, px-deploy uses a UUID.

### list

```text
$ px-deploy list
DEPLOYMENT  CLOUD  REGION         PLATFORM  TEMPLATE  CLUSTERS  NODES/CL  CREATED
foo         aws    eu-west-1      k8s       px               1      3  2026-02-11T16:14:06Z
bar         gcp    europe-north1  gke       <none>           1      3  2026-02-04T09:50:11Z
```

### connect

Connects to the first master of cluster 1 as root, with the key px-deploy generated for this deployment when it was created (`~/.px-deploy/keys/id_rsa.<cloud>.<name>`):

```shell
px-deploy connect -n my-demo
px-deploy connect -n my-demo "kubectl get nodes"
```

From there, the other masters and nodes answer to their host names (`master-2`, `node-1-1`, ...).

### status

```shell
px-deploy status -n my-demo
```

Each master and node shows `Ready` once its setup has finished. The setup logs are on each master in `/var/log/px-deploy`, one file per script:

```shell
px-deploy connect -n my-demo "ls -lrt /var/log/px-deploy"
```

### stop and start

```shell
px-deploy stop -n my-demo
px-deploy start -n my-demo
```

AWS only, and not for `ocp4` or `eks`. Instances stop and keep their disks; `-c <n>` limits it to one cluster.

### destroy

```shell
px-deploy destroy -n my-demo
```

Before Terraform removes the infrastructure, px-deploy copies each master's setup logs to `~/.px-deploy/logs/` (the last 10 are kept), runs any pre-delete scripts the templates installed, and on AWS, GCP and vSphere deletes the Portworx cloud drives, which Terraform does not know about.

- `--force`: continue even if a pre-delete script fails.
- `-a`: destroy every deployment that is not locked.
- `--clear`: delete only the local files, for a deployment whose cloud resources are already gone. Anything still in the cloud must then be removed by hand.

A locked deployment refuses `destroy`:

```shell
px-deploy unlock -n my-demo
px-deploy destroy -n my-demo
```

### kubeconfig

```shell
px-deploy kubeconfig -n my-demo
kubectl get nodes --kubeconfig ~/.px-deploy/kubeconfig/my-demo.1
```

The file suffix is the cluster number.

## 6. Templates, scripts and assets

### Precedence

`defaults.yml`, then the template, then command-line flags: each overrides the one before. `env` values merge the same way, with `-e` last.

### Templates

Templates live in `~/.px-deploy/templates/`:

```yaml
description: Deploys 2 clusters with Portworx, sets up a cluster pair and an async DR schedule
clusters: 2
cloud: aws
scripts: ["install-awscli", "install-px", "licenses"]
cluster:
  - id: 1
    scripts: ["aws-elb"]
  - id: 2
    scripts: ["clusterpair"]
env:
  licenses: "XXXX-XXXX-XXXX-XXXX-XXXX-XXXX-XXXX-XXXX"
```

`scripts` run on the first master of every cluster, in order. `cluster[].scripts` run after them, on that cluster's master only. `cluster[].nodes` can give one cluster a different node count.

### Scripts and their variables

Scripts live in `~/.px-deploy/scripts/` and run as root. They can read:

- every setting from `defaults.yml`, the template and the flags, as a lower-case variable (`$cloud`, `$platform`, `$clusters`, `$nodes`, `$k8s_version`, `$px_version`, `$aws_region`, ...);
- every `env` entry, under its own name;
- `$cluster` (this cluster's number), `$clusternodes` (an array of node counts per cluster) and `$pxd_uuid` (the deployment's id).

### Keep your own changes

Every `create` copies the release's `assets`, `scripts`, `templates` and `infra` over your `~/.px-deploy` folders. Files with the same name are overwritten; files with new names are left alone. So give your own templates and scripts new names, or pass `--no_sync` while you work on them:

```shell
px-deploy create --name=dev-test --template=my-template --no_sync
```

### Assets

Everything in `~/.px-deploy/assets` is copied to `/assets` on every master and node. A script can then run, for example:

```shell
kubectl apply -f /assets/petclinic.yaml
```

### Reaching applications and terminals

- **NodePort:** a `type: NodePort` service answers on any worker's public address, for example `http://<node-ip>:30333` for the Petclinic demo.
- **Load balancers and DNS (AWS):** templates that use the AWS Load Balancer Controller and ExternalDNS publish names in a Route 53 zone you own.
- **Browser terminals (AWS):** the `wetty` script serves a shell over HTTPS for classrooms where students can't use an SSH client. It publishes `<name>-wetty` and `<name>-ssh` records in the Route 53 zone named by `ocp4_domain`, using the certificate in the `training_cert` and `training_key` variables, and removes the records on `destroy`. Log-in accounts come from the training templates.

## 7. Storage for Portworx

px-deploy gives Portworx its storage one of two ways:

1. **Attached disks:** `aws_ebs`, `azure_disks` and `gcp_disks` are created and attached to each worker when the VMs are built.
2. **Cloud drives:** Portworx creates and attaches its own disks through the cloud's API. vSphere always works this way (`vsphere_disks`).

OpenShift 4 and the managed services (EKS, AKS, GKE) build their own workers, so they always need cloud drives.

Turn cloud drives on with the `cloud_drive` variable. Its value is URL-encoded because `-e` itself splits on `,` and `=`:

```shell
# AWS, Kubernetes
px-deploy create -n foo -t px --cloud aws -e cloud_drive=type%3Dgp2%2Csize%3D150
# AWS EKS
px-deploy create -n foo -t px --cloud aws --platform eks -e cloud_drive=type%3Dgp2%2Csize%3D150
# AWS OpenShift 4
px-deploy create -n bar -t px --cloud aws --platform ocp4 -e cloud_drive=type%3Dgp2%2Csize%3D150
# GCP GKE
px-deploy create -n baz -t px --cloud gcp --platform gke -e cloud_drive="type%3Dpd-standard%2Csize%3D150"
# Azure AKS
px-deploy create -n foo -t px --cloud azure --platform aks -e cloud_drive="type%3DStandard_LRS%2Csize%3D150"
```

## 8. Troubleshooting

### `status` says "Error get status of cluster 1", or SSH fails just after create

Terraform has finished, but the first master hasn't finished booting or isn't answering SSH yet.

1. Wait a minute or two and run `status` again.
2. Check in the cloud console that the instance is running and its checks have passed.
3. Connect and read the setup logs:

   ```shell
   px-deploy connect -n <name>
   ls -lrt /var/log/px-deploy
   cat /var/log/px-deploy/<script>
   ```

### EKS: "Your current IAM principal doesn't have access to Kubernetes objects on this cluster"

The console is signed in as a different IAM principal from the one px-deploy used (common with SSO roles). Find the console identity (for example `aws sts get-caller-identity` in CloudShell) and add it under `env` in `defaults.yml`:

```yaml
env:
  AWS_ADD_EKS_IAM_ROLE: "MySSOAdminRole"
  # or
  AWS_ADD_EKS_IAM_USER: "developerUser1"
```

### AWS: `InvalidClientTokenId` or "invalid security token"

The key was mistyped or truncated, or the region is blocked for your account. Check that `aws_access_key_id` starts with `AKIA` or `ASIA`, and that `aws_region` is one your organization allows.

### OpenShift 4 seems to hang

It isn't hung. The OpenShift installer builds its own control plane and bootstrap node, which takes about 45 minutes. Don't interrupt it.
