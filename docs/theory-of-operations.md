# px-deploy Theory of Operations

How px-deploy works inside: what runs where, in what order, where state lives and why things fail. It is written for people who maintain px-deploy, add a cloud or a template, or debug a deployment. For installing and using px-deploy, see the [User Guide](user-guide.md) and the [README](../README.md).

File and function names refer to this repository; the line of code is the authority when this document and the code disagree.

## 1. Purpose and boundaries

px-deploy turns one command into one or more working Kubernetes or OpenShift clusters with Portworx, on a public cloud or vSphere, and removes them again. Its design choices follow from that purpose:

- **Speed over hardening.** VMs get a shared root key, password SSH stays on, SELinux and the firewall are off, and credentials are passed to the VMs as plain environment variables. These are lab conveniences, not defaults to copy.
- **Disposable state.** A deployment is meant to live for hours or days. There is no upgrade path for a running deployment; you destroy it and create a new one.
- **Shell scripts as the extension point.** Everything after the VMs exist (Kubernetes, Portworx, demos) is a Bash script, so a template author never touches Go or Terraform.

## 2. The parts

| Part | Where it lives | Role |
|---|---|---|
| Shell function | your `~/.zshrc` or `~/.bash_profile` | Starts the container for each command |
| Container image | `ghcr.io/purestorage-openconnect/px-deploy:<version>` | The `px-deploy` binary, Terraform 1.9.8, `govc`, `ssh`, `rsync`, and pre-initialized Terraform modules for each cloud |
| Your state | `~/.px-deploy` on your machine, mounted at `/px-deploy/.px-deploy` | Settings, deployments, keys, Terraform working directories, logs |
| Cloud resources | your cloud account or vCenter | Networks, VMs, disks |
| VMs | Rocky Linux (the Rocky 9 cloud images on AWS, Azure and GCP; the px-deploy template on vSphere) | Run the setup scripts at first boot |

The binary is a Cobra CLI in `px-deploy.go`, with one file per cloud (`aws.go`, `azure.go`, `gcp.go`, `vsphere.go`) for the work Terraform does not do: finding a VM's address, stopping instances, cleaning up disks Portworx created.

## 3. Execution model

The installer prints a shell function like this one:

```shell
px-deploy() { [ "$DEFAULTS" ] && params="-v $DEFAULTS:/px-deploy/.px-deploy/defaults.yml" ; docker run --network host -it -e PXDUSER=$USER --rm --name px-deploy.$$ $params -v $HOME/.px-deploy:/px-deploy/.px-deploy px-deploy /root/go/bin/px-deploy "$@" ; }
```

So every command:

- runs in a fresh, throw-away container (`--rm`) that shares your machine's network (`--network host`);
- sees `~/.px-deploy` at `/px-deploy/.px-deploy`, and changes into it before doing anything. Every relative path in this document is relative to that folder;
- sees an alternative settings file in place of `defaults.yml` when `DEFAULTS` is set.

Nothing persists in the container. Everything px-deploy needs between commands is in `~/.px-deploy`.

### The release check

`create` reads `/VERSION` from the image and the `VERSION` file on the repository's `master` branch. If they differ, it stops unless `--ignore_version` is given. If GitHub can't be reached, it carries on. Other commands don't check.

## 4. State on disk

| Path under `~/.px-deploy` | Written by | Contents |
|---|---|---|
| `defaults.yml` | you | Your settings. Never overwritten by the installer. |
| `versions.yml` | installer, `create` | The release's recommended versions; `create` warns when yours are lower |
| `templates/`, `scripts/`, `infra/`, `assets/` | installer, every `create` | Copies of the image's files (see section 6) |
| `deployments/<name>.yml` | `create` | The merged settings of one deployment; later commands read this, not `defaults.yml` |
| `tf-deployments/<name>/` | `create` | Terraform working directory: `.tf` files, `.tfvars`, `env.sh`, one generated script per VM, `tfplan`, the Terraform state |
| `tf-deployments/<name>/<name>.lock` | `create --lock` | Presence alone blocks `destroy` |
| `keys/id_rsa.<cloud>.<name>` and `.pub` | Terraform | The deployment's SSH key pair |
| `kubeconfig/<name>.<cluster>` | `kubeconfig` | One kubeconfig per cluster |
| `logs/<name>_<time>/` | `destroy` | Setup logs copied from each master; the newest 10 are kept |
| `log` | `create` | One line per `create`: the arguments and the full settings, base64-encoded. `history` reads it. |
| `gcp.json` | you | GCP service-account key |

Two details matter when debugging:

- **AWS keys are not copied into `deployments/<name>.yml`.** Commands that call AWS later (`status`, `stop`, `destroy`) read them from `defaults.yml` at that moment. Rotating your key works; deleting it from `defaults.yml` breaks `destroy` of existing AWS deployments.
- **The Terraform state is only in `tf-deployments/<name>/`.** Lose that folder and px-deploy can no longer destroy the deployment; `destroy --clear` then removes the local files and the cloud resources must be removed by hand.

## 5. Configuration and precedence

`create` builds one settings object from three layers, each overriding the one before:

1. `defaults.yml`
2. the template (`templates/<name>.yml`), if `-t` is given
3. command-line flags, including `-e`

Scalar settings are replaced; the `env` maps are merged key by key, so `-e foo=bar` adds or replaces `foo` and keeps every other `env` entry. `-e` splits its argument on `,` and `=`, which is why values such as `cloud_drive` are URL-encoded.

The merged object is validated (`validate_config`) before anything is written. The rules include: valid cloud and platform names, platform-and-cloud pairs (`ocp4` and `rancher` only on AWS, `gke` only on GCP, `aks` only on Azure), the settings each cloud needs, and the deployment name (`^[a-z0-9_\-\.]+$`; a UUID if none is given). Settings files containing non-ASCII characters are refused.

Every deployment also gets a random `pxd_uuid`. Cloud resources are tagged with it, and later commands find instances and Portworx disks by that tag rather than by name.

## 6. The sync model

Each `create` first runs:

```shell
rsync -a /px-deploy/assets /px-deploy/scripts /px-deploy/templates /px-deploy/infra /px-deploy/versions.yml /px-deploy/.px-deploy/
```

That copies the image's files over yours. A file with the same name as one in the image is overwritten; a file with a new name is untouched. This is why the shipped templates say "do not edit": edit a copy under a new name, or pass `--no_sync` while you iterate on a shipped file.

The installer does a heavier version of the same thing: it backs up `infra`, `scripts`, `templates`, `assets` and `docs` as `<folder>.<timestamp>` and replaces them.

## 7. Lifecycle of `create`

```text
your shell                container (px-deploy)                    cloud / VMs
----------                ---------------------                    -----------
px-deploy create ──────▶  release check
                          rsync image files into ~/.px-deploy
                          merge settings, validate
                          write deployments/<name>.yml, log
                          copy terraform/<cloud>/*.tf into
                            tf-deployments/<name>/
                          write env.sh and one script per VM
                            (each checked with bash -n)
                          write .tfvars
                          terraform plan, terraform apply ──────▶  network, VMs, disks
                                                                    key pair → keys/
                                                                    upload env.sh, the
                                                                    VM's script, assets/
                          append cloud outputs to
                            deployments/<name>.yml
                          create lock file (--lock)
◀────── returns                                                     VMs keep running
                                                                    their scripts
px-deploy status ──────▶  ssh master-<c>-1:                ◀──────  tracking file fills
                            cat /var/log/px-deploy/completed/tracking
```

`create` returns as soon as Terraform finishes. The minutes that follow, while Kubernetes and Portworx install, happen on the VMs, and `status` is how you watch them.

### 7.1 What each VM runs

`write_nodescripts` builds one script per VM by concatenating files. Each part runs in a subshell, logs to `/var/log/px-deploy/<part>`, and records its start and stop times in `/var/log/px-deploy/script_tracking`.

**Master of cluster _c_** (`master-<c>-1`):

1. `infra/all-common`: base packages, SSH keys and config, `/etc/hosts`, host name
2. `infra/<platform>-common`
3. `infra/all-master`: tools (`helm`, `etcdctl`, `k9s`), NFS export, cloud credentials into place
4. `infra/<platform>-master`: the control plane (or, for `ocp4`, `eks`, `aks`, `gke` and `rancher`, the platform's own installer)
5. the template's `scripts`, in order
6. the template's `cluster[c].scripts`, in order
7. append `master-<c> <public IP>` to `/var/log/px-deploy/completed/tracking`

**Worker _n_ of cluster _c_** (`node-<c>-<n>`):

1. `infra/all-common`, `infra/<platform>-common`, `infra/<platform>-node`
2. over SSH, append `node-<c>-<n> <public IP>` to its master's tracking file, and copy its own `script_tracking` there

A part that fails does not stop the parts after it. Only the final line of the master's script marks it Ready, so a failed `install-px` still ends in `Ready`; read the per-script logs to tell.

### 7.2 How the scripts reach the VMs

Terraform starts each VM with a small cloud-init (or GCP startup script). It waits until two files appear in `/tmp`, sources the first, then runs the second:

- `/tmp/env.sh`: the settings (section 8)
- `/tmp/<vm>_scripts.sh`: the VM's script from section 7.1

Terraform's `file` provisioners upload both over SSH after the VM boots, along with `~/.px-deploy/assets/` to `/assets`. Cloud-init also sets `cloud`, `cluster` and a few cloud-specific variables (on AWS, the VPC, subnet and security group IDs).

### 7.3 Names and addresses

- On AWS, Azure and GCP, cluster _c_ gets the subnet `192.168.<100+c>.0/24`. Its master is `.90`, worker _n_ is `.<100+n>`. `all-common` writes these into `/etc/hosts` as `master-<c>` and `node-<c>-<n>`, so every VM can reach every other by name.
- On vSphere, addresses come from DHCP. `all-common` polls vCenter with `govc` until every VM of the deployment is powered on with an address, then writes `/etc/hosts` from that.
- From your machine, px-deploy reaches a VM by asking the cloud API for its public address (or, on vSphere, its guest address): `aws_get_node_ip`, `azure_get_node_ip`, `gcp_get_node_ip`, `vsphere_get_node_ip`.

### 7.4 Who reaches whom

- `connect`, `kubeconfig` and log collection on `destroy` go to the first master of cluster 1, and from there over SSH to `master-<c>` by name. That first master is the deployment's bastion.
- `status` connects to each cluster's first master directly.
- Every VM holds the deployment's private key (`/root/.ssh/id_rsa`) and trusts it, so any VM can SSH to any other as root.
- `ssh_pub_key` from your settings is added to root's `authorized_keys` on every VM.

## 8. Variables and credentials on the VMs

`env.sh` exports:

- every string setting of the merged settings, lower-cased (`cloud`, `platform`, `clusters`, `nodes`, `k8s_version`, `px_version`, `aws_region`, ...);
- every `env` entry under its own name;
- `pxd_uuid`.

Each VM's script also defines `clusternodes`, an array of node counts per cluster, and cloud-init adds `cluster`.

"Every string setting" includes credentials: `aws_access_key_id` and `aws_secret_access_key`, `azure_client_secret`, `vsphere_password`, `ocp4_pull_secret`. `env.sh` stays in `/tmp` on every VM and in `tf-deployments/<name>/`. On AWS the access key is also written to `/root/.aws/credentials`, and on GCP the service-account key to `/root/gcp.json`, so that scripts can call the cloud's API. Treat every lab VM, and anyone who can log in to one, as holding your cloud credentials.

## 9. Storage model

Portworx gets its disks one of two ways.

**Attached disks.** `aws_ebs`, `azure_disks` and `gcp_disks` are Terraform resources: created with the workers, attached to them, and removed by `terraform destroy`.

**Cloud drives.** With `cloud_drive` set (or always, on vSphere), `scripts/install-px` asks the Portworx spec generator for a spec in which Portworx creates its own disks through the cloud's API. The spec tags each disk `pxd_uuid_cd:<pxd_uuid>`. Terraform never knows about these disks, so `destroy` finds them by tag (AWS, GCP) or by the VMs they hang off (vSphere) and deletes them itself, before or after `terraform destroy` as each cloud requires. On vSphere, `install-px` also stores the vCenter user and password in a Kubernetes Secret so Portworx can create the disks.

OpenShift 4 and the managed platforms build their own workers, so attached disks are not possible there and cloud drives are required.

`install-px` builds the Portworx spec URL from the settings (`px_version`, the cluster's Kubernetes version, `journal`, `security`, `px_store`, an external `etcd`, and a free-form `px_suffix`) and applies the result through the Portworx operator.

## 10. Lifecycle of `destroy`

1. Refuse if `tf-deployments/<name>/<name>.lock` exists.
2. Copy each master's `/var/log/px-deploy/` to `logs/<name>_<time>/<cluster>/`, via the bastion; keep the newest 10 log folders.
3. Run the **pre-delete hooks** on every master: each `.sh` in `/px-deploy/script-delete/`, and for `ocp4` also `/px-deploy/platform-delete/` (which removes the OpenShift cluster, about 15 minutes each). A template script registers cleanup by writing a file there; the `wetty` script, for example, drops one that deletes its DNS records. A failed `script` hook is a warning; a failed `platform` hook stops the destroy unless `--force` is given.
4. Cloud-specific work Terraform can't do: on AWS, delete EKS node groups, terminate instances and collect the Portworx cloud drives; on GCP, stop instances with cloud drives, delete GKE node pools, then detach and delete the drives; on vSphere, power off, detach and delete the cloud-drive files.
5. `terraform plan -destroy` and apply it. The plan skips refresh to save time, except on vSphere, where it must refresh. If the deployment was created by an older release whose Terraform modules are gone from the image, px-deploy runs `terraform init` and tries again.
6. On AWS, delete the collected cloud drives.
7. Remove `tf-deployments/<name>/`, `deployments/<name>.yml`, the key pair and the kubeconfigs.

`destroy --clear` does step 7 only. `destroy -a` runs the full sequence for every deployment that isn't locked.

## 11. Cloud adapters

A cloud is spread across a fixed set of places. Adding one means adding a branch to each:

| Concern | Where |
|---|---|
| Settings and their validation | `Config` struct and `validate_config` in `px-deploy.go`; keys in `defaults.yml` |
| Terraform | `terraform/<cloud>/` (`main.tf`, `variables.tf`, a cloud-init template); `terraform init` in the `Dockerfile` |
| Rendering variables | `<cloud>_create_variables` in `<cloud>.go`; a case in `create_deployment` |
| Values Terraform returns | a case in `run_terraform_apply` that appends them to `deployments/<name>.yml` |
| Finding a VM's address | `<cloud>_get_node_ip`, used by `get_ip`, `get_deployment_status` and `exec_predelete` |
| Master naming for pre-delete | a case in `prepare_predelete` |
| Teardown | a branch in `destroy_deployment` (cloud-drive cleanup) |
| `/etc/hosts` on the VMs | a branch in `infra/all-common` |
| `list` region column | a case in the `list` command |
| Optional | `--region` support in `prepare_deployment`; `stop` and `start` |

Current support:

| | AWS | Azure | GCP | vSphere |
|---|---|---|---|---|
| `k8s`, `none` | Yes | Yes | Yes | Yes |
| `ocp4`, `rancher` | Yes | | | |
| Managed Kubernetes | `eks` | `aks` | `gke` | |
| `--region` | Yes | Yes | Yes | |
| `stop` / `start` | Yes (not `ocp4`, `eks`) | | | |
| `--dry_run` | Yes | Yes | | |
| Cloud-drive cleanup on `destroy` | Yes | | Yes | Yes |
| VM addressing | static, per-cluster subnet | static | static | DHCP, read from vCenter |

## 12. Cost control

- **Locks.** `--lock` (or `lock: true`) creates the lock file; `destroy` and `destroy -a` skip a locked deployment until `unlock`.
- **The cron reminder.** `px-deploy.cron` runs `px-deploy_cron.sh` at 08:00 and 16:00. It counts AWS deployments in `deployments/`, estimates a daily cost from the number of VMs, and shows a dialog (macOS or WSL). It never stops or destroys anything.
- **`stop`.** On AWS, stopping instances keeps their disks and stops compute charges.

## 13. Failure modes and where to look

| Symptom | Likely cause | Look at |
|---|---|---|
| `create` stops at "Please update to latest release" | The image is older than `master`'s `VERSION` | Re-run the installer, or `--ignore_version` |
| "Found N errors in config" | Validation | The listed settings in `defaults.yml`, the template or flags |
| "terraform plan failed" or "apply failed" | Credentials, quota, marketplace terms, region | Terraform's own output above the message |
| `status`: "Error get status of cluster" | The master isn't up or SSH isn't ready yet | Wait; then the cloud console |
| `status` stays `NotReady` for a node | The node's script hasn't finished, or it can't SSH to its master | `/var/log/px-deploy/` on the node; `script_tracking_node-*` on the master |
| `Ready`, but Portworx or the app is missing | A script part failed; parts don't stop the chain | `/var/log/px-deploy/<script>` on the master |
| `destroy` stops on a pre-delete failure | A `platform` hook failed (OpenShift) | Make sure the master is on and reachable, then retry or use `--force` |
| `destroy` cannot find the deployment's state | `tf-deployments/<name>/` is gone | `destroy --clear`, then remove cloud resources by hand |
| Disks left behind after `destroy` | Cloud drives on a cloud without cleanup (Azure), or a failed cleanup | Search the cloud for the tag `pxd_uuid_cd:<pxd_uuid>` |
