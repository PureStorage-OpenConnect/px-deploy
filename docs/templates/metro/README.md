<!-- If you update this, you probably also want to update the Migration and Async-DR documents -->
# Metro-DR

Deploys 2 clusters with Portworxs, Metro DR and an external etcd node running on master-1, sets up and configures a ClusterPair, configures a Metro DR schedule with a loadbalancer in front of the setup.

# Supported Environments

* AWS

No other enviroments are currently supported.

# Requirements

## Configure a DR licence

Metro-DR requires a DR licence (the trial licence does not include DR).

This can be specified in `defaults.yml`:

```
env:
  licenses: "XXXX-XXXX-XXXX-XXXX-XXXX-XXXX-XXXX-XXXX"
```

You will need to request a valid activation code if you do not have a DR licence.

## Deploy the template

It is a best practice to use your initials or name as part of the name of the deployment in order to make it easier for others to see the ownership of the deployment in the AWS console.

```
px-deploy create -t metro -n <my-deployment-name>
```

# Demo Workflow

1. Obtain the external IPs for each cluster:

```
px-deploy status -n <my-deployment-name>
```

2. Open a browser tab for each and go to http://<ip1:30333> and http://<ip2:30333> (the second will not work at this stage).

3. Connect to the deployment in two terminals, and in the second one connect to the second master:

```
ssh master-2
```

4. In each cluster, show they are independent Kubernetes clusters, but a single Portworx cluster:

```
kubectl get nodes
pxctl status
```

5. In cluster 1, show the ClusterPair object:

```
kubectl get clusterpair -n kube-system
storkctl get clusterpair -n kube-system
kubectl describe clusterpair -n kube-system
kubectl edit clusterpair -n kube-system
:set nowrap
```

`storkctl get clusterpair` gives us a human-readable output of status of the ClusterPair. Talk about how this means that Kubernetes cluster 1 can authenticate with Kubernetes cluster 2, and that there is a single Portworx cluster stretched across Kubernetes clusters, and that with both of these things in place we are able to migrate objects from cluster 1 to cluster 2, while volumes are synchronously replicated. This means that we can migrate not just an application or its data, but both at the same time. Furthermore, we can migrate an entire namespace or list of namespaces, so we can migrate an entire application stack. `kubectl describe clusterpair` will give us additional debugging information if the pairing were to be unsuccessful.

6. Show the SchedulePolicy and MigrationSchedule YAML:

```
cat /assets/metro-schedule.yml
```

Mention that the SchedulePolicy is globally-scoped, but the MigrationSchedule is in the `kube-system` namespace which means we can use it to migrate any namespace. If we were to create it in any other namespace, we would only be able to use it to migrate that namespace.

In the MigrationSchedule, note three main parameters:

* `clusterPair` - a reference to the ClusterPair object we just saw - defines **where** we are migrating
* `namespaces` - an array of namespaces to be migrated - defines **what** we are migrating
* `schedulePolicyName` - a reference to the SchedulePolicy - defines **when** we are migrating

Also mention the `startApplications` parameter - this will patch the application specs, eg Deployments, StatefulSets and operator-based applications, to prevent them from starting on the target cluster. However, they will be annotated with the original number of application replicas, as we shall see shortly.

7. In cluster 1, show that we have a SchedulePolicy and MigrationSchedule:

```
kubectl get schedulepolicy
kubectl get migrationschedule -n kube-system
storkctl get schedulepolicy
storkctl get migrationschedule -n kube-system
```

`storkctl get migrationschedule` gives us a more human-readable output.

8. In each cluster, show Petclinic is running in cluster 1 but not cluster 2:

```
kubectl get ns
```

9. Refresh the first tab in your browser. Click Find Owners, Add Owner and populate the form with some dummy data and then click Add Owner. Click Find Owners and Find Owner, and show that there is the entry at the bottom of the list.

10. Refer back to the MigrationSchedule and how creating it will trigger the creation of a Migration object every 2 minutes (in our case). Show the Migration objects:

```
kubectl get migrations -n kube-system
storkctl get migrations -n kube-system
```

Do not continue until at least one Migration has started and succeeded.

11. We will now failover the application to the second cluster. It is recommended that instead of failing cluster 1, we just suspend the MigrationSchedule to prevent any further migrations from taking place:

```
storkctl suspend migrationschedule dr-schedule -n kube-system
```

Check there are no migrations currently in progress, or wait until the last one has completed:

```
storkctl get migrations -n kube-system
```

12. On cluster 2, show that the namespace and its contents have been migrated:

```
kubectl get all,pvc -n petclinic
```

Note that the Deployments have been migrated, but they are scaled down to 0. Take a look at them:

```
kubectl edit deploy -n petclinic
```

Show that the `replicas` parameter has been set to `0` as part of the migration. Show that the original number of replicas has been saved in the `migrationReplicas` annotation.

13. On cluster 2, scale up the application:

```
storkctl activate migration -n petclinic
```

Talk about how `storkctl` is going to find all the apps, ie Deployments, StatefulSets and operator-based applications, look for those annotations and then scale everything up to where they originally were.

14. On cluster 2, show the pods starting:

```
kubectl get pod -n petclinic
```

It will take another minute or so to start.

15. Refresh the browser tab for the second cluster. Click Find Owners and Find Owner and show the data is still there.
