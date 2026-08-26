variable "ocp4_domain" {
	description = "domain used for ocp4 cluster"
	type 		= string
}

variable "ocp4_credentials_mode" {
	description = "credentials mode used for ocp4 cluster"
	type 		= string
}

variable "ocp4_pull_secret" {
	description = "ocp4 pull secret"
	type 		= string
}

variable "ocp4config" {
	description		= "list / config of all ocp4 instances"
	default = [{}]
}


resource "aws_vpc_dhcp_options" "dhcpopt" {
  domain_name          = format("%s.compute.internal",var.aws_region)
  domain_name_servers  = ["AmazonProvidedDNS"]
  tags = {
    Name = format("%s-%s-%s",var.name_prefix,var.config_name,"dhcp_opt")
  }
}

resource "aws_vpc_dhcp_options_association" "dns_resolver" {
  vpc_id          = aws_vpc.vpc.id
  dhcp_options_id = aws_vpc_dhcp_options.dhcpopt.id
}

resource "aws_eip" "nat_gateway" {
  domain     = "vpc"
  depends_on = [aws_internet_gateway.igw]
}

resource "aws_nat_gateway" "natgw" {
  allocation_id = aws_eip.nat_gateway.id
  subnet_id     = aws_subnet.subnet[0].id

  tags = {
        Name = format("%s-%s-%s",var.name_prefix,var.config_name,"ngw")
  }
  depends_on = [aws_internet_gateway.igw]
}


resource "aws_subnet" "ocp4_private1" {
  count	= var.clusters
  vpc_id = aws_vpc.vpc.id
  #availability_zone = aws_subnet.subnet[count.index].availability_zone
  availability_zone = data.aws_availability_zones.available.names[0]
  cidr_block = "192.168.${count.index + 151}.0/24"
  tags = {
    Name = format("%s-%s-ocp4-private1-subnet-%s",var.name_prefix,var.config_name, count.index + 1)
  }
}

resource "aws_subnet" "ocp4_private2" {
  count	= var.clusters
  vpc_id = aws_vpc.vpc.id
  #availability_zone = aws_subnet.subnet[count.index].availability_zone
  availability_zone = data.aws_availability_zones.available.names[1]
  cidr_block = "192.168.${count.index + 181}.0/24"
  tags = {
    Name = format("%s-%s-ocp4-private2-subnet-%s",var.name_prefix,var.config_name, count.index + 1)
  }
}

resource "aws_subnet" "ocp4_private3" {
  count	= var.clusters
  vpc_id = aws_vpc.vpc.id
  #availability_zone = aws_subnet.subnet[count.index].availability_zone
  availability_zone = data.aws_availability_zones.available.names[2]
  cidr_block = "192.168.${count.index + 211}.0/24"
  tags = {
    Name = format("%s-%s-ocp4-private3-subnet-%s",var.name_prefix,var.config_name, count.index + 1)
  }
}


resource "aws_route_table" "rt_sn_private" {
  count	= var.clusters        
  vpc_id  = aws_vpc.vpc.id
  route {
    cidr_block = "0.0.0.0/0"
    nat_gateway_id = aws_nat_gateway.natgw.id
  }

  tags = {
    Name = format("%s-%s-ocp4-private-rt-%s",var.name_prefix,var.config_name, count.index + 1)
  }
}

resource "aws_route_table_association" "rta_private1" {
    count               = var.clusters
    subnet_id           = aws_subnet.ocp4_private1[count.index].id
    route_table_id      = aws_route_table.rt_sn_private[count.index].id
}

resource "aws_route_table_association" "rta_private2" {
    count               = var.clusters
    subnet_id           = aws_subnet.ocp4_private2[count.index].id
    route_table_id      = aws_route_table.rt_sn_private[count.index].id
}

resource "aws_route_table_association" "rta_private3" {
    count               = var.clusters
    subnet_id           = aws_subnet.ocp4_private3[count.index].id
    route_table_id      = aws_route_table.rt_sn_private[count.index].id
}

resource "aws_subnet" "ocp_public2" {
	count					= 	var.clusters
	availability_zone 		= 	data.aws_availability_zones.available.names[1]
	map_public_ip_on_launch =   true
  vpc_id 					=	aws_vpc.vpc.id
	cidr_block 				= 	"192.168.${count.index + 31}.0/24"
	tags = {
		Name = format("%s-%s-ocp-public2-%s",var.name_prefix,var.config_name, count.index + 1)
	}
}

resource "aws_subnet" "ocp_public3" {
	count					= 	var.clusters
	availability_zone 		= 	data.aws_availability_zones.available.names[2]
  map_public_ip_on_launch =   true
	vpc_id 					=	aws_vpc.vpc.id
	cidr_block 				= 	"192.168.${count.index + 61}.0/24"
	tags = {
		Name = format("%s-%s-ocp-public3-%s",var.name_prefix,var.config_name, count.index + 1)
	}
}

resource "aws_route_table_association" "rt_ocp_pub2" {
	count			= var.clusters
	subnet_id 		= aws_subnet.ocp_public2[count.index].id
	route_table_id 	= aws_route_table.rt.id
}

resource "aws_route_table_association" "rt_ocp_pub3" {
	count			= var.clusters
	subnet_id 		= aws_subnet.ocp_public3[count.index].id
	route_table_id 	= aws_route_table.rt.id
}

resource "aws_security_group" "sg_ocp-nodes" {
	name 		= 	format("pxd-ocp-nodes-%s",var.config_name)
	description = 	"sg for ocp px east-west (tf-created)"
	vpc_id = aws_vpc.vpc.id
	ingress {
		description = "portworx mgmt (REST)"
		from_port 	= 17001
		to_port 	= 17001
		protocol	= "tcp"
		cidr_blocks = ["0.0.0.0/0"]
		}
  ingress {
    description = "portworx gRPC SDK gateway (REST)"
		from_port 	= 17018
		to_port 	= 17018
		protocol	= "tcp"
		cidr_blocks = ["0.0.0.0/0"]
		}
  ingress {
		description = "portworx ports"
		from_port 	= 17002
		to_port 	= 17021
		protocol	= "tcp"
		self = true
		}
  ingress {
		description = "portworx gossip"
		from_port 	= 17002
		to_port 	= 17002
		protocol	= "udp"
		self = true
		}
  ingress {
		description = "portworx rwx nfs"
		from_port 	= 2049
		to_port 	= 2049
		protocol	= "tcp"
		self = true
		}
  ingress {
		description = "portworx nfs portmapper"
		from_port 	= 111
		to_port 	= 111
		protocol	= "tcp"
		self = true
		}
  ingress {
		description = "portworx nfs mountd"
		from_port 	= 20048
		to_port 	= 20048
		protocol	= "tcp"
		self = true
		}
  ingress {
		description = "portworx rwx nfs"
		from_port 	= 2049
		to_port 	= 2049
		protocol	= "udp"
		self = true
		}
  ingress {
		description = "portworx nfs portmapper"
		from_port 	= 111
		to_port 	= 111
		protocol	= "udp"
		self = true
		}
  ingress {
		description = "portworx nfs mountd"
		from_port 	= 20048
		to_port 	= 20048
		protocol	= "udp"
		self = true
		}
	tags = {
		Name=format("px-deploy-%s",var.config_name)
		}
}

locals {
  ocp4config = [
    for ocp4cluster in var.ocp4config : {
        instance_type 	= ocp4cluster.instance_type
		nodecount		= ocp4cluster.nodecount
		clusternum 		= ocp4cluster.cluster
	}
  ]
}

locals {
  ocp4clusters = flatten(local.ocp4config)
}

resource "local_file" "ocp4-install-config" {
        for_each = {for cluster in local.ocp4clusters: cluster.clusternum =>  cluster}
        content = templatefile("${path.module}/ocp4-install-config.tpl", {
		                tpl_sshkey 	=  tls_private_key.ssh.public_key_openssh  
                        tpl_aws_region  = var.aws_region
                        tpl_aws_iamrole = aws_iam_role.node-iam-role.name
                        tpl_ocp4domain  = var.ocp4_domain
                        tpl_ocp4_credentials_mode = var.ocp4_credentials_mode
                        tpl_ocp4pullsecret = base64decode(var.ocp4_pull_secret)
                        tpl_cluster     = each.key
                        tpl_awstype     = each.value.instance_type
                        tpl_configname  = var.config_name
                        tpl_aws_tag     = var.aws_tags
                        tpl_nodes       = each.value.nodecount
                        tpl_cidr        = var.aws_cidr_vpc
                        tpl_privsubnet1  = aws_subnet.ocp4_private1[each.key - 1].id
                        tpl_privsubnet2  = aws_subnet.ocp4_private2[each.key - 1].id
                        tpl_privsubnet3  = aws_subnet.ocp4_private3[each.key - 1].id
                        tpl_pubsubnet1   = aws_subnet.subnet[each.key - 1].id
                        tpl_pubsubnet2   = aws_subnet.ocp_public2[each.key - 1].id
                        tpl_pubsubnet3   = aws_subnet.ocp_public3[each.key - 1].id
                        tpl_ocp_sg      = aws_security_group.sg_ocp-nodes.id
                }
        )
        filename = "${path.module}/ocp4-install-config-master-${each.key}-1.yaml"
}

// range thru the master nodes (by definition on ocp4 only master nodes...)
// copy the cluster specific ocp4 config file
resource "null_resource" "ocp4cluster" {
        for_each = aws_instance.node
 
        connection {
                type = "ssh"
                user = "rocky"
                host = each.value.public_ip
                private_key = tls_private_key.ssh.private_key_openssh
        }
	        
        provisioner "file" {
            source = format("%s/ocp4-install-config-%s.yaml",path.module,each.key)
            destination = "/tmp/ocp4-install-config.yaml"
        }
}
