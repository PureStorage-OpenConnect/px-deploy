variable "name_prefix" {
	description = "prefix to apply to name of ressources"
    type 		= string
    default     = "px-deploy"
}

variable "aws_tags" {
	description = "user-defined custom aws tags"
	type 		= map(string)
}

variable "config_name" {
	description = "px-deploy config name"
	type 		= string
}

variable "config_uuid" {
	description = "px-deploy deployment uuid"
	type 		= string
}

variable "clusters" {
	description 	= "number of clusters to create"
	type			= number
}

variable "nodeconfig" {
	description		= "list / config of all ec2 instances"
	default = [{}]
}

variable "ip_base" {
	description = "default first to ip octets"
	default = "192.168"
}

variable "aws_cidr_vpc" {
	description ="CIDR block for VPC"
	type		= string
	default 	= "192.168.0.0/16"
}

variable "aws_cidr_sn" {
	description ="CIDR block for Subnet"
	type		= string
	default 	= "192.168.0.0/16"
}

variable "aws_region" {
	description ="AWS Region"
	type		= string
}

variable "aws_access_key_id" {
	description ="AWS Access Key"
	type		= string
}

variable "aws_secret_access_key" {
	description ="AWS Secret Access Key"
	type		= string
}

data "local_file" "env_script" {
	filename = "${path.module}/env.sh"
}
