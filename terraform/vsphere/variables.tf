variable "config_name" {
	description = "px-deploy config name"
	type 		= string
}

variable "name_prefix" {
	description = "prefix to apply to name of ressources"
    type 		= string
    default     = "px-deploy"
}

variable "clusters" {
	description 	= "number of clusters to create"
	type			= number
}

variable "nodeconfig" {
	description		= "list / config of all gcp instances"
	default = [{}]
}

variable "vsphere_host" {
	description = "vCenter Server"
	type = string
}

variable "vsphere_compute_resource" {
	description = "vSphere Cluster"
	type = string
}

variable "vsphere_resource_pool" {
	description = "vCenter resource pool"
	type = string
}

variable "vsphere_datacenter" {
	description = "vCenter Datacenter"
	type = string
}

variable "vsphere_template" {
	description = "px-deploy template"
	type = string
}

variable "vsphere_folder" {
	description = "vCenter Folder"
	type = string
}

variable "vsphere_user" {
	description = "vCenter user"
	type = string
}

variable "vsphere_password" {
	description = "vCenter password"
	type = string
}

variable "vsphere_datastore" {
	description = "vCenter Datastore"
	type = string
}

variable "vsphere_network" {
	description = "vCenter Network"
	type = string
}

variable "vsphere_memory" {
	description = "vSphere Memory"
	type = string
}

variable "vsphere_cpu" {
	description = "vSphere CPU"
	type = string
}

variable "vsphere_ip" {
	description = "vSphere VM starting IP"
	type = string
	default = ""
}

variable "vsphere_gw" {
	description = "vSphere VM Gateway"
	type = string
	default = ""
}

variable "vsphere_dns" {
	description = "vSphere VM DNS"
	type = string
	default = ""
}

variable "aws_access_key_id" {
	description ="AWS Access Key"
	type		= string
}

variable "aws_secret_access_key" {
	description ="AWS Secret Access Key"
	type		= string
}