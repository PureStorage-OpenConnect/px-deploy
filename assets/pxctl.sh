NAMESPACE=$(kubectl get stc -A -o jsonpath='{.items[].metadata.namespace}')
[ -z $NAMESPACE ] && echo "Cannot find StorageCluster in any namespace." && exit 1
POD=$(kubectl get pods -n $NAMESPACE --field-selector=status.phase==Running -l name=portworx -o wide -o jsonpath='{.items[0].metadata.name}')
[ -z $POD ] && echo "Cannot find a running oci-monitor pod in namespace $NAMESPACE." && exit 1
ADMIN_TOKEN=$(kubectl -n $NAMESPACE get secret px-admin-token -o jsonpath='{.data.auth-token}' 2>/dev/null | base64 -d)
[ "$ADMIN_TOKEN" ] && kubectl -n $NAMESPACE exec -ti $POD -c portworx -- /opt/pwx/bin/pxctl context create admin --token=$ADMIN_TOKEN
kubectl -n $NAMESPACE exec -ti $POD -c portworx -- bash -c "/opt/pwx/bin/pxctl $* || true"
