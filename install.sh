docker run --help | grep -q -- "--platform string" && PLATFORM="--platform=linux/amd64"
docker run $PLATFORM --rm --privileged --network host -i -e home=$HOME -v /var/run/docker.sock:/var/run/docker.sock -v $HOME/.px-deploy:/.px-deploy rockylinux:8 <<\EOF
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[1;34m'
WHITE='\033[0;37m'
NC='\033[0m'

# find existing deployments not supported by pxd5 
found_legacy=false

# find deployments with awstf
for i in $(grep -l 'cloud: awstf' /.px-deploy/deployments/*.yml 2>/dev/null); do
        echo -e "${RED} AWSTF Deployment $(basename $i .yml) is being created by px-deploy version < 5. Please remove prior to upgrading to version 5"
        found_legacy=true
done

#find deployments being created by old aws code (no tf-deployments folder exists)
for i in $(grep -l 'cloud: aws' /.px-deploy/deployments/*.yml 2>/dev/null); do
    if [ ! -d /.px-deploy/tf-deployments/$(basename $i .yml) ]; then
        echo -e "${RED} AWS Deployment $(basename $i .yml) is being created by px-deploy version < 5. Please remove prior to upgrading to version 5"
        found_legacy=true
    fi
done
if [ "$found_legacy" = true ]; then
        echo -e "${RED}Old AWS deployment(s) found. Please destroy before updating"
        exit
fi


echo -e ${BLUE}Setting up installation container
dnf install -y dnf-plugins-core >&/dev/null
dnf config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo >&/dev/null
dnf install -y git docker-ce >&/dev/null
echo Cloning repo
git clone https://github.com/andrewh1978/px-deploy >&/dev/null
cd px-deploy
git checkout $(cat VERSION)
PXDVERSION=$(cat VERSION)
echo Building container
docker build $PLATFORM --network host -t px-deploy . >&/dev/null
mkdir -p /.px-deploy/{keys,deployments,kubeconfig,tf-deployments}

#remove remainders of terraform (outside container)
#*** can be removed after sept 2023***
rm -rf /.px-deploy/terraform*

# backup existing directories and force copy from current branch
time=$(date +%s)
for i in scripts templates assets; do
  [ -e /.px-deploy/$i ] && echo Backing up $home/.px-deploy/$i to $home/.px-deploy/$i.$time && cp -r /.px-deploy/$i /.px-deploy/$i.$time
  cp -rf $i /.px-deploy
done

# existing defaults.yml found. Dont replace, but ask for updating versions
if [ -e /.px-deploy/defaults.yml ]; then
  cp defaults.yml /.px-deploy/defaults.yml.$PXDVERSION
  echo -e ${YELLOW}Existing defaults.yml found. Please consider updating k8s/px Versions to release settings. Can be found in ./px-deploy/defaults.yml.$PXDVERSION
fi

echo
echo -e ${YELLOW}If you are using zsh, append this to your .zshrc:
echo -e ${WHITE}'px-deploy() { docker run --help | grep -q -- "--platform string" && PLATFORM="--platform=linux/amd64" ; [ "$DEFAULTS" ] && params="-v $DEFAULTS:/px-deploy/.px-deploy/defaults.yml" ; docker run $PLATFORM --network host -it -e PXDUSER=$USER --rm --name px-deploy.$$ $=params -v $HOME/.px-deploy:/px-deploy/.px-deploy -v $HOME/.config/gcloud:/root/.config/gcloud -v $HOME/.azure:/root/.azure px-deploy /root/go/bin/px-deploy $* ; }'
echo -e ${YELLOW}If you are using bash, append this to your .bash_profile:
echo -e ${WHITE}'px-deploy() { docker run --help | grep -q -- "--platform string" && PLATFORM="--platform linux/amd64" ; [ "$DEFAULTS" ] && params="-v $DEFAULTS:/px-deploy/.px-deploy/defaults.yml" ; docker run $PLATFORM --network host -it -e PXDUSER=$USER --rm --name px-deploy.$$ $params -v $HOME/.px-deploy:/px-deploy/.px-deploy -v $HOME/.config/gcloud:/root/.config/gcloud -v $HOME/.azure:/root/.azure px-deploy /root/go/bin/px-deploy $* ; }'
echo
echo -e ${GREEN}When your px-deploy function is set, create a deployment with:
echo -e "${WHITE}px-deploy create --name myDeployment --template px$NC"
echo
echo -e ${YELLOW}If using bash completion, execute:
echo -e ${WHITE}'px-deploy completion | tr -d "\\r" >$HOME/.px-deploy/bash-completion'
echo -e ${YELLOW}and append this to your .bash_profile:
echo -e "${WHITE}[ -n \$BASH_COMPLETION ] && . \$HOME/.px-deploy/bash-completion"
EOF
