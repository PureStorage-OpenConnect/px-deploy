echo "Applying aws-eks-load-balancer-controller"
vpcid=$(aws ec2 describe-vpcs --filters "Name=tag:pxd_uuid,Values=$pxd_uuid" --query "Vpcs[].VpcId" --output text)
helm repo add eks https://aws.github.io/eks-charts
lbc_clustername=$name-$cluster
if [ "$platform" = ocp4 ]; then
  lbc_clustername=$(kubectl get infrastructure cluster -o jsonpath='{.status.infrastructureName}')
  oc adm policy add-scc-to-user hostnetwork-v2 -z aws-load-balancer-controller -n kube-system
  lbc_extra="--set hostNetwork=true --set controllerConfig.featureGates.ALBGatewayAPI=false --set controllerConfig.featureGates.NLBGatewayAPI=false"
fi
helm install aws-load-balancer-controller eks/aws-load-balancer-controller -n kube-system --version 3.2.1 \
  --set clusterName=$lbc_clustername \
  --set feature-gates=ServiceTypeLoadBalancerOnly=true \
  --set defaultLoadBalancerScheme=internet-facing \
  --set defaultTags.pxd_uuid=$pxd_uuid \
  --set controllerConfig.featureGates.ALBGatewayAPI=false \
  --set controllerConfig.featureGates.NLBGatewayAPI=false \
  --set replicaCount=1 \
  --set vpcId=$vpcid \
  --set region=$aws_region $lbc_extra
kubectl get mutatingwebhookconfiguration aws-load-balancer-webhook -o json | jq '.webhooks[].failurePolicy = "Ignore"' | kubectl apply -f - # Prevent service from blocking Service creation later
