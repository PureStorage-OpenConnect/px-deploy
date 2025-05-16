## How to deploy

* ensure your px-deploy environment is able to create OCP4 deployments 
 
* please consult the [readme](https://github.com/purestorage-openconnect/px-deploy?tab=readme-ov-file#notes-for-ocp4-on-aws)

```
$ px-deploy create -n mydeployment -t ocp-kubevirt
```

* ensure your ~/.px-deploy/defaults.yml contains env variables with valid DR license and defining existing & accessible S3 Buckets 

```
env:
  DR_BUCKET: "mys3bucket"
  BACKUP_BUCKET: "mys3bucket"
  licenses: "xxx-xxxx-xxx"
```

## Prepare

* open 2 cli consoles

* login to deployment on both consoles `$ px-deploy connect -n mydeployment`

* on console 1 stay on master-1 (**c1cli**), on console 2 ssh into master-2 (**c2cli**)

* both consoles will show login credentials to OCP, **c1cli** will also show PX-Backup credentials

* login to PX-Backup UI (**pxbui**) & both OCP4 UI on Browser (**c1ui** **c2ui**)

### Check

* **c1ui**: on namespace pxbbq ubuntu-mongodb VM and 3 pxbbq pods are running

* get route to app, open app, login to app and place an order

* **c2ui**: show pxbbq namespace does not exist


* **c1cli**: 

* show and talk about ClusterPair, SchedulePolicy and MigrationSchedule 

* `$ storkctl get clusterpair -n kube-system`

* `$ vi /assets/kubevirt/ocp/async-dr.yml`

* apply SchedulePolicy & MigrationSchedule 

* `$ kubectl apply -f /assets/kubevirt/ocp/async-dr.yml`

* wait for first migration to finish
 
* `$ storkctl get migrations -n kube-system`

* **c2cli**: run a controlled failover of application `$ storkctl perform failover -m pxbbq -n kube-system`

* **c2cli**: watch the failover progressing `storkctl get failover failover-pxbbq-xxxxxxxxxxxx -n kube-system`

* **c2ui**: show pods / vm in pxbbq namespace coming up

* **c1cli**: DELETE PXBBQ namespace as this takes some time

* **c2ui**: show missing route, talk about not having route migrated

* create route to pxbbq app (service pxbbq-svc)

* access app from cluster to via route

* login to app, show existing order, add another one

* **pxbui**: login to px-backup gui

* check if cloud, object target are set

* onboard cluster-2

* take a backup from pxbbq namespace on cluster-2. can use namespace label app=pxbbq

* when backup is taken, restore to cluster-1. exclude route

* while restore running, switch to cluster1 ui to show everything is empty
