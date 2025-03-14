#!/bin/bash

# install packer (tested 1.8.5) 
# https://developer.hashicorp.com/packer/tutorials/docker-get-started/get-started-install-cli

# install ovftool & include in $PATH (tested 4.6.2)
# https://developer.vmware.com/web/tool/4.6.0/ovf-tool
# hint: if install does not start install ncurses-compat
# or check this https://rguske.github.io/post/vmware-ovftool-installation-was-unsuccessful-on-ubuntu-20/
# run with --extract -> copy vmware-ovftool to /usr/lib and ln -s /usr/lib/vmware-ovftool/ovftool /usr/bin/ovftool

# install & configure awscli

S3_BUCKET=px-deploy
PXDTEMPLATEID=$(date '+%Y%m%d%H%M%S')
KEYPAIR="~/.ssh/id_rsa"

#check if ovftool is within path
if [ ! $(type -P ovftool) ]; then
  echo "ovftool missing"
  exit
fi

#check if packer is within path
if [ ! $(type -P packer) ]; then
  echo "packer missing"
  exit
fi

# check if Bucket is accessible
aws s3 ls s3://$S3_BUCKET
if [ $? != 0 ]; then
 echo "error accessing bucket $S3_BUCKET"
 exit;
fi

mkdir -p tmp

sed -e 's/:[^:\/\/]/=/g;s/ *=/=/g' ~/.px-deploy/defaults.yml | grep vsphere > ./tmp/env.sh
source ./tmp/env.sh
vsphere_template_base=$(basename $vsphere_template)
vsphere_template_dir=$(dirname $vsphere_template)

echo This will take a few minutes...
cat <<EOF >./tmp/vsphere-rocky.json
{
  "variables": {
    "vsphere-server": "$vsphere_host",
    "vsphere-user": "$vsphere_user",
    "vsphere-password": "$vsphere_password",
    "vsphere-cluster": "$vsphere_compute_resource",
    "vsphere-datacenter": "$vsphere_datacenter",
    "vsphere-resource-pool": "$vsphere_resource_pool",
    "vsphere-network": "$vsphere_network",
    "vsphere-datastore": "$vsphere_datastore",
    "vsphere-folder": "$vsphere_template_dir",
    "vm-name": "pxdeploy-template-build",
    "pxd-templateid": "$PXDTEMPLATEID",
    "vm-cpu-num": "4",
    "vm-mem-size": "8192",
    "vm-disk-size": "52000",
    "iso_url": "https://dl.rockylinux.org/vault/rocky/9.4/isos/x86_64/Rocky-9.4-x86_64-minimal.iso",
    "kickstart_file": "/vsphere-ks.cfg"
  },
  "builders": [
    {
      "CPUs": "{{user \`vm-cpu-num\`}}",
      "RAM": "{{user \`vm-mem-size\`}}",
      "RAM_reserve_all": false,
      "boot_command": [
        "<wait>",
        "<tab>",
        "linux inst.ks=hd:/dev/sr1:vsphere-ks.cfg",
        "<enter>"
      ],
      "boot_order": "disk,cdrom",
      "boot_wait": "10s",
      "cluster": "{{user \`vsphere-cluster\`}}",
      "configuration_parameters": {
          "guestinfo.metadata": "---",
          "guestinfo.metadata.encoding": "---",
          "guestinfo.userdata": "---",
          "guestinfo.userdata.encoding": "---",
          "pxd.deployment": "TEMPLATE",
          "pxd.hostname": "---",
          "pxd.templateid": "{{user \`pxd-templateid\`}}"
      },
      "export": {
       "force": "true",
       "options": ["extraconfig"]
      },
      "datastore": "{{user \`vsphere-datastore\`}}",
      "disk_controller_type": "pvscsi",
      "folder": "{{user \`vsphere-folder\`}}",
      "guest_os_type": "rhel8_64Guest",
      "vm_version": "14",
      "insecure_connection": "true",
      "iso_checksum": "sha256:ee3ac97fdffab58652421941599902012179c37535aece76824673105169c4a2",
      "iso_url": "{{user \`iso_url\`}}",
      "cd_files": ["./vsphere-ks.cfg"],
      "cd_label": "kickstart",
      "network_adapters": [
        {
          "network": "{{user \`vsphere-network\`}}",
          "network_card": "vmxnet3"
        }
      ],
      "notes": "https://github.com/purestorage-openconnect/px-deploy \n Template ID {{user \`pxd-templateid\`}}",
      "password": "{{user \`vsphere-password\`}}",
      "destroy": "true",
      "resource_pool": "{{user \`vsphere-resource-pool\`}}",
      "ssh_username": "root",
      "ssh_private_key_file": "$KEYPAIR",
      "storage": [
        {
          "disk_size": "{{user \`vm-disk-size\`}}",
          "disk_thin_provisioned": true
        }
      ],
      "type": "vsphere-iso",
      "username": "{{user \`vsphere-user\`}}",
      "vcenter_server": "{{user \`vsphere-server\`}}",
      "vm_name": "{{user \`vm-name\`}}"
    }
  ],
  "provisioners": [
    {
      "inline": [
        "sudo dnf install -y cloud-init"
      ],
      "type": "shell"
    }
  ]
}
EOF

cat <<EOF >./tmp/vsphere-ks.cfg
repo --name=BaseOS --baseurl=https://dl.rockylinux.org/vault/rocky/9.4/BaseOS/x86_64/os/
repo --name=AppStream --baseurl=https://dl.rockylinux.org/vault/rocky/9.4/AppStream/x86_64/os/
text
firstboot --enable
ignoredisk --only-use=sda
keyboard --vckeymap=us --xlayouts='us'
lang en_US.UTF-8
#network  --bootproto=dhcp --device=ens192 --onboot=true --noipv6
network  --bootproto=static --ip=10.220.113.220 --netmask=255.255.254.0 --gateway=10.220.112.1 --nameserver=10.220.255.252 --device=ens192 --onboot=true --noipv6
network  --hostname=localhost.localdomain
rootpw portworx
sshkey --username=root "$(cat ~/.ssh/id_rsa.pub)"
services --disabled="chronyd,avahi-daemon.service,bluetooth.service,rhnsd.service,rhsmcertd.service"
timezone UTC --utc
timesource --ntp-disable
clearpart --all --initlabel
part /boot/efi --fstype=vfat --fsoptions='defaults,umask=0027,fmask=0077,uid=0,gid=0' --size=600 --ondisk=/dev/sda
part /boot --fstype=xfs --fsoptions='nosuid,nodev' --size=1024 --ondisk=/dev/sda
part / --fstype="xfs" --ondisk=sda --size=50000
bootloader --append="rd.driver.blacklist=dm-multipath,crashkernel=1G-4G:192M,4G-64G:256M,64G-:512M" --location=mbr --boot-drive=sda

cdrom

%packages
@base
@core
dnf
kexec-tools
open-vm-tools
kernel-headers 
nfs-utils 
jq 
bash-completion 
nfs-utils 
chrony 
docker 
vim-enhanced 
git
glib2
python3-devel
python3-pip
%end

%post
systemctl enable vmtoolsd
systemctl start vmtoolsd
dnf -y install epel-release
dnf update -y
dnf clean all
%end

reboot --eject
EOF

cd tmp
echo $PXDTEMPLATEID > pxdid.txt

echo "1. running packer"
packer build -force ./vsphere-rocky.json
if [ $? != 0 ]; then
  echo "Packer build failed"
  exit
fi

echo "2. running ovftool"
ovftool --allowExtraConfig output-vsphere-iso/pxdeploy-template-build.ovf template.ova
if [ $? != 0 ]; then
  echo "ovftool build failed"
  exit
fi

echo "3. copy ova to s3"
aws s3 cp template.ova s3://$S3_BUCKET/templates/template_r94.ova
if [ $? != 0 ]; then
  echo "s3 template upload failed"
  exit
fi

#echo "4. copy pxdid.txt to s3"
#aws s3 cp pxdid.txt s3://$S3_BUCKET/templates/pxdid.txt
#if [ $? != 0 ]; then
#  echo "s3 pxdid.txt upload failed"
#  exit
#fi

cd ..
rm -rf tmp
