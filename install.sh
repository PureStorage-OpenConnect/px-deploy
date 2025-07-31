RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[1;34m'
WHITE='\033[0;37m'
NC='\033[0m'
ver=$1

if [ $USER != root -a -d $HOME/.px-deploy ]; then
  if [ $(find $HOME/.px-deploy -uid 0 | wc -l) != 0 ]; then
    echo -e "${RED}Found root-owned files in $HOME/.px-deploy - run this command before rerunning install.sh:$NC"
    echo "sudo chown -R $USER $HOME/.px-deploy"
    exit 1
  fi
fi

rm -rf /tmp/px-deploy.build
mkdir /tmp/px-deploy.build
cd /tmp/px-deploy.build
echo Cloning repo
git clone https://github.com/PureStorage-OpenConnect/px-deploy
cd px-deploy
if [ -z "$ver" ]; then
  ver=$(cat VERSION)
  git checkout v$ver
fi
echo "Pulling image (version $ver)"
docker pull ghcr.io/purestorage-openconnect/px-deploy:$ver
docker tag ghcr.io/purestorage-openconnect/px-deploy:$ver px-deploy

#echo Building container
#docker build $PLATFORM --network host -t px-deploy . >&/dev/null
#if [ $? -ne 0 ]; then
#  echo -e ${RED}Image build failed${NC}
#  exit
#fi
mkdir -p $HOME/.px-deploy/{keys,deployments,kubeconfig,tf-deployments,docs,logs}

# backup existing directories and force copy from current branch
time=$(date +%s)
for i in infra scripts templates assets docs; do
  [ -e $HOME/.px-deploy/$i ] && echo Backing up $HOME/.px-deploy/$i to $HOME/.px-deploy/$i.$time && cp -r $HOME/.px-deploy/$i $HOME/.px-deploy/$i.$time
  cp -rf $i $HOME/.px-deploy
done

# existing defaults.yml found. Dont replace, but ask for updating versions
if [ -e $HOME/.px-deploy/defaults.yml ]; then
  echo -e "${YELLOW}Existing defaults.yml found. Please consider updating k8s_version and px_version to release settings (check $HOME/px-deploy/versions.yml)."
else
  cp defaults.yml $HOME/.px-deploy/defaults.yml
fi
cp defaults.yml $HOME/.px-deploy/versions.yml

echo
echo -e ${YELLOW}If you are using zsh, append this to your .zshrc:
echo -e ${WHITE}'px-deploy() { [ "$DEFAULTS" ] && params="-v $DEFAULTS:/px-deploy/.px-deploy/defaults.yml" ; docker run --network host -it -e PXDUSER=$USER --rm --name px-deploy.$$ $=params -v $HOME/.px-deploy:/px-deploy/.px-deploy px-deploy /root/go/bin/px-deploy $* ; }'
echo -e ${YELLOW}If you are using bash, append this to your .bash_profile:
echo -e ${WHITE}'px-deploy() { [ "$DEFAULTS" ] && params="-v $DEFAULTS:/px-deploy/.px-deploy/defaults.yml" ; docker run --network host -it -e PXDUSER=$USER --rm --name px-deploy.$$ $params -v $HOME/.px-deploy:/px-deploy/.px-deploy px-deploy /root/go/bin/px-deploy "$@" ; }'
echo
#echo -e ${YELLOW}If using bash completion, execute:
#echo -e ${WHITE}'px-deploy completion | tr -d "\\r" >$HOME/.px-deploy/bash-completion'
#echo -e ${YELLOW}and append this to your .bash_profile:
#echo -e "${WHITE}[ -n \$BASH_COMPLETION ] && . \$HOME/.px-deploy/bash-completion"
if echo $OSTYPE | grep -q darwin; then
  cp px-deploy.cron px-deploy_cron.sh $HOME/.px-deploy
  echo -e ${YELLOW}To enable cost reminders twice a day, execute:
  echo -e ${WHITE}crontab \$HOME/.px-deploy/px-deploy.cron$NC
fi
