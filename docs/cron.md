# What

You can install a cron job to give you a reminder of what you have running in AWS and what it is costing. It will run on MacOS and Windows (WSL) and can get metadata from a bastion if you are running one.

# How

1. The cron job and scripts are located in `$HOME/.px-deploy`. If you are running a bastion, you will need to download the files from Github. If you are running PX-Deploy on your workstation, skip this step:
```
mkdir -p $HOME/.px-deploy
curl -o $HOME/.px-deploy/px-deploy.cron https://raw.githubusercontent.com/PureStorage-OpenConnect/px-deploy/refs/heads/master/px-deploy.cron
curl -o $HOME/.px-deploy/px-deploy_cron.sh https://raw.githubusercontent.com/PureStorage-OpenConnect/px-deploy/refs/heads/master/px-deploy_cron.sh
```

2. The script will get deployment metadata from `$HOME/.px-deploy/deployments`. If you are running a bastion, it needs to know how to obtain this metadata. If you are running PX-Deploy on your workstation, skip this step:
* Edit `$HOME/.px-deploy/px-deploy_cron.sh`
* Uncomment and set the `BASTION` environment variable in the form `user@hostname`

3. Optionally, customise the times at which the job will trigger by editing `$HOME/.px-deploy/px-deploy.cron`.

4. Load the cron job
```
crontab $HOME/.px-deploy/px-deploy.cron
```

At the configured times, if there are any deployments running in AWS, a dialog box will appear on your screen summarising the numbers and costs.
