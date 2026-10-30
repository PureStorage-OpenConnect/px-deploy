echo "Applying aws-eks-load-balancer-controller"
vpcid=$(aws ec2 describe-vpcs --filters "Name=tag:pxd_uuid,Values=$pxd_uuid" --query "Vpcs[].VpcId" --output text)
helm repo add eks https://aws.github.io/eks-charts
helm install aws-load-balancer-controller eks/aws-load-balancer-controller -n kube-system --version 3.2.1 --set clusterName=$name-$cluster --set feature-gates=ServiceTypeLoadBalancerOnly=true --set defaultLoadBalancerScheme=internet-facing --set defaultTags.pxd_uuid=$pxd_uuid --set replicaCount=1 --set vpcId=$vpcid --set region=$region
kubectl get mutatingwebhookconfiguration aws-load-balancer-webhook -o json | jq '.webhooks[].failurePolicy = "Ignore"' | kubectl apply -f - # Prevent service from blocking Service creation later
