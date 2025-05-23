<!-- If you update this, you probably also want to update the Async-DR and Metro documents -->
# PX-Migrate

Deploys 2 clusters with Portworx, sets up and configures a ClusterPair

# Supported Environments

* AWS

No other enviroments are currently supported.

# Requirements

## Create a bucket for use by PX-Migrate

PX-Migrate requires an S3 bucket. This can be specified in `defaults.yml`:

```
env:
  DR_BUCKET: "<YOUR BUCKET NAME>"
```

If the `$DR_BUCKET` bucket does not exist, it will be automatically created in us-east-1. If you create it manually, it can be in any region.

## Deploy the template

It is a best practice to use your initials or name as part of the name of the deployment in order to make it easier for others to see the ownership of the deployment in the AWS console.

```
px-deploy create -t migration -n <my-deployment-name>
```

# Demo Workflow

1. Obtain the external IPs for each cluster:

```
px-deploy status -n <my-deployment-name>
```

2. Open a browser tab for each and go to http://<ip1:30333> and http://<ip2:30333> (don't worry, they will not work at this stage).

3. Connect to the deployment in two terminals, and in the second one connect to the second master:

```
ssh master-2
```

4. In each cluster, show they are independent clusters:

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

`storkctl get clusterpair` gives us a human-readable output of status of the ClusterPair. Talk about how this means that Kubernetes cluster 1 can authenticate with Kubernetes cluster 2, and Portworx cluster 1 can authenticate with Portworx cluster 2, and that with both of these things in place we are able to migrate both objects and volumes from cluster 1 to cluster 2. This means that we can migrate not just an application or its data, but both at the same time. Furthermore, we can migrate an entire namespace or list of namespaces, so we can migrate an entire application stack. `kubectl describe clusterpair` will give us additional debugging information if the pairing were to be unsuccessful.

6. Show the Migration YAML that is to be applied:

```
cat /assets/app-migration.yml
```

Mention that the Migration is in the `kube-system` namespace which means we can use it to migrate any namespace. If we were to create it in any other namespace, we would only be able to use it to migrate that namespace.

In the Migration, note two main parameters:

* `clusterPair` - a reference to the ClusterPair object we just saw - defines **where** we are migrating
* `namespaces` - an array of namespaces to be migrated - defines **what** we are migrating

7. In each cluster, show there are no apps running yet:

```
kubectl get ns
```

8. In cluster 1, provision the Petclinic app:

```
kubectl apply -f /assets/petclinic/petclinic.yml
```

Talk about how it is a stateless Java app backed by a MySQL data, which itself is backed by a Portworx volume:

```
kubectl get pvc,deploy -n petclinic
```

Wait for it to be ready (it takes a minute or so):

```
kubectl get pod -n petclinic
```

9. Refresh the first tab in your browser. Click Find Owners, Add Owner and populate the form with some dummy data and then click Add Owner. Click Find Owners and Find Owner, and show that there is the entry at the bottom of the list.

10. In cluster 1, apply the Migration object:

```
kubectl apply -f /assets/app-migration.yml
```

11. Show the Migration object:

```
kubectl get migrations -n kube-system
storkctl get migrations -n kube-system
```

Do not continue until the Migration has succeeded.

12. On cluster 2, show that the namespace and its contents have been migrated:

```
kubectl get all,pvc -n petclinic
```

13. On cluster 2, show the pods starting:

```
kubectl get pod -n petclinic
```

It will take another minute or so to start.

14. Refresh the browser tab for the second cluster. Click Find Owners and Find Owner and show the data is still there.
