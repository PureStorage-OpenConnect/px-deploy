terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "5.99.1"
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
	helm = {
      source  = "hashicorp/helm"
      version = "2.16.1"
    }
    rancher2 = {
      source  = "rancher/rancher2"
      version = "6.0.0"
    }
	random = {
		source ="hashicorp/random"
	}
    ssh = {
      source  = "loafoe/ssh"
      version = "2.7.0"
    }
    time = {
	  source = "hashicorp/time"		
    }
  }
}

provider "aws" {
	region 	= var.aws_region
	default_tags {
		tags = var.aws_tags
	  }
}

data "aws_availability_zones" "available" {
	state = "available"
	filter {
          name   = "opt-in-status"
          values = ["opt-in-not-required"]
        }
}

# how to find it: aws ec2 describe-images  --owners 679593333241 --filters Name=architecture,Values=x86_64 Name=name,Values="Rocky-9*" Name=root-device-type,Values=ebs
# check for availability in different aws regions!
data "aws_ami" "rocky" {
  owners = ["679593333241"]
  include_deprecated = true  
  most_recent = true
  filter {
    name   = "name"
    values = ["Rocky-9-EC2-Base-9.5-20241118.0.x86_64-3f230a17-9877-4b16-aa5e-b1ff34ab206b"]
  }
   
  filter {
	name = "architecture"
	values = ["x86_64"]
  }
  
  filter {
	name = "root-device-type"
	values = ["ebs"]
  }
}

resource "tls_private_key" "ssh" {
	algorithm = "RSA" 
	rsa_bits  = 2048
}

resource "local_file" "ssh_private_key" {
	content = tls_private_key.ssh.private_key_openssh
	file_permission = "0600"
	filename = format("/px-deploy/.px-deploy/keys/id_rsa.aws.%s",var.config_name)
}

resource "local_file" "ssh_public_key" {
	content = tls_private_key.ssh.public_key_openssh
	file_permission = "0644"
	filename = format("/px-deploy/.px-deploy/keys/id_rsa.aws.%s.pub",var.config_name)
}

resource "aws_key_pair" "deploy_key" {
	key_name = format("px-deploy.%s",var.config_name)
	public_key = tls_private_key.ssh.public_key_openssh
}

resource "aws_vpc" "vpc" {
	cidr_block	= var.aws_cidr_vpc
	enable_dns_hostnames	= true
	enable_dns_support		= true
	tags = {
		Name = format("%s.%s-%s",var.name_prefix,var.config_name,"vpc")
	}
}

resource "aws_subnet" "subnet" {
	count					= 	var.clusters
	availability_zone 		= 	data.aws_availability_zones.available.names[0]
	map_public_ip_on_launch =   true
	vpc_id 					=	aws_vpc.vpc.id
	cidr_block 				= 	"192.168.${count.index + 101}.0/24"
	tags = {
		Name = format("%s-%s-subnet-%s",var.name_prefix,var.config_name, count.index + 1)
		"kubernetes.io/role/elb" = 1
		}
}

data "aws_caller_identity" "current" {}

resource "aws_vpc_endpoint" "guardduty" {
  vpc_id            = aws_vpc.vpc.id
  service_name      = format("com.amazonaws.%s.guardduty-data",var.aws_region)
  vpc_endpoint_type = "Interface"
  
  subnet_ids = [
    aws_subnet.subnet[0].id
  ]

  security_group_ids = [
    aws_security_group.guardduty.id
  ]

  private_dns_enabled = true

  policy = jsonencode({
	Version = "2012-10-17"
	Statement= [{
			Action= "*",
			Resource= "*",
			Effect= "Allow",
			Principal= "*"
		},
		{
			Condition= {StringNotEquals= {"aws:PrincipalAccount": "${data.aws_caller_identity.current.account_id}"}}
			Action= "*",
			Resource= "*",
			Effect= "Deny",
			Principal= "*"
		}
	]
})
}

resource "aws_security_group" "guardduty" {
	name 		= 	format("pxd-gd-%s",var.config_name)
	description = 	"Security group for guardduty"
	vpc_id = aws_vpc.vpc.id
	
   	ingress {
		description = "https"
		from_port 	= 443
		to_port 	= 443
		protocol	= "tcp"
		cidr_blocks = [var.aws_cidr_vpc]
		}
    tags = {
		Name=format("pxd-gd-%s",var.config_name)
		}
}


resource "aws_internet_gateway" "igw" {
	vpc_id = aws_vpc.vpc.id
	tags = {
		Name = format("%s-%s-%s",var.name_prefix,var.config_name,"igw")
	}
}

resource "aws_route_table" "rt" {
	vpc_id = aws_vpc.vpc.id
	route {
		cidr_block = "0.0.0.0/0"
		gateway_id = aws_internet_gateway.igw.id
	}
	tags = {
		Name = format("%s-%s-%s",var.name_prefix,var.config_name,"rt")
	}  
}

resource "aws_route_table_association" "rt" {
	count			= var.clusters
	subnet_id 		= aws_subnet.subnet[count.index].id
	route_table_id 	= aws_route_table.rt.id
}

resource "aws_security_group" "sg_px-deploy" {
	name 		= 	format("px-deploy-%s",var.config_name)
	description = 	"Security group for px-deploy (tf-created)"
	vpc_id = aws_vpc.vpc.id
	ingress {
		description = "ssh"
		from_port 	= 22
		to_port 	= 22
		protocol	= "tcp"
		cidr_blocks = ["0.0.0.0/0"]
		}
	ingress {
		description = "http"
		from_port 	= 80
		to_port 	= 80
		protocol	= "tcp"
		cidr_blocks = ["0.0.0.0/0"]
		}
   	ingress {
		description = "https"
		from_port 	= 443
		to_port 	= 443
		protocol	= "tcp"
		cidr_blocks = ["0.0.0.0/0"]
		}
    ingress {
		description = "tcp 2382"
		from_port 	= 2382
		to_port 	= 2382
		protocol	= "tcp"
		cidr_blocks = ["0.0.0.0/0"]
		}
    ingress {
		description = "tcp 5900"
		from_port 	= 5900
		to_port 	= 5900
		protocol	= "tcp"
		cidr_blocks = ["0.0.0.0/0"]
		}
    ingress {
		description = "tcp 8080"
		from_port 	= 8080
		to_port 	= 8080
		protocol	= "tcp"
		cidr_blocks = ["0.0.0.0/0"]
		}
    ingress {
		description = "tcp 8443"
		from_port 	= 8443
		to_port 	= 8443
		protocol	= "tcp"
		cidr_blocks = ["0.0.0.0/0"]
		}
	ingress {
		description = "tcp 6443"
		from_port 	= 6443
		to_port 	= 6443
		protocol	= "tcp"
		cidr_blocks = ["0.0.0.0/0"]
		}
    ingress {
		description = "k8s nodeport"
		from_port 	= 30000
		to_port 	= 32767
		protocol	= "tcp"
		cidr_blocks = ["0.0.0.0/0"]
		}

    ingress {
		description = "all ingress from within vpc"
		from_port 	= 0
		to_port 	= 0 
		protocol	= "all"
		cidr_blocks = [aws_vpc.vpc.cidr_block]
		}

	egress {
		from_port   = 0
		to_port     = 0
		protocol    = "-1"
		cidr_blocks = ["0.0.0.0/0"]
		}
	tags = {
		Name=format("px-deploy-%s",var.config_name)
		}
}

resource "aws_iam_policy" "px-policy" {
  name = format("px-policy-%s-%s",var.name_prefix,var.config_name)
  description = "portworx node policy"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid = "" 
        Effect = "Allow"
        Action = [
          "ec2:AttachVolume",
          "ec2:ModifyVolume",
          "ec2:DetachVolume",
          "ec2:CreateTags",
          "ec2:CreateVolume",
          "ec2:DeleteTags",
          "ec2:DeleteVolume",
          "ec2:DescribeTags",
          "ec2:DescribeVolumeAttribute",
          "ec2:DescribeVolumesModifications",
          "ec2:DescribeVolumeStatus",
          "ec2:DescribeVolumes",
          "ec2:DescribeInstances",
          "autoscaling:DescribeAutoScalingGroups"
        ]
        Resource = "*"
      },
      {
        Effect = "Allow"
        Action = [
          "route53:ChangeResourceRecordSets"
        ]
        Resource = "arn:aws:route53:::hostedzone/*"
      },
      {
        Effect = "Allow"
        Action = [
          "route53:ListHostedZones",
          "route53:ListResourceRecordSets"
        ]
        Resource = "*"
      }
    ]
  })
}

resource "aws_iam_policy" "elb-policy" {
  name = format("elb-policy-%s-%s",var.name_prefix,var.config_name)
  description = "px-deploy elb node policy"
  policy = data.aws_iam_policy_document.elb_policy_doc.json
}

resource "aws_iam_role" "node-iam-role" {
  name = format("%s-%s-nodes",var.name_prefix,var.config_name)

  assume_role_policy = jsonencode({
    Statement = [{
      Action = "sts:AssumeRole"
      Effect = "Allow"
      Principal = {
        Service = "ec2.amazonaws.com"
      }
    }]
    Version = "2012-10-17"
  })
}

resource "aws_iam_role_policy_attachment" "px-pol-attach" {
  role       = aws_iam_role.node-iam-role.name
  policy_arn = aws_iam_policy.px-policy.arn
}

resource "aws_iam_role_policy_attachment" "elb-pol-attach" {
  role       = aws_iam_role.node-iam-role.name
  policy_arn = aws_iam_policy.elb-policy.arn
}

resource "aws_iam_instance_profile" "ec2_profile" {
	name = format("%s-%s-inst-prof",var.name_prefix,var.config_name)
	role = aws_iam_role.node-iam-role.name
}

locals {
  nodeconfig = [
    for vm in var.nodeconfig : [
      for i in range(1, vm.nodecount+1) : {
        instance_name 	= "${vm.role}-${vm.cluster}-${i}"
        instance_type 	= vm.instance_type
		nodenum			= i
		cluster 		= vm.cluster
        blockdisks 		= vm.ebs_block_devices
		ip_start 		= vm.ip_start
      }
    ]
  ]
}

locals {
  instances = flatten(local.nodeconfig)
}

resource "aws_instance" "node" {
	for_each 					=	{for server in local.instances: server.instance_name =>  server}
	ami 						= 	data.aws_ami.rocky.id
	instance_type				=	each.value.instance_type
	iam_instance_profile		=	aws_iam_instance_profile.ec2_profile.name	
	vpc_security_group_ids 		=	[aws_security_group.sg_px-deploy.id]
	subnet_id					=	aws_subnet.subnet[each.value.cluster - 1].id
	private_ip 					= 	format("%s.%s.%s",var.ip_base,each.value.cluster+100, each.value.ip_start + each.value.nodenum)
	associate_public_ip_address = 	true
	key_name 					= 	aws_key_pair.deploy_key.key_name
	
	root_block_device {
		encrypted               =   true
	  	volume_size				=	50
	  	delete_on_termination 	= 	true
		tags = merge({Name = format("%s.%s.%s.%s",var.name_prefix,var.config_name,each.key,"root")}, var.aws_tags)
	}

	metadata_options {
		http_tokens = "required"
		http_put_response_hop_limit = "2"
	}

	timeouts {
		delete = "30m"
	}
	
	dynamic "ebs_block_device"{
    	for_each 				= each.value.blockdisks
    	content {
			encrypted           = true
      		volume_type 		= ebs_block_device.value.ebs_type
      		volume_size 		= ebs_block_device.value.ebs_size
			tags = merge({Name = format("%s.%s.%s.%s",var.name_prefix,var.config_name,each.key,"root")}, var.aws_tags)
      		device_name 		= ebs_block_device.value.ebs_device_name
    	}
	}
	user_data_base64			= 	base64gzip(local_file.cloud-init[each.key].content)
	tags 					= {
								Name = each.key
								pxd_cluster = each.value.cluster
	}

        connection {
                        type = "ssh"
                        user = "rocky"
                        host = "${self.public_ip}"
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
		tpl_aws_access_key_id = var.aws_access_key_id
		tpl_aws_secret_access_key = var.aws_secret_access_key
		tpl_name = each.key
		tpl_vpc = aws_vpc.vpc.id,
		tpl_sg = aws_security_group.sg_px-deploy.id,
		tpl_subnet = aws_subnet.subnet[each.value.cluster - 1].id,
		tpl_gw = aws_internet_gateway.igw.id,
		tpl_routetable = aws_route_table.rt.id,
		tpl_ami = 	data.aws_ami.rocky.id,
		tpl_cluster = each.value.cluster
		}	
	)
	filename = "${path.module}/cloud-init-${each.key}-generated.yaml"
}

#resource "aws_s3_bucket" "drbucket" {
#  bucket = format("%s-%s",var.name_prefix,var.config_name)
#  force_destroy = true
#}

resource "local_file" "aws-returns" {
	content = templatefile("${path.module}/aws-returns.tpl", { 
		tpl_vpc = aws_vpc.vpc.id,
		tpl_sg = aws_security_group.sg_px-deploy.id,
		tpl_gw = aws_internet_gateway.igw.id,
		tpl_routetable = aws_route_table.rt.id,
		tpl_ami = 	data.aws_ami.rocky.id,
		}
	)
	filename = "${path.module}/aws-returns-generated.yaml"
}
