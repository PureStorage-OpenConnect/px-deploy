if [ $UID -gt 1000 ]; then
  n=$(echo $USER | sed s/training//)
  [ $[2*$[$n/2]] -ne $n ] && n=$[$n+1]
  ADMIN_PW=$(kubectl --kubeconfig /tmp/kubeconfig.$n get secret pxcentral-keycloak-http -n central -o jsonpath="{.data.password}" | base64 --decode)
  PXB_URL=$(kubectl --kubeconfig /tmp/kubeconfig.$n get svc px-backup-ui -n central -o=jsonpath='{.status.loadBalancer.ingress[0].hostname}')
  echo Portworx Backup UI: http://$PXB_URL/
  echo Portworx Backup username: admin
  echo Portworx Backup password: $ADMIN_PW
fi
