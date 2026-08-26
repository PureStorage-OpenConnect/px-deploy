variable "eks_nodes" {
	description = "number of worker nodes"
	type 		= number
}

variable "eks_version" {
	description = "EKS k8s version"
	type 		= string
}

variable "run_everywhere" {
   description = "content of run_everywhere"
   type = string
   default = "echo \"no run_everywhere set\""
}

variable "eksclusters" {
	description = "map of clusternumber & aws_type"
	type 		= map
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


resource "aws_subnet" "eks_private1" {
  count	= var.clusters
  vpc_id = aws_vpc.vpc.id
  availability_zone = data.aws_availability_zones.available.names[0]
  cidr_block = "192.168.${count.index + 151}.0/24"
  tags = {
    Name = format("%s-%s-eks-private1-%s",var.name_prefix,var.config_name, count.index + 1)
    "kubernetes.io/role/internal-elb" = 1
  }
}

resource "aws_subnet" "eks_private2" {
  count	= var.clusters
  vpc_id = aws_vpc.vpc.id
  availability_zone = data.aws_availability_zones.available.names[1]
  cidr_block = "192.168.${count.index + 181}.0/24"
  tags = {
    Name = format("%s-%s-eks-private2-%s",var.name_prefix,var.config_name, count.index + 1)
    "kubernetes.io/role/internal-elb" = 1
  }
}

resource "aws_subnet" "eks_private3" {
  count	= var.clusters
  vpc_id = aws_vpc.vpc.id
  availability_zone = data.aws_availability_zones.available.names[2]
  cidr_block = "192.168.${count.index + 111}.0/24"
  tags = {
    Name = format("%s-%s-eks-private3-%s",var.name_prefix,var.config_name, count.index + 1)
    "kubernetes.io/role/internal-elb" = 1
  }
}

resource "aws_subnet" "eks_public2" {
	count					= 	var.clusters
	availability_zone 		= 	data.aws_availability_zones.available.names[1]
	map_public_ip_on_launch =   true
  vpc_id 					=	aws_vpc.vpc.id
	cidr_block 				= 	"192.168.${count.index + 31}.0/24"
	tags = {
		Name = format("%s-%s-eks-public2-%s",var.name_prefix,var.config_name, count.index + 1)
		"kubernetes.io/role/elb" = 1
	}
}

resource "aws_subnet" "eks_public3" {
	count					= 	var.clusters
	availability_zone 		= 	data.aws_availability_zones.available.names[2]
  map_public_ip_on_launch =   true
	vpc_id 					=	aws_vpc.vpc.id
	cidr_block 				= 	"192.168.${count.index + 61}.0/24"
	tags = {
		Name = format("%s-%s-eks-public3-%s",var.name_prefix,var.config_name, count.index + 1)
		"kubernetes.io/role/elb" = 1
	}
}

resource "aws_route_table_association" "rt_eks_pub2" {
	count			= var.clusters
	subnet_id 		= aws_subnet.eks_public2[count.index].id
	route_table_id 	= aws_route_table.rt.id
}

resource "aws_route_table_association" "rt_eks_pub3" {
	count			= var.clusters
	subnet_id 		= aws_subnet.eks_public3[count.index].id
	route_table_id 	= aws_route_table.rt.id
}

resource "aws_route_table" "rt_sn_private" {
  count	= var.clusters        
  vpc_id  = aws_vpc.vpc.id
  route {
    cidr_block = "0.0.0.0/0"
    nat_gateway_id = aws_nat_gateway.natgw.id
  }

  tags = {
    Name = format("%s-%s-eks-private-rt-%s",var.name_prefix,var.config_name, count.index + 1)
  }
}

resource "aws_route_table_association" "rta_private1" {
    count               = var.clusters
    subnet_id           = aws_subnet.eks_private1[count.index].id
    route_table_id      = aws_route_table.rt_sn_private[count.index].id
}

resource "aws_route_table_association" "rta_private2" {
    count               = var.clusters
    subnet_id           = aws_subnet.eks_private2[count.index].id
    route_table_id      = aws_route_table.rt_sn_private[count.index].id
}

resource "aws_route_table_association" "rta_private3" {
    count               = var.clusters
    subnet_id           = aws_subnet.eks_private3[count.index].id
    route_table_id      = aws_route_table.rt_sn_private[count.index].id
}

resource "aws_iam_role" "eks-iam-role" {
  name = format("%s-%s-eks-iam",var.name_prefix,var.config_name)
  path = "/"
  assume_role_policy = jsonencode({
    Statement: [{
     Action = "sts:AssumeRole"
     Effect = "Allow"
     Principal = {
       Service = "eks.amazonaws.com"
     }
   }]
   Version = "2012-10-17"
  })
}

resource "aws_iam_role_policy_attachment" "AmazonEKSClusterPolicy" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"
  role    = aws_iam_role.eks-iam-role.name
}

resource "aws_iam_role_policy_attachment" "AmazonEC2ContainerRegistryReadOnly-EKS" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
  role    = aws_iam_role.eks-iam-role.name
}

resource "aws_iam_role_policy_attachment" "AmazonEKSWorkerNodePolicy" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy"
  role       = aws_iam_role.node-iam-role.name
}

resource "aws_iam_role_policy_attachment" "AmazonEKS_CNI_Policy" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy"
  role       = aws_iam_role.node-iam-role.name
}

resource "aws_iam_role_policy_attachment" "AmazonEC2ContainerRegistryReadOnly" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
  role       = aws_iam_role.node-iam-role.name
}

resource "aws_eks_cluster" "eks" {
  for_each = var.eksclusters
  name = format("%s-%s-%s",var.name_prefix,var.config_name, each.key)
  version = var.eks_version
  role_arn = aws_iam_role.eks-iam-role.arn
  vpc_config {
    subnet_ids = [aws_subnet.eks_private1[each.key - 1].id, aws_subnet.eks_private2[each.key - 1].id, aws_subnet.eks_private3[each.key - 1].id, aws_subnet.subnet[each.key - 1].id, aws_subnet.eks_public2[each.key - 1].id, aws_subnet.eks_public3[each.key - 1].id]
  }    

  depends_on = [
    aws_iam_role.eks-iam-role,
    aws_iam_role_policy_attachment.AmazonEKSClusterPolicy,
    aws_iam_role_policy_attachment.AmazonEC2ContainerRegistryReadOnly-EKS,
  ]
  tags = {
    Name = format("%s-%s-%s",var.name_prefix,var.config_name, each.key)
  }
}

resource "aws_eks_node_group" "worker-node-group" {
  for_each = var.eksclusters
  cluster_name  = aws_eks_cluster.eks[each.key].name
  node_group_name = format("%s-%s-%s",var.name_prefix,var.config_name, each.key)
  node_role_arn  = aws_iam_role.node-iam-role.arn
  subnet_ids = [aws_subnet.eks_private1[each.key - 1].id, aws_subnet.eks_private2[each.key - 1].id, aws_subnet.eks_private3[each.key - 1].id, aws_subnet.subnet[each.key - 1].id, aws_subnet.eks_public2[each.key - 1].id, aws_subnet.eks_public3[each.key - 1].id]
   
  launch_template {
    id      = data.aws_launch_template.cluster[each.key].id
    version = data.aws_launch_template.cluster[each.key].latest_version
  }
  
  scaling_config {
   desired_size = var.eks_nodes
   max_size   = var.eks_nodes
   min_size   = var.eks_nodes
  }
 
  depends_on = [
   aws_iam_role_policy_attachment.AmazonEKSWorkerNodePolicy,
   aws_iam_role_policy_attachment.AmazonEKS_CNI_Policy,
   aws_iam_role_policy_attachment.AmazonEC2ContainerRegistryReadOnly,
  ]
  tags = {
    Name = format("%s-%s-%s",var.name_prefix,var.config_name, each.key)
  }
 }

resource "local_file" "eks_run_everywhere" {
        content = templatefile("${path.module}/eks_run_everywhere.tpl", {
                tpl_cmd = var.run_everywhere
                }
        )
        filename = "${path.module}/eks_run_everywhere-generated.yaml"
}

data "aws_launch_template" "cluster" {
  for_each = var.eksclusters
  name = aws_launch_template.cluster[each.key].name
  //depends_on = [aws_launch_template.cluster[each.key]]
}

resource "aws_launch_template" "cluster" {
  for_each = var.eksclusters
  name = format("%s-%s-%s",var.name_prefix,var.config_name, each.key)
  instance_type = each.value
  metadata_options {
    http_endpoint               = "enabled"
    http_put_response_hop_limit = 3
  }

  block_device_mappings {
    device_name = "/dev/xvda"
    ebs {
      volume_size = 50
      volume_type = "gp2"
    }
  }
  tag_specifications {
    resource_type = "instance"
    tags = merge(var.aws_tags, {
      Name = format("%s-%s-%s-node",var.name_prefix,var.config_name, each.key)
    },)
  }
  tag_specifications {
    resource_type = "volume"
    tags = merge(var.aws_tags, {
      Name = format("%s-%s-%s-node",var.name_prefix,var.config_name, each.key)
    },)
  }
  user_data = base64encode(local_file.eks_run_everywhere.content)
  key_name =  aws_key_pair.deploy_key.key_name
}
