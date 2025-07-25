files=$(grep -l "cloud: aws" $HOME/.px-deploy/deployments/*.yml)
deployments=$(wc -w <<<$files | tr -d " ")
[ $deployments = 0 ] && exit 0
eks=$(grep "platform: eks" $files | wc -l | tr -d " ")
ocp4=$(grep "platform: ocp4" $files | wc -l | tr -d " ")
clusters=$(awk -F \" '/clusters:/ {sum+=$2} END {print sum}' $files)
nodes=$(awk -F \" '/nodes:/ {sum+=$2} END {print sum}' $files)
nodes=$[$nodes+$clusters]
cost=$(bc <<< "scale=2; $nodes*1.92")
msg="PX-Deploy has $deployments deployments running:
	$clusters clusters
	$nodes nodes

This is costing a minimum of approximately:
	\$$cost per day"
if [ $eks != 0 -o $ocp4 != 0 ]; then
  msg2="\n\nYou are also running
	$eks EKS deployments
	$ocp4 OCP4 deployments
  which could be costing significantly more"
fi
osascript -e "display dialog \"$msg$msg2\" buttons {\"Understood\"} default button \"Understood\" with icon caution" >/dev/null
