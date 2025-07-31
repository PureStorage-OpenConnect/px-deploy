# Uncomment to use a bastion - make sure you can ssh with no keys or errors
# BASTION="username@host.name"

# Set $dir to where we are looking, create a temp directory if using a bastion
dir=$HOME/.px-deploy/deployments
if [ "$BASTION" ]; then
  dir=$HOME/.px-deploy/tmp/deployments
  mkdir -p $dir
  scp -q $BASTION:.px-deploy/deployments/* $dir/
fi

# Get the metadata
files=$(grep -l "cloud: aws" $dir/*.yml 2>/dev/null)
deployments=$(wc -w <<<$files | tr -d " ")
[ $deployments = 0 ] && exit 0
eks=$(grep "platform: eks" $files | wc -l | tr -d " ")
ocp4=$(grep "platform: ocp4" $files | wc -l | tr -d " ")
clusters=$(awk -F \" '/clusters:/ {sum+=$2} END {print sum}' $files)
nodes=$(awk -F \" '/nodes:/ {sum+=$2} END {print sum}' $files)
nodes=$[$nodes+$clusters]
cost=$(bc <<< "scale=2; $nodes*1.92")

# Generate a message
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

if [ -x /usr/bin/osascript ]; then
  osascript -e "display dialog \"$msg$msg2\" buttons {\"Understood\"} default button \"Understood\" with icon caution" >/dev/null
elif [ -x /mnt/c/Windows/system32/msg.exe ]; then
  /mnt/c/Windows/system32/msg.exe \* "$msg$msg2" 2>/dev/null
else
  echo Cannot find /usr/bin/osascript or /mnt/c/Windows/system32/msg.exe >&2
fi

# Clean up
rm -rf $HOME/.px-deploy/tmp/deployments
