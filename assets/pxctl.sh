NAMESPACE=$(kubectl get stc -A -o jsonpath='{.items[].metadata.namespace}')
POD=$(kubectl get pods -n $NAMESPACE -l name=portworx -o wide | tail -1 | cut -f 1 -d " ")
ADMIN_TOKEN=$(kubectl -n $NAMESPACE get secret px-admin-token -o jsonpath='{.data.auth-token}' 2>/dev/null | base64 -d)
[ "$ADMIN_TOKEN" ] && kubectl -n $NAMESPACE exec -ti $POD -c portworx -- /opt/pwx/bin/pxctl context create admin --token=$ADMIN_TOKEN
kubectl -n $NAMESPACE exec -ti $POD -c portworx -- /opt/pwx/bin/pxctl $@
