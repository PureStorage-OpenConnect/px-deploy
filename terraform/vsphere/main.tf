terraform {
  required_providers {
    vsphere = {
      source  = "hashicorp/vsphere"
      version = "2.6.1"
    }
    local = {
      source = "hashicorp/local"
    }
    null = {
      source = "hashicorp/null"
    }
    tls = {
      source = "hashicorp/tls"
    }
  }
}

provider "vsphere" {
	user = var.vsphere_user
	password = var.vsphere_password
	vsphere_server = var.vsphere_host
	allow_unverified_ssl = true
}

data "vsphere_datacenter" "dc" {
	name			= var.vsphere_datacenter
}

data "vsphere_compute_cluster" "cluster" {
	name			= var.vsphere_compute_resource
	datacenter_id 	= data.vsphere_datacenter.dc.id
}

data "vsphere_datastore" "datastore" {
	name			= var.vsphere_datastore
	datacenter_id 	= data.vsphere_datacenter.dc.id
}

data "vsphere_resource_pool" "pool" {
	name			= var.vsphere_resource_pool
	datacenter_id 	= data.vsphere_datacenter.dc.id
}

data "vsphere_network" "network" {
	name          	= var.vsphere_network
	datacenter_id 	= data.vsphere_datacenter.dc.id
}

data "vsphere_virtual_machine" "template" {
  name          = var.vsphere_template
  datacenter_id = data.vsphere_datacenter.dc.id
}

resource "tls_private_key" "ssh" {
	algorithm = "RSA" 
	rsa_bits  = 2048
}

resource "local_file" "ssh_private_key" {
	content = tls_private_key.ssh.private_key_openssh
	file_permission = "0600"
	filename = format("/px-deploy/.px-deploy/keys/id_rsa.vsphere.%s",var.config_name)
}

resource "local_file" "ssh_public_key" {
	content = tls_private_key.ssh.public_key_openssh
	file_permission = "0644"
	filename = format("/px-deploy/.px-deploy/keys/id_rsa.vsphere.%s.pub",var.config_name)
}

locals {
  nodeconfig = [
    for vm in var.nodeconfig : [
      for i in range(1, vm.nodecount+1) : {
        instance_name 	= "${vm.role}-${vm.cluster}-${i}"
		    nodenum			= i
    		cluster 		= vm.cluster
        role        = vm.role
      }
    ]
  ]
  ip_address = split(".",element(split("/",var.vsphere_ip),0))
}

locals {
  instances = flatten(local.nodeconfig)
}

resource "vsphere_virtual_machine" "node" {
  for_each  	   =	{for key,value in local.instances: key => value}
  
  // to maintain compatibility to scripts master nodes must be named with master-[clusternum]; nodes have node-[clusternum]-[nodenum]
  name = each.value.role == "master" ? "${var.config_name}-master-${each.value.cluster}" : "${var.config_name}-${each.value.instance_name}"
  //name             = "${var.config_name}-${each.value.instance_name}"
  enable_disk_uuid = true
  ept_rvi_mode     = "automatic"
  hv_mode          = "hvAuto"
  resource_pool_id = data.vsphere_resource_pool.pool.id
  datastore_id     = data.vsphere_datastore.datastore.id
  folder           = var.vsphere_folder
  num_cpus         = var.vsphere_cpu    
  memory           = var.vsphere_memory * 1024
  guest_id         = "rhel8_64Guest"
  network_interface {
    network_id = data.vsphere_network.network.id
  }
  clone {
    template_uuid = data.vsphere_virtual_machine.template.id
  }
  lifecycle {
    ignore_changes = [disk,ept_rvi_mode,hv_mode,]
  }
  
  extra_config = {
	  "guestinfo.userdata" = base64encode(local_file.cloud-init[each.key].content)
    "guestinfo.userdata.encoding" = "base64"

    "guestinfo.metadata" = length(local.ip_address) == 4 ? base64encode(local_file.metadata[each.key].content) : ""
    "guestinfo.metadata.encoding" = length(local.ip_address) == 4 ? "base64" : ""
    
    // to maintain compatibility to scripts master nodes must be named with master-[clusternum]; nodes have node-[clusternum]-[nodenum]
    "pxd.hostname" = each.value.role == "master" ? "master-${each.value.cluster}" : each.value.instance_name
    "pxd.deployment" = var.config_name
  }

  disk {
    label = "disk0"
    size  = 128
  }

  connection {
    type = "ssh"
    user = "rocky"
    host = "${self.default_ip_address}"
    private_key = tls_private_key.ssh.private_key_openssh
  }
		
	provisioner "remote-exec" {
    inline = [
      "sudo mkdir /assets",
      "sudo chown rocky.users /assets"
    ]
  }

	provisioner "file" {
    source = format("%s%s",path.module,"/env.sh")
    destination = "/tmp/env.sh"
  }

  provisioner "file" {
    source = format("%s/%s",path.module,each.value.instance_name)
    destination = format("%s%s%s","/tmp/",each.value.instance_name,"_scripts.sh")
  }
	
	provisioner "file" {
		source = "/px-deploy/.px-deploy/assets/"
		destination = "/assets"
  }
}

resource "local_file" "cloud-init" {
	for_each =	{for key, value in local.instances: key => value}
	content = templatefile("${path.module}/cloud-init.tpl", 
  {
	  tpl_priv_key = base64encode(tls_private_key.ssh.private_key_openssh),
    tpl_pub_key = tls_private_key.ssh.public_key_openssh,
	  tpl_name = each.value.instance_name,
	  tpl_cluster = each.value.cluster
	})
	filename = "${path.module}/cloud-init-${each.value.instance_name}-generated.yaml"
}

resource "local_file" "metadata" {
	for_each 					=	{for key,value in local.instances: key => value}
	content = length(local.ip_address) == 4 ? templatefile("${path.module}/metadata.tpl", 
  {
		tpl_name = each.value.instance_name,
    tpl_ip = format("%s.%s.%s.%s/%s", element(local.ip_address,0),element(local.ip_address,1),element(local.ip_address,2),each.key + element(local.ip_address,3), element(split("/",var.vsphere_ip),1) )
    tpl_dns = var.vsphere_dns
    tpl_gw = var.vsphere_gw
  }) : ""
	filename = "${path.module}/metadata-${each.value.instance_name}-generated.yaml"
}

resource "local_file" "nodemap" {
  content = "%{ for vm in vsphere_virtual_machine.node}${format("\"%s\": \"%s,%s\"\n",vm.name,vm.moid,vm.network_interface[0].mac_address)}%{endfor}"
  filename = "${path.module}/nodemap.txt"
}
