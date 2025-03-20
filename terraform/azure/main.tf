terraform {
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "=4.13.0"
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

provider "azurerm" {
  features {
    resource_group {
      prevent_deletion_if_contains_resources = false
    }
  }
  client_id = var.azure_client_id
  client_secret = var.azure_client_secret
  tenant_id = var.azure_tenant_id
  subscription_id = var.azure_subscription_id
}

resource "azurerm_resource_group" "rg" {
  name     = format("%s.%s",var.name_prefix,var.config_name)
  location = var.azure_region
  tags                = var.azure_tags
}

resource "tls_private_key" "ssh" {
	algorithm = "RSA" 
	rsa_bits  = 2048
}

resource "local_file" "ssh_private_key" {
	content = tls_private_key.ssh.private_key_openssh
	file_permission = "0600"
	filename = format("/px-deploy/.px-deploy/keys/id_rsa.azure.%s",var.config_name)
}

resource "local_file" "ssh_public_key" {
	content = tls_private_key.ssh.public_key_openssh
	file_permission = "0644"
	filename = format("/px-deploy/.px-deploy/keys/id_rsa.azure.%s.pub",var.config_name)
}

resource "azurerm_ssh_public_key" "deploy_key" {
  name                = format("px-deploy.%s",var.config_name)
  resource_group_name = azurerm_resource_group.rg.name
  location            = azurerm_resource_group.rg.location
  public_key          = tls_private_key.ssh.public_key_openssh
  tags                = var.azure_tags
}

resource "azurerm_virtual_network" "vnet" {
  name                = format("%s-%s-%s",var.name_prefix,var.config_name,"vnet")
  resource_group_name = azurerm_resource_group.rg.name
  location            = azurerm_resource_group.rg.location
  address_space       = [var.azure_cidr_vnet]
  tags                = var.azure_tags
}

resource "azurerm_subnet" "subnet" {
  count				   = var.clusters
  name                 = format("%s-%s-subnet-%s",var.name_prefix,var.config_name, count.index + 1)
  resource_group_name  = azurerm_resource_group.rg.name
  virtual_network_name = azurerm_virtual_network.vnet.name
  address_prefixes     = ["192.168.${count.index + 101}.0/24"]
}

resource "azurerm_subnet_network_security_group_association" "sg_sn" {
  count				   = var.clusters
  subnet_id            = azurerm_subnet.subnet[count.index].id
  network_security_group_id = azurerm_network_security_group.sg_default.id
}

resource "azurerm_network_security_group" "sg_default" {
  name                = format("px-deploy-%s",var.config_name)
  resource_group_name  = azurerm_resource_group.rg.name
  location            = azurerm_resource_group.rg.location
  tags                = var.azure_tags
  security_rule {
    name                       = "ssh"
    priority                   = 100
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "22"
    source_address_prefix      = "*"
    destination_address_prefix = "*"
  }
  security_rule {
    name                       = "http"
    priority                   = 200
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "80"
    source_address_prefix      = "*"
    destination_address_prefix = "*"
  }
  security_rule {
    name                       = "https"
    priority                   = 300
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "443"
    source_address_prefix      = "*"
    destination_address_prefix = "*"
  }
  security_rule {
    name                       = "tcp_2382"
    priority                   = 400
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "2382"
    source_address_prefix      = "*"
    destination_address_prefix = "*"
  }
  security_rule {
    name                       = "tcp_5900"
    priority                   = 500
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "5900"
    source_address_prefix      = "*"
    destination_address_prefix = "*"
  }
  security_rule {
    name                       = "tcp_8080"
    priority                   = 600
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "8080"
    source_address_prefix      = "*"
    destination_address_prefix = "*"
  }
  security_rule {
    name                       = "tcp_8443"
    priority                   = 700
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "8443"
    source_address_prefix      = "*"
    destination_address_prefix = "*"
  }
  security_rule {
    name                       = "k8s_nodeport"
    priority                   = 800
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "30000 - 32767"
    source_address_prefix      = "*"
    destination_address_prefix = "*"
  }
}

locals {
  nodeconfig = [
    for vm in var.nodeconfig : [
      for i in range(1, vm.nodecount+1) : {
        instance_name 	= "${vm.role}-${vm.cluster}-${i}"
        instance_type 	= vm.instance_type
		    nodenum			    = i
		    cluster 		    = vm.cluster
        blockdisks 		  = vm.block_devices
		    ip_start        = vm.ip_start
      }
    ]
  ]
}

locals {
  instances = flatten(local.nodeconfig)
}

resource "azurerm_public_ip" "pub_ip" {
  for_each  		  =	{for server in local.instances: server.instance_name =>  server}
  name                = format("%s.%s.%s",var.name_prefix,var.config_name,each.key)
  ip_version          = "IPv4"
  resource_group_name = azurerm_resource_group.rg.name
  location            = azurerm_resource_group.rg.location
  allocation_method   = "Static"
  tags                = var.azure_tags
}

data "azurerm_public_ip" "pub_ip" {
  for_each  		  =	{for server in local.instances: server.instance_name =>  server}
  name = azurerm_public_ip.pub_ip[each.key].name
  resource_group_name = azurerm_resource_group.rg.name
}

resource "azurerm_network_interface" "nic" {
  for_each  		  =	{for server in local.instances: server.instance_name =>  server}
  name                = format("%s.%s.%s",var.name_prefix,var.config_name,each.key)
  resource_group_name = azurerm_resource_group.rg.name
  location            = azurerm_resource_group.rg.location
  tags                = var.azure_tags

  ip_configuration {
    name                          = "internal"
    subnet_id                     = azurerm_subnet.subnet[each.value.cluster - 1].id
    private_ip_address_allocation = "Static"
    private_ip_address = format("%s.%s.%s",var.ip_base,each.value.cluster+100, each.value.ip_start + each.value.nodenum)
    public_ip_address_id = azurerm_public_ip.pub_ip[each.key].id
  }
}

locals {
  diskconfig = [
    for server in local.instances : [
      for index,i in server.blockdisks : {
        name = format("%s-%s",server.instance_name,index)
        attach_node = server.instance_name
        lun         = index+10
        type        = i.type
        size        = i.size
      }
    ]
  ]
}

locals {
  datadisks = flatten(local.diskconfig)
}

resource "azurerm_managed_disk" "data" {
  for_each              = {for disk in local.datadisks: disk.name => disk}
  name                  = format("%s-%s",var.config_name,each.key)
  resource_group_name   = azurerm_resource_group.rg.name
  location              = azurerm_resource_group.rg.location
  storage_account_type  = each.value.type
  create_option         = "Empty"
  disk_size_gb          = each.value.size
}

resource "azurerm_virtual_machine_data_disk_attachment" "data" {
  for_each           = {for disk in local.datadisks: disk.name => disk}
  managed_disk_id    = azurerm_managed_disk.data[each.key].id
  virtual_machine_id = azurerm_linux_virtual_machine.node[each.value.attach_node].id
  lun                = each.value.lun
  caching            = "ReadWrite"
}

resource "azurerm_linux_virtual_machine" "node" {
  for_each  		      =	{for server in local.instances: server.instance_name =>  server}
  name                = each.key
  resource_group_name = azurerm_resource_group.rg.name
  location            = azurerm_resource_group.rg.location
  size                = each.value.instance_type
  admin_username      = "rocky"
  tags                = var.azure_tags
  
  network_interface_ids = [
    azurerm_network_interface.nic[each.key].id,
  ]
  user_data = base64gzip(local_file.cloud-init[each.key].content)

  admin_ssh_key {
    public_key = azurerm_ssh_public_key.deploy_key.public_key
    username = "rocky"
  }
  
  os_disk {
    name = each.key
    caching = "ReadWrite"
    storage_account_type = "Standard_LRS"
    disk_size_gb = 50
  }

// when changing dont forget to update information how to accept eula
// e.g. 'az vm image terms accept --urn "resf:rockylinux-x86_64:9-base:9.3.20231113"'

  plan {
    name = "9-base"
    publisher = "resf"
    product = "rockylinux-x86_64"
  }

  source_image_reference {
    publisher = "resf"
    offer     = "rockylinux-x86_64"
    sku       = "9-base"
    version   = "9.3.20231113"
  }

  connection {
      type = "ssh"
      user = "rocky"
      host = "${data.azurerm_public_ip.pub_ip[each.key].ip_address}"
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
    source = format("%s/%s",path.module,each.key)
    destination = format("%s%s%s","/tmp/",each.key,"_scripts.sh")
  }

  provisioner "file" {
	  source = "/px-deploy/.px-deploy/assets/"
		destination = "/assets"
	}
}

resource "local_file" "cloud-init" {
	for_each 					=	{for server in local.instances: server.instance_name =>  server}
	content = templatefile("${path.module}/cloud-init.tpl", {
		tpl_priv_key = base64encode(tls_private_key.ssh.private_key_openssh),
		tpl_name = each.key
		tpl_azure_client = var.azure_client_id,
		tpl_azure_secret = var.azure_client_secret,
		tpl_azure_tenant = var.azure_tenant_id,
    tpl_azure_group = azurerm_resource_group.rg.name,
  	tpl_cluster = each.value.cluster
		}	
	)
	filename = "${path.module}/cloud-init-${each.key}-generated.yaml"
}
