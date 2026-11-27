package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	compute "google.golang.org/api/compute/v1"
	container "google.golang.org/api/container/v1"
	"google.golang.org/api/option"
)

func gcp_compute_connect(Auth_Json string) (*compute.Service, error) {
	computeService, err := compute.NewService(context.Background(), option.WithCredentialsFile(Auth_Json))
	if err != nil {
		return nil, err
	}
	return computeService, nil
}

func gcp_container_connect(Auth_Json string) (*container.Service, error) {
	containerService, err := container.NewService(context.Background(), option.WithCredentialsFile(Auth_Json))
	if err != nil {
		return nil, err
	}
	return containerService, nil
}

func gcp_get_instances(config *Config) ([]string, error) {
	var instances []string

	computeService, err := gcp_compute_connect(config.Gcp_Auth_Json)
	if err != nil {
		return nil, err
	}

	listInstanceCall := computeService.Instances.List(config.Gcp_Project, fmt.Sprintf("%s-%s", config.Gcp_Region, config.Gcp_Zone))

	list, err := listInstanceCall.Do()
	if err != nil {
		return nil, err
	}

	for _, val := range list.Items {
		for _, net := range val.NetworkInterfaces {
			if net.Network == config.Gcp__Vpc {
				instances = append(instances, val.Name)
			}
		}
	}
	return instances, nil
}

// detach disk from instance, wait for DONE, delete disk, wait for DONE
func gcp_detach_delete_wait_clouddrive(instance string, disk string, config *Config, wg *sync.WaitGroup) {
	defer wg.Done()
	computeService, err := gcp_compute_connect(config.Gcp_Auth_Json)
	if err != nil {
		cloud_fail("gcp", "detaching or deleting a Portworx cloud drive", err)
	}

	detachresult, err := computeService.Instances.DetachDisk(config.Gcp_Project, fmt.Sprintf("%s-%s", config.Gcp_Region, config.Gcp_Zone), instance, disk).Do()
	if err != nil {
		cloud_fail("gcp", "detaching or deleting a Portworx cloud drive", err)
	}

	err = gcp_waiter(config, computeService, detachresult.Name)
	if err != nil {
		cloud_fail("gcp", "detaching or deleting a Portworx cloud drive", err)
	}

	deletediskresult, err := computeService.Disks.Delete(config.Gcp_Project, fmt.Sprintf("%s-%s", config.Gcp_Region, config.Gcp_Zone), disk).Do()
	if err != nil {
		cloud_fail("gcp", "detaching or deleting a Portworx cloud drive", err)
	}

	err = gcp_waiter(config, computeService, deletediskresult.Name)
	if err != nil {
		cloud_fail("gcp", "detaching or deleting a Portworx cloud drive", err)
	}
}

func gcp_delete_wait_clouddrive(disk string, config *Config, wg *sync.WaitGroup) {
	defer wg.Done()
	computeService, err := gcp_compute_connect(config.Gcp_Auth_Json)
	if err != nil {
		cloud_fail("gcp", "deleting a Portworx cloud drive", err)
	}

	deletediskresult, err := computeService.Disks.Delete(config.Gcp_Project, fmt.Sprintf("%s-%s", config.Gcp_Region, config.Gcp_Zone), disk).Do()
	if err != nil {
		cloud_fail("gcp", "deleting a Portworx cloud drive", err)
	}

	err = gcp_waiter(config, computeService, deletediskresult.Name)
	if err != nil {
		cloud_fail("gcp", "deleting a Portworx cloud drive", err)
	}
}

// stop instance and wait for DONE
func gcp_stop_wait_instance(instance string, config *Config, wg *sync.WaitGroup) {
	defer wg.Done()

	computeService, err := gcp_compute_connect(config.Gcp_Auth_Json)
	if err != nil {
		cloud_fail("gcp", "stopping an instance", err)
	}

	stopresult, err := computeService.Instances.Stop(config.Gcp_Project, fmt.Sprintf("%s-%s", config.Gcp_Region, config.Gcp_Zone), instance).Do()
	if err != nil {
		cloud_fail("gcp", "stopping an instance", err)
	}

	err = gcp_waiter(config, computeService, stopresult.Name)
	if err != nil {
		cloud_fail("gcp", "stopping an instance", err)
	}
}

func gcp_get_nodepools(config *Config, cluster string) ([]string, error) {
	var nodepools []string

	containerService, err := gcp_container_connect(config.Gcp_Auth_Json)
	if err != nil {
		return nil, err
	}

	nodepool, err := containerService.Projects.Zones.Clusters.NodePools.List(config.Gcp_Project, fmt.Sprintf("%s-%s", config.Gcp_Region, config.Gcp_Zone), cluster).Do()
	if err != nil {
		return nil, err
	}

	for _, i := range nodepool.NodePools {
		nodepools = append(nodepools, i.Name)
	}
	return nodepools, nil
}

func gcp_delete_wait_nodepools(config *Config, cluster string, nodepool string, wg *sync.WaitGroup) {
	defer wg.Done()
	containerService, err := gcp_container_connect(config.Gcp_Auth_Json)
	if err != nil {
		cloud_fail("gcp", "deleting a GKE node pool", err)
	}

	deletepoolresult, err := containerService.Projects.Zones.Clusters.NodePools.Delete(config.Gcp_Project, fmt.Sprintf("%s-%s", config.Gcp_Region, config.Gcp_Zone), cluster, nodepool).Do()
	if err != nil {
		cloud_fail("gcp", "deleting a GKE node pool", err)
	}

	deleted := false
	for !deleted {
		deletestatus, err := containerService.Projects.Zones.Operations.Get(config.Gcp_Project, fmt.Sprintf("%s-%s", config.Gcp_Region, config.Gcp_Zone), deletepoolresult.Name).Do()
		if err != nil {
			cloud_fail("gcp", "deleting a GKE node pool", err)
		}
		switch deletestatus.Status {
		case "DONE":
			{
				starttime, _ := time.Parse(time.RFC3339, deletestatus.StartTime)
				endtime, _ := time.Parse(time.RFC3339, deletestatus.EndTime)
				fmt.Printf("  deleted nodepool '%s' (cluster '%s') after %v\n", nodepool, cluster, endtime.Sub(starttime).Round(time.Second))
				deleted = true
			}
		case "RUNNING", "PENDING":
			{
				starttime, _ := time.Parse(time.RFC3339, deletestatus.StartTime)
				fmt.Printf("\t deleting nodepool '%s' (cluster '%s'). %v elapsed\n", nodepool, cluster, time.Since(starttime).Round(time.Second))
				time.Sleep(10 * time.Second)
			}
		default:
			{
				fmt.Printf("Status check for nodepool %v on cluster %v  ended with unknown status %v", nodepool, cluster, deletestatus.Status)
				die("Deleting GKE node pool '" + nodepool + "' of cluster '" + cluster + "' ended with status " + deletestatus.Status + ". Check the node pool in the GCP console, then run destroy again.")
			}
		}
	}
}

// enable the 2min default waiter, check for status "DONE", repeat if not DONE
func gcp_waiter(config *Config, service *compute.Service, operation string) error {

	gcpwaiter, err := service.ZoneOperations.Wait(config.Gcp_Project, fmt.Sprintf("%s-%s", config.Gcp_Region, config.Gcp_Zone), operation).Do()
	if err != nil {
		return err
	}

	for gcpwaiter.Status != "DONE" {
		fmt.Printf("\t hit 2min timeout. re-start waiter \n")
		gcpwaiter, err = service.ZoneOperations.Wait(config.Gcp_Project, fmt.Sprintf("%s-%s", config.Gcp_Region, config.Gcp_Zone), operation).Do()
		if err != nil {
			return err
		}
	}
	return nil
}

func gcp_get_clouddrives(instance string, config *Config) ([]string, error) {
	var clouddrives []string
	computeService, err := gcp_compute_connect(config.Gcp_Auth_Json)
	if err != nil {
		return nil, err
	}

	// instances of current vpc
	// not able to build this filter into golang filter
	// gcloud compute instances list --filter "networkInterfaces[].network:px-deploy-dpaul-gcp-vpc"
	//                                         (disks[].deviceName:px-do-not-delete-*) AND (networkInterfaces[].network:px-deploy-dpaul-gcp-vpc)

	listInstanceCall := computeService.Instances.List(config.Gcp_Project, fmt.Sprintf("%s-%s", config.Gcp_Region, config.Gcp_Zone))
	listInstanceCall.Filter("name=" + instance)
	list, err := listInstanceCall.Do()
	if err != nil {
		return nil, err
	}

	for _, val := range list.Items {
		for _, disk := range val.Disks {
			if strings.Contains(disk.DeviceName, "px-do-not-delete-") {
				clouddrives = append(clouddrives, disk.DeviceName)
			}
		}
	}

	return clouddrives, nil
}

func gcp_get_node_ip(deployment string, node string) string {
	config := parse_yaml("/px-deploy/.px-deploy/deployments/" + deployment + ".yml")
	computeService, err := gcp_compute_connect(config.Gcp_Auth_Json)
	if err != nil {
		cloud_fail("gcp", "connecting to the Compute API", err)
	}

	listInstanceCall := computeService.Instances.List(config.Gcp_Project, fmt.Sprintf("%s-%s", config.Gcp_Region, config.Gcp_Zone))
	listInstanceCall.Filter("name = " + node)
	list, err := listInstanceCall.Do()
	if err != nil {
		cloud_fail("gcp", "listing instances", err)
	}

	if len(list.Items) != 1 {
		fmt.Printf("Warning: found %v instance(s) of %v\n", len(list.Items), node)
		return ""
	} else {
		return strings.TrimSuffix(list.Items[0].NetworkInterfaces[0].AccessConfigs[0].NatIP, "\n")
	}
}

func gcp_create_variables(config *Config) []string {
	var tf_var_ebs []string
	var tf_var_tags []string
	var tf_variables []string
	var tf_cluster_instance_type string
	var tf_cluster_nodes string
	var tf_variables_eks []string
	var pxduser string

	// create EBS definitions
	// split ebs definition by spaces and range the results

	ebs := strings.Fields(config.Gcp_Disks)
	for _, val := range ebs {
		// split by : and create common .tfvars entry for all nodes
		entry := strings.Split(val, ":")
		tf_var_ebs = append(tf_var_ebs, "      {\n        ebs_type = \""+entry[0]+"\"\n        ebs_size = \""+entry[1]+"\"\n      },")
	}
	// other node ebs processing happens in cluster/node loop

	// AWS default tagging
	tf_var_tags = append(tf_var_tags, "aws_tags = {")

	if config.Tags != "" {
		tags := strings.Split(config.Tags, ",")
		for _, val := range tags {
			entry := strings.Split(val, "=")
			tf_var_tags = append(tf_var_tags, "  "+strings.ToLower(strings.TrimSpace(entry[0]))+" = \""+strings.ToLower(strings.TrimSpace(entry[1]))+"\"")
		}
	}
	// get PXDUSER env and apply to tf_variables
	pxduser = os.Getenv("PXDUSER")
	if pxduser != "" {
		tf_var_tags = append(tf_var_tags, "  px-deploy_username = \""+pxduser+"\"")
	} else {
		tf_var_tags = append(tf_var_tags, "  px-deploy_username = \"unknown\"")
	}
	tf_var_tags = append(tf_var_tags, "  px-deploy_name = \""+config.Name+"\"")
	tf_var_tags = append(tf_var_tags, "  pxd_uuid = \""+config.Pxd_uuid.String()+"\"")
	tf_var_tags = append(tf_var_tags, "}\n")

	switch config.Platform {
	case "gke":
		{
			tf_variables = append(tf_variables, "gke_nodes = \""+config.Nodes+"\"")
			tf_variables = append(tf_variables, "gke_version = \""+config.Gke_Version+"\"")
			config.Nodes = "0"
			//if config.Env["run_everywhere"] != "" {
			//	tf_variables = append(tf_variables, "run_everywhere = \""+strings.Replace(config.Env["run_everywhere"], "'", "\\\"", -1)+"\"")
			//}
		}
	}

	Clusters, _ := strconv.Atoi(config.Clusters)
	Nodes, _ := strconv.Atoi(config.Nodes)

	// build terraform variable file
	tf_variables = append(tf_variables, "config_name = \""+config.Name+"\"")
	tf_variables = append(tf_variables, "clusters = "+config.Clusters)
	tf_variables = append(tf_variables, "gcp_region = \""+config.Gcp_Region+"\"")
	tf_variables = append(tf_variables, "gcp_zone = \""+config.Gcp_Zone+"\"")
	tf_variables = append(tf_variables, "gcp_project = \""+config.Gcp_Project+"\"")
	tf_variables = append(tf_variables, "gcp_auth_json = \""+config.Gcp_Auth_Json+"\"")

	switch config.Platform {

	case "gke":
		{
			tf_variables_eks = append(tf_variables_eks, "gkeclusters = {")
		}
	}

	tf_variables = append(tf_variables, "nodeconfig = [")

	// loop clusters (masters and nodes) to build tfvars and master/node scripts
	for c := 1; c <= Clusters; c++ {
		masternum := strconv.Itoa(c)
		tf_cluster_instance_type = config.Gcp_Type
		tf_cluster_nodes = strconv.Itoa(Nodes)

		// if exist, apply individual scripts/settings for nodes of a cluster
		for _, clusterconf := range config.Cluster {
			if clusterconf.Id == c {
				//is there a cluster specific aws_type override? if not, set from generic config
				if clusterconf.Gcp_Type != "" {
					tf_cluster_instance_type = clusterconf.Gcp_Type
				}
				if clusterconf.Nodes != "" {
					tf_cluster_nodes = clusterconf.Nodes
				}
			}
		}

		// process .tfvars file for deployment
		tf_variables = append(tf_variables, "  {")
		tf_variables = append(tf_variables, "    role = \"master\"")
		tf_variables = append(tf_variables, "    ip_start = 89")
		tf_variables = append(tf_variables, "    nodecount = 1")
		tf_variables = append(tf_variables, "    instance_type = \""+config.Gcp_Type+"\"")
		tf_variables = append(tf_variables, "    cluster = "+masternum)
		tf_variables = append(tf_variables, "    ebs_block_devices = [] ")
		tf_variables = append(tf_variables, "  },")

		tf_variables = append(tf_variables, "  {")
		tf_variables = append(tf_variables, "    role = \"node\"")
		tf_variables = append(tf_variables, "    ip_start = 100")
		tf_variables = append(tf_variables, "    nodecount = "+tf_cluster_nodes)
		tf_variables = append(tf_variables, "    instance_type = \""+tf_cluster_instance_type+"\"")
		tf_variables = append(tf_variables, "    cluster = "+masternum)
		tf_variables = append(tf_variables, "    ebs_block_devices = [")
		tf_variables = append(tf_variables, tf_var_ebs...)
		tf_variables = append(tf_variables, "    ]\n  },")

		switch config.Platform {
		case "gke":
			{
				tf_variables_eks = append(tf_variables_eks, "  \""+masternum+"\" = \""+tf_cluster_instance_type+"\",")
			}
		}
	}
	tf_variables = append(tf_variables, "]")

	switch config.Platform {
	case "gke":
		{
			tf_variables_eks = append(tf_variables_eks, "}")
			tf_variables = append(tf_variables, tf_variables_eks...)
		}
	}

	tf_variables = append(tf_variables, tf_var_tags...)
	return tf_variables
}
