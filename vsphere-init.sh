#!/bin/bash

url=$vsphere_user:$vsphere_password@$vsphere_host
dc=$(govc find -k -u $url / -type c -name "$vsphere_compute_resource" | cut -f 2 -d /)
for i in $(govc find -k -u $url / -type m -runtime.powerState poweredOff -dc $dc | grep -v " "); do
  if [ "$(govc vm.info -k -u $url -json $i | jq -r '.VirtualMachines[0].Config.ExtraConfig[] | select(.Key==("pxd.deployment")).Value' 2>/dev/null)" = TEMPLATE ] ; then
    echo Found template $i - please use this one, or delete it and retry.
    exit 1
  fi
done

echo This will take a few minutes...
cat <<EOF >/vsphere-centos.json
{
  "variables": {
    "vsphere-server": "$vsphere_host",
    "vsphere-user": "$vsphere_user",
    "vsphere-password": "$vsphere_password",
    "vsphere-cluster": "$vsphere_compute_resource",
    "vsphere-resource-pool": "$vsphere_resource_pool",
    "vsphere-network": "$vsphere_network",
    "vsphere-datastore": "$vsphere_datastore",
    "vsphere-folder": "$vsphere_template_dir",
    "vm-name": "$vsphere_template_base",
    "vm-cpu-num": "4",
    "vm-mem-size": "8192",
    "vm-disk-size": "52000",
    "iso_url": "https://vault.centos.org/7.8.2003/isos/x86_64/CentOS-7-x86_64-Minimal-2003.iso",
    "kickstart_file": "/vsphere-ks.cfg"
  },
  "builders": [
    {
      "CPUs": "{{user \`vm-cpu-num\`}}",
      "RAM": "{{user \`vm-mem-size\`}}",
      "RAM_reserve_all": false,
      "boot_command": [
        "<tab> text ks=hd:fd0:/{{user \`kickstart_file\`}}<enter><wait>"
      ],
      "boot_order": "disk,cdrom,floppy",
      "boot_wait": "10s",
      "cluster": "{{user \`vsphere-cluster\`}}",
      "resource_pool": "{{user \`vsphere-resource-pool\`}}",
      "convert_to_template": true,
      "datastore": "{{user \`vsphere-datastore\`}}",
      "disk_controller_type": "pvscsi",
      "floppy_files": [
        "{{user \`kickstart_file\`}}"
      ],
      "folder": "{{user \`vsphere-folder\`}}",
      "guest_os_type": "centos7_64Guest",
      "insecure_connection": "true",
      "iso_url": "{{user \`iso_url\`}}",
      "iso_checksum": "md5:f99e2b01389c62a56bb0d3afdbc202f2",
      "network_adapters": [
        {
          "network": "{{user \`vsphere-network\`}}",
          "network_card": "vmxnet3"
        }
      ],
      "notes": "Build via Packer",
      "password": "{{user \`vsphere-password\`}}",
      "storage": [
        {
          "disk_size": "{{user \`vm-disk-size\`}}",
          "disk_thin_provisioned": true
        }
      ],
      "type": "vsphere-iso",
      "username": "{{user \`vsphere-user\`}}",
      "vcenter_server": "{{user \`vsphere-server\`}}",
      "vm_name": "{{user \`vm-name\`}}",
      "ssh_username": "root",
      "ssh_password": "portworx",
      "configuration_parameters": {
          "guestinfo.metadata": "---",
          "guestinfo.metadata.encoding": "---",
          "guestinfo.userdata": "---",
          "guestinfo.userdata.encoding": "---",
          "pxd.deployment": "TEMPLATE",
          "pxd.hostname": "---"
      }
    }
  ],
  "provisioners": [
    {
      "inline": [
        "sudo yum upgrade -y",
        "sudo yum install -y cloud-init",
        "sudo yum install -y epel-release",
        "sudo yum install -y python-pip",
        "sudo curl -sSL https://raw.githubusercontent.com/vmware/cloud-init-vmware-guestinfo/master/install.sh | sudo sh -"
      ],
      "type": "shell"
    }
  ]
}
EOF

cat <<\EOF >/vsphere-ks.cfg
auth --enableshadow --passalgo=sha512
cdrom
text
firstboot --enable
ignoredisk --only-use=sda
keyboard --vckeymap=us --xlayouts='us'
lang en_US.UTF-8
network  --bootproto=dhcp --device=ens192 --onboot=yes --noipv6
network  --hostname=localhost.localdomain
rootpw --iscrypted $6$oHCngZUb/uEBImIf$Og9pS/av0PCXBOd2saohkK0P8yFl72QG4ei3467bIaGFFfNxyoTW8KZevE6AhkXrDMgvbeOSchAS5c.NNaWLJ0
services --disabled="chronyd"
timezone UTC --isUtc --nontp
bootloader --append=" crashkernel=auto" --location=mbr --boot-drive=sda
clearpart --none --initlabel
part /boot --fstype="xfs" --ondisk=sda --size=1024
part / --fstype="xfs" --ondisk=sda --size=50000

%packages
@^minimal
@core
kexec-tools

%end

%addon com_redhat_kdump --enable --reserve-mb='auto'

%end

%anaconda
pwpolicy root --minlen=6 --minquality=1 --notstrict --nochanges --notempty
pwpolicy user --minlen=6 --minquality=1 --notstrict --nochanges --emptyok
pwpolicy luks --minlen=6 --minquality=1 --notstrict --nochanges --notempty
%end

%post
yum -y install open-vm-tools
systemctl enable vmtoolsd
systemctl start vmtoolsd
yum -y install kernel-headers nfs-utils dnf jq bash-completion nfs-utils chrony docker vim-enhanced git
yum update -y glib2
%end

reboot
EOF

cd /
/usr/bin/packer build /vsphere-centos.json
