package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	awscredentials "github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/google/uuid"
)

func aws_create_variables(config *Config) []string {
	var pxduser string
	var tf_variables []string
	var tf_variables_ocp4 []string
	var tf_variables_rancher []string
	var tf_variables_eks []string
	var tf_cluster_instance_type string
	var tf_cluster_nodes string
	var tf_var_ebs []string
	var tf_var_tags []string
	// create EBS definitions
	// split ebs definition by spaces and range the results

	ebs := strings.Fields(config.Aws_Ebs)
	for i, val := range ebs {
		// split by : and create common .tfvars entry for all nodes
		entry := strings.Split(val, ":")
		tf_var_ebs = append(tf_var_ebs, fmt.Sprintf("      {\n        ebs_type = \"%s\"\n        ebs_size = \"%s\"\n        ebs_device_name = \"/dev/sd%c\"\n      },", entry[0], entry[1], i+98))
	}
	// other node ebs processing happens in cluster/node loop

	// AWS default tagging
	tf_var_tags = append(tf_var_tags, "aws_tags = {")

	if config.Tags != "" {
		tags := strings.Split(config.Tags, ",")
		for _, val := range tags {
			entry := strings.Split(val, "=")
			tf_var_tags = append(tf_var_tags, "  "+strings.TrimSpace(entry[0])+" = \""+strings.TrimSpace(entry[1])+"\"")
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
	//	case "ocp4":
	//		{
	//			tf_variables = append(tf_variables, "ocp4_nodes = \""+config.Nodes+"\"")
	//			config.Nodes = "0"
	//		}
	case "rancher":
		{
			tf_variables = append(tf_variables, "rancher_nodes = \""+config.Nodes+"\"")
			config.Nodes = "0"
		}
	case "eks":
		{
			tf_variables = append(tf_variables, "eks_nodes = \""+config.Nodes+"\"")
			tf_variables = append(tf_variables, "eks_version = \""+config.Eks_Version+"\"")
			config.Nodes = "0"
			if config.Env["run_everywhere"] != "" {
				tf_variables = append(tf_variables, "run_everywhere = \""+strings.Replace(config.Env["run_everywhere"], "'", "\\\"", -1)+"\"")
			}
		}
	}

	Clusters, _ := strconv.Atoi(config.Clusters)
	Nodes, _ := strconv.Atoi(config.Nodes)

	// build terraform variable file
	tf_variables = append(tf_variables, "config_name = \""+config.Name+"\"")
	tf_variables = append(tf_variables, "clusters = "+config.Clusters)
	tf_variables = append(tf_variables, "aws_region = \""+config.Aws_Region+"\"")
	tf_variables = append(tf_variables, "aws_access_key_id = \""+config.Aws_Access_Key_Id+"\"")
	tf_variables = append(tf_variables, "aws_secret_access_key = \""+config.Aws_Secret_Access_Key+"\"")
	tf_variables = append(tf_variables, "aws_session_token = \""+config.Aws_Session_Token+"\"")
	tf_variables = append(tf_variables, "aws_existing_vpc_id = \""+config.Aws_Existing_Vpc_Id+"\"")
	tf_variables = append(tf_variables, "aws_existing_subnet_id = \""+config.Aws_Existing_Subnet_Id+"\"")

	switch config.Platform {
	case "ocp4":
		{
			tf_variables = append(tf_variables, "ocp4_domain = \""+config.Ocp4_Domain+"\"")
			tf_variables = append(tf_variables, "ocp4_credentials_mode = \""+config.Ocp4_Credentials_Mode+"\"")
			tf_variables = append(tf_variables, "ocp4_pull_secret = \""+base64.StdEncoding.EncodeToString([]byte(config.Ocp4_Pull_Secret))+"\"")
			tf_variables_ocp4 = append(tf_variables_ocp4, "ocp4config = [")
		}
	case "eks":
		{
			tf_variables_eks = append(tf_variables_eks, "eksclusters = {")
		}
	case "rancher":
		{
			tf_variables = append(tf_variables, "rancher_domain = \""+config.Ocp4_Domain+"\"")
			tf_variables = append(tf_variables, "rancher_k3s_version = \""+config.Rancher_K3s_Version+"\"")
			tf_variables = append(tf_variables, "rancher_k8s_version = \""+config.Rancher_K8s_Version+"\"")
			tf_variables = append(tf_variables, "rancher_version = \""+config.Rancher_Version+"\"")
			tf_variables_rancher = append(tf_variables_rancher, "rancherclusters = {")
		}
	}

	tf_variables = append(tf_variables, "nodeconfig = [")

	// loop clusters (masters and nodes) to build tfvars and master/node scripts
	for c := 1; c <= Clusters; c++ {
		masternum := strconv.Itoa(c)
		tf_cluster_instance_type = config.Aws_Type
		tf_cluster_nodes = strconv.Itoa(Nodes)

		// if exist, apply individual scripts/settings for nodes of a cluster
		for _, clusterconf := range config.Cluster {
			if clusterconf.Id == c {
				//is there a cluster specific aws_type override? if not, set from generic config
				if clusterconf.Aws_Type != "" {
					tf_cluster_instance_type = clusterconf.Aws_Type
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
		tf_variables = append(tf_variables, "    instance_type = \"t3.large\"")
		tf_variables = append(tf_variables, "    cluster = "+masternum)
		tf_variables = append(tf_variables, "    ebs_block_devices = [] ")
		tf_variables = append(tf_variables, "  },")

		switch config.Platform {
		case "ocp4":
			{
				tf_variables_ocp4 = append(tf_variables_ocp4, "  {")
				tf_variables_ocp4 = append(tf_variables_ocp4, "    cluster = "+masternum)
				tf_variables_ocp4 = append(tf_variables_ocp4, "    nodecount = "+tf_cluster_nodes)
				tf_variables_ocp4 = append(tf_variables_ocp4, "    instance_type = \""+tf_cluster_instance_type+"\"")
				tf_variables_ocp4 = append(tf_variables_ocp4, "  },")
			}
		case "rancher":
			{
				tf_variables_rancher = append(tf_variables_rancher, "  \""+masternum+"\" = \""+tf_cluster_instance_type+"\",")
			}
		case "eks":
			{
				tf_variables_eks = append(tf_variables_eks, "  \""+masternum+"\" = \""+tf_cluster_instance_type+"\",")
			}
		default:
			{
				tf_variables = append(tf_variables, "  {")
				tf_variables = append(tf_variables, "    role = \"node\"")
				tf_variables = append(tf_variables, "    ip_start = 100")
				tf_variables = append(tf_variables, "    nodecount = "+tf_cluster_nodes)
				tf_variables = append(tf_variables, "    instance_type = \""+tf_cluster_instance_type+"\"")
				tf_variables = append(tf_variables, "    cluster = "+masternum)
				tf_variables = append(tf_variables, "    ebs_block_devices = [")
				tf_variables = append(tf_variables, tf_var_ebs...)
				tf_variables = append(tf_variables, "    ]\n  },")
			}
		}
	}
	tf_variables = append(tf_variables, "]")

	switch config.Platform {
	case "ocp4":
		{
			tf_variables_ocp4 = append(tf_variables_ocp4, "]")
			tf_variables = append(tf_variables, tf_variables_ocp4...)
		}
	case "rancher":
		{
			tf_variables_rancher = append(tf_variables_rancher, "}")
			tf_variables = append(tf_variables, tf_variables_rancher...)
		}
	case "eks":
		{
			tf_variables_eks = append(tf_variables_eks, "}")
			tf_variables = append(tf_variables, tf_variables_eks...)
		}
	}
	tf_variables = append(tf_variables, tf_var_tags...)
	return tf_variables
}

func aws_load_config(config *Config) aws.Config {
	cfg, err := awscfg.LoadDefaultConfig(
		context.TODO(),
		//awscfg.WithRetryer(func() aws.Retryer { return retry.AddWithMaxAttempts(retry.NewStandard(), 15) }),
		awscfg.WithRegion(config.Aws_Region),
		awscfg.WithCredentialsProvider(awscredentials.NewStaticCredentialsProvider(config.Aws_Access_Key_Id, config.Aws_Secret_Access_Key, config.Aws_Session_Token)))
	if err != nil {
		panic(fmt.Sprintf("failed loading config, %v \n", err))
	}
	return cfg
}

func aws_connect_ec2(awscfg *aws.Config) *ec2.Client {
	client := ec2.NewFromConfig(*awscfg)
	return client
}

// returns an array of instance IDs
func aws_get_instances(config *Config, client *ec2.Client, cluster int) ([]string, error) {
	var aws_instances []string
	var awsfilter []types.Filter

	awsfilter = append(awsfilter, types.Filter{Name: aws.String("tag:pxd_uuid"), Values: []string{config.Pxd_uuid.String()}})
	if cluster > 0 {
		awsfilter = append(awsfilter, types.Filter{Name: aws.String("tag:pxd_cluster"), Values: []string{strconv.Itoa(cluster)}})
	}

	instances, err := client.DescribeInstances(context.TODO(), &ec2.DescribeInstancesInput{Filters: awsfilter})

	if err != nil {
		fmt.Println("Got an error retrieving information about your Amazon EC2 instances:")
		return nil, err
	}

	for _, r := range instances.Reservations {
		for _, i := range r.Instances {
			//fmt.Println("   " + *i.InstanceId+"  "+ *i.PrivateIpAddress)
			aws_instances = append(aws_instances, *i.InstanceId)
		}
	}
	return aws_instances, nil
}

func aws_get_clouddrives(aws_instances_split []([]string), uuid uuid.UUID, client *ec2.Client) ([]string, error) {
	var aws_volumes []string

	fmt.Printf("Searching for portworx clouddrive volumes attached to instances within VPC\n")
	for i := range aws_instances_split {
		//get list of attached volumes, filter for PX Clouddrive Volumes
		volumes, err := client.DescribeVolumes(context.TODO(), &ec2.DescribeVolumesInput{
			Filters: []types.Filter{
				{
					Name:   aws.String("attachment.instance-id"),
					Values: aws_instances_split[i],
				},
				{
					Name: aws.String("tag:pxtype"),
					Values: []string{
						"data",
						"kvdb",
						"journal",
						"metadata",
					},
				},
			},
		})

		if err != nil {
			fmt.Println("Got an error retrieving information about volumes:")
			return nil, err
		}

		for _, i := range volumes.Volumes {
			fmt.Printf("  %s\n", *i.VolumeId)
			aws_volumes = append(aws_volumes, *i.VolumeId)
		}
	}
	fmt.Printf("Searching for portworx clouddrive volumes by deployment uuid tag: %s\n", uuid.String())
	//get list of attached volumes, filter for PX Clouddrive Volumes
	volumes, err := client.DescribeVolumes(context.TODO(), &ec2.DescribeVolumesInput{
		Filters: []types.Filter{
			{
				Name: aws.String("tag:pxd_uuid_cd"),
				Values: []string{
					uuid.String(),
				},
			},
		},
	})

	if err != nil {
		fmt.Println("Error retrieving volume list by uuid:")
		return nil, err
	}

	for _, i := range volumes.Volumes {
		fmt.Printf("  %s\n", *i.VolumeId)
		aws_volumes = append(aws_volumes, *i.VolumeId)
	}
	fmt.Println("merging volume results")
	slices.Sort(aws_volumes)
	return slices.Compact(aws_volumes), nil
}

func aws_delete_nodegroups(config *Config) error {
	cfg := aws_load_config(config)

	eksclient := eks.NewFromConfig(cfg)
	fmt.Println("Deleting EKS Nodegroups: (timeout 20min)")
	clusters, _ := strconv.Atoi(config.Clusters)
	for i := 1; i <= clusters; i++ {
		nodegroups, err := eksclient.ListNodegroups(context.TODO(), &eks.ListNodegroupsInput{
			ClusterName: aws.String(fmt.Sprintf("px-deploy-%s-%d", config.Name, i)),
		})
		if err != nil {
			fmt.Println("Error retrieving information about EKS Node Groups:")
			return err
		}

		wg.Add(len(nodegroups.Nodegroups))
		for _, nodegroupname := range nodegroups.Nodegroups {
			//fmt.Printf("Nodegroup %s \n",nodegroupname)
			go terminate_and_wait_nodegroup(eksclient, nodegroupname, fmt.Sprintf("px-deploy-%s-%d", config.Name, i), 20)
		}
	}
	wg.Wait()
	return nil
}

// get node ip implementation leveraging API calls
func aws_get_node_ip(deployment string, node string) string {
	config := parse_yaml("/px-deploy/.px-deploy/deployments/" + deployment + ".yml")
	var output []byte

	// load default config as we need current aws credentials
	defaultConfig := parse_yaml("/px-deploy/.px-deploy/defaults.yml")
	config.Aws_Access_Key_Id = defaultConfig.Aws_Access_Key_Id
	config.Aws_Secret_Access_Key = defaultConfig.Aws_Secret_Access_Key
	config.Aws_Session_Token = defaultConfig.Aws_Session_Token

	// connect to aws API
	cfg, err := awscfg.LoadDefaultConfig(
		context.TODO(),
		//awscfg.WithRetryer(func() aws.Retryer { return retry.AddWithMaxAttempts(retry.NewStandard(), 15) }),
		awscfg.WithRegion(config.Aws_Region),
		awscfg.WithCredentialsProvider(awscredentials.NewStaticCredentialsProvider(config.Aws_Access_Key_Id, config.Aws_Secret_Access_Key, config.Aws_Session_Token)))
	if err != nil {
		panic("aws configuration error, " + err.Error())
	}

	client := ec2.NewFromConfig(cfg)

	// get running instances matching node name in current VPC
	instances, err := client.DescribeInstances(context.TODO(), &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name: aws.String("tag:pxd_uuid"),
				Values: []string{
					config.Pxd_uuid.String(),
				},
			},
			{
				Name: aws.String("instance-state-name"),
				Values: []string{
					"running",
				},
			},
			{
				Name: aws.String("tag:Name"),
				Values: []string{
					node,
				},
			},
		},
	})

	if err != nil {
		fmt.Println("Got an error retrieving information about your Amazon EC2 instances:")
		panic("Error getting IP of instance:" + err.Error())
	}

	if len(instances.Reservations) == 1 {
		if len(instances.Reservations[0].Instances) == 1 {
			instance := instances.Reservations[0].Instances[0]
			// Prefer public IP if available, otherwise use private IP (for existing VPC deployments)
			if instance.PublicIpAddress != nil {
				output = []byte(*instance.PublicIpAddress)
			} else if instance.PrivateIpAddress != nil {
				output = []byte(*instance.PrivateIpAddress)
			} else {
				output = []byte("")
			}
		} else {
			// no [or multiple] instances found
			output = []byte("")
		}

	} else {
		// no [or multiple] instances found
		output = []byte("")
	}

	return strings.TrimSuffix(string(output), "\n")
}

// range thru ELBs of VPC
// collect list of SGs used by ELBs
// find SGs referncing those ELB SGs in rules
// delete those rules first
// delete ELB SGs
func delete_elb_instances(vpc string, cfg aws.Config) {
	var elb_sg_list []string

	elbclient := elasticloadbalancing.NewFromConfig(cfg)
	elbv2client := elasticloadbalancingv2.NewFromConfig(cfg)
	ec2client := ec2.NewFromConfig(cfg)

	elb, err := elbclient.DescribeLoadBalancers(context.TODO(), &elasticloadbalancing.DescribeLoadBalancersInput{})
	if err != nil {
		fmt.Println("Error retrieving information about classic ELBs:")
		fmt.Println(err)
		return
	}

	elbv2, err := elbv2client.DescribeLoadBalancers(context.TODO(), &elasticloadbalancingv2.DescribeLoadBalancersInput{})
	if err != nil {
		fmt.Println("Error retrieving information about v2 ELBs:")
		fmt.Println(err)
		return
	}

	// range thru loadbalancers within VPC
	// cumulate list of their security groups
	// delete ELB instance
	fmt.Printf("Deleting classic ELBs within VPC\n")
	for _, i := range elb.LoadBalancerDescriptions {
		if *i.VPCId == vpc {
			fmt.Printf("  classic ELB: %s ", *i.LoadBalancerName)
			for _, z := range i.SecurityGroups {
				fmt.Printf(" (attached SG %s)", z)
				elb_sg_list = append(elb_sg_list, z)
			}
			fmt.Printf("\n")
			wg.Add(1)
			go delete_and_wait_elb(elbclient, *i.LoadBalancerName)
		}
	}
	fmt.Printf("Deleting v2 ELBs within VPC\n")
	for _, i := range elbv2.LoadBalancers {
		if *i.VpcId == vpc {
			fmt.Printf("  v2 ELB: %s (type %s)", *i.LoadBalancerName, i.Type)
			for _, z := range i.SecurityGroups {
				fmt.Printf(" (attached SG %s)", z)
				elb_sg_list = append(elb_sg_list, z)
			}
			fmt.Printf("\n")
			wg.Add(1)
			go delete_and_wait_elbv2(elbv2client, *i.LoadBalancerArn)
		}
	}
	wg.Wait()

	//find other SGs referencing the ELB SGs
	//delete the referencing rules
	// find referencing SGs
	fmt.Println(" Deleting SG rules referencing the ELB SGs")
	sg, err := ec2client.DescribeSecurityGroups(context.TODO(), &ec2.DescribeSecurityGroupsInput{
		Filters: []types.Filter{
			{
				Name: aws.String("vpc-id"),
				Values: []string{
					vpc,
				},
			},
			{
				Name:   aws.String("ip-permission.group-id"),
				Values: elb_sg_list,
			},
		},
	})

	if err != nil {
		fmt.Println("Error retrieving SG references:")
		fmt.Println(err)
		return
	}

	// range thru referencing SGs, get their rules
	for _, ref_sg := range sg.SecurityGroups {
		//fmt.Printf("    referenced by SG %s \n",*ref_sg.GroupId)
		ref_sg_rules, err := ec2client.DescribeSecurityGroupRules(context.TODO(), &ec2.DescribeSecurityGroupRulesInput{
			Filters: []types.Filter{
				{
					Name: aws.String("group-id"),
					Values: []string{
						*ref_sg.GroupId,
					},
				},
			},
		})

		if err != nil {
			fmt.Println("Error retrieving SG rule refs:")
			fmt.Println(err)
			return
		}

		for _, ref_rule := range ref_sg_rules.SecurityGroupRules {
			if ref_rule.ReferencedGroupInfo != nil {
				//fmt.Printf("      rule %s references %s \n", *ref_rule.SecurityGroupRuleId, aws.ToString(ref_rule.ReferencedGroupInfo.GroupId))
				// if referenced rule is within elb_sg_list, delete it
				for _, v := range elb_sg_list {
					if aws.ToString(ref_rule.ReferencedGroupInfo.GroupId) == v {
						wg.Add(1)
						go delete_and_wait_sgrule(ec2client, *ref_rule.GroupId, *ref_rule.SecurityGroupRuleId, *ref_rule.IsEgress)
					}
				}
			}
		}
	}
	wg.Wait()

	fmt.Println(" Deleting ELB SGs")
	for _, v := range elb_sg_list {
		fmt.Printf("   delete SG %s \n", v)
		wg.Add(1)
		go delete_and_wait_sg(ec2client, v)
	}
	wg.Wait()
	// dont finally wait for SGs to be deleted as terraform VPC destruction will finally wait for it
}

// delete a elb instance and wait until DescribeLoadBalancer returns Error LoadBalancerNotFound
// could be moved to waiter as soon as available in aws sdk
func delete_and_wait_elb(client *elasticloadbalancing.Client, elbName string) {
	defer wg.Done()
	_, err := client.DeleteLoadBalancer(context.TODO(), &elasticloadbalancing.DeleteLoadBalancerInput{
		LoadBalancerName: aws.String(elbName),
	})
	if err != nil {
		fmt.Println("Error deleting ELB:")
		fmt.Println(err)
		return
	}

	deleted := false
	for !deleted {
		_, err := client.DescribeLoadBalancers(context.TODO(), &elasticloadbalancing.DescribeLoadBalancersInput{
			LoadBalancerNames: []string{elbName},
		})

		if err != nil {
			if strings.Contains(fmt.Sprint(err), "LoadBalancerNotFound") {
				deleted = true
			} else {
				fmt.Println("Error retrieving information about ELB deletion status:")
				fmt.Println(err)
				return
			}
		}
		if !deleted {
			fmt.Printf("   Wait 5 sec for deletion of ELB %s \n", elbName)
			time.Sleep(5 * time.Second)
		}
	}
}

// delete a elb instance and wait until DescribeLoadBalancer returns Error LoadBalancerNotFound
// could be moved to waiter as soon as available in aws sdk
func delete_and_wait_elbv2(client *elasticloadbalancingv2.Client, elbArn string) {
	defer wg.Done()
	_, err := client.DeleteLoadBalancer(context.TODO(), &elasticloadbalancingv2.DeleteLoadBalancerInput{
		LoadBalancerArn: aws.String(elbArn),
	})
	if err != nil {
		fmt.Println("Error deleting v2 ELB:")
		fmt.Println(err)
		return
	}

	deleted := false
	for !deleted {
		_, err := client.DescribeLoadBalancers(context.TODO(), &elasticloadbalancingv2.DescribeLoadBalancersInput{
			LoadBalancerArns: []string{elbArn},
		})

		if err != nil {
			if strings.Contains(fmt.Sprint(err), "LoadBalancerNotFound") {
				deleted = true
			} else {
				fmt.Println("Error retrieving information about ELB deletion status:")
				fmt.Println(err)
				return
			}
		}
		if !deleted {
			fmt.Printf("   Wait 5 sec for deletion of ELB %s \n", elbArn)
			time.Sleep(5 * time.Second)
		}
	}
}

func delete_and_wait_sg(client *ec2.Client, sgName string) {
	defer wg.Done()
	deleted := false
	for !deleted {
		_, err := client.DeleteSecurityGroup(context.TODO(), &ec2.DeleteSecurityGroupInput{
			//DryRun: aws.Bool(true),
			GroupId: aws.String(sgName),
		})

		if err != nil {
			if strings.Contains(fmt.Sprint(err), "DependencyViolation") {
				fmt.Printf("    wait 5 sec to resolve dependency violation during deletion of %s \n", sgName)
				time.Sleep(5 * time.Second)
			} else {
				fmt.Println("Error deleting SG:")
				fmt.Println(err)
				return
			}
		} else {
			deleted = true
		}
	}
}

func delete_and_wait_sgrule(client *ec2.Client, groupId string, ruleId string, isEgress bool) {
	var err error
	defer wg.Done()

	if isEgress {
		fmt.Printf("   delete %s egress rule %s \n", groupId, ruleId)
		_, err = client.RevokeSecurityGroupEgress(context.TODO(), &ec2.RevokeSecurityGroupEgressInput{
			//DryRun: aws.Bool(true),
			GroupId:              aws.String(groupId),
			SecurityGroupRuleIds: []string{ruleId},
		})
	} else {
		fmt.Printf("   delete %s ingress rule %s \n", groupId, ruleId)
		_, err = client.RevokeSecurityGroupIngress(context.TODO(), &ec2.RevokeSecurityGroupIngressInput{
			//DryRun: aws.Bool(true),
			GroupId:              aws.String(groupId),
			SecurityGroupRuleIds: []string{ruleId},
		})
	}

	if err != nil {
		fmt.Println("Error deleting SG rule:")
		fmt.Println(err)
		return
	}

	// check if security rule is deleted in API
	// to be replaced by a waiter as soon as implemented in AWS SDK
	deleted := false
	for !deleted {
		sg_rules, err := client.DescribeSecurityGroupRules(context.TODO(), &ec2.DescribeSecurityGroupRulesInput{
			Filters: []types.Filter{
				{
					Name: aws.String("group-id"),
					Values: []string{
						groupId,
					},
				},
				{
					Name: aws.String("security-group-rule-id"),
					Values: []string{
						ruleId,
					},
				},
			},
		})

		if err != nil {
			fmt.Println("Error retrieving SG rule to check deletion status:")
			fmt.Println(err)
			return
		}

		if len(sg_rules.SecurityGroupRules) == 0 {
			deleted = true
		}

		if !deleted {
			fmt.Printf("   Wait 5 sec for deletion of SG rule %s (%s) \n", ruleId, groupId)
			time.Sleep(5 * time.Second)
		}
	}
}

func start_ec2_instances(client *ec2.Client, instanceIDs []string) {
	_, err := client.StartInstances(context.TODO(), &ec2.StartInstancesInput{
		InstanceIds: instanceIDs,
	})

	if err != nil {
		fmt.Printf("error starting ec2 instances %s \n", instanceIDs)
		fmt.Println(err)
		return
	}
	fmt.Printf("Starting %d instances\n", len(instanceIDs))
}

func stop_ec2_instances(client *ec2.Client, instanceIDs []string) {
	_, err := client.StopInstances(context.TODO(), &ec2.StopInstancesInput{
		InstanceIds: instanceIDs,
	})

	if err != nil {
		fmt.Printf("error stopping ec2 instances %s \n", instanceIDs)
		fmt.Println(err)
		return
	}
	fmt.Printf("Stopping %d instances\n", len(instanceIDs))
}

func terminate_ec2_instances(client *ec2.Client, instanceIDs []string) {
	//fmt.Printf("  %s \n", instanceIDs)
	_, err := client.TerminateInstances(context.TODO(), &ec2.TerminateInstancesInput{
		InstanceIds:    instanceIDs,
		SkipOsShutdown: aws.Bool(true),
	})

	if err != nil {
		fmt.Printf("error terminating ec2 instances %s \n", instanceIDs)
		fmt.Println(err)
		return
	}
}

func wait_ec2_termination(client *ec2.Client, instanceID string, timeout_min time.Duration) {
	defer wg.Done()
	waiter := ec2.NewInstanceTerminatedWaiter(client)
	params := &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name: aws.String("instance-id"),
				Values: []string{
					instanceID,
				},
			},
			{
				Name: aws.String("instance-state-name"),
				Values: []string{
					"terminated",
				},
			},
		},
	}

	maxWaitTime := timeout_min * time.Minute
	err := waiter.Wait(context.TODO(), params, maxWaitTime)
	if err != nil {
		fmt.Printf("waiter error: %s \n", instanceID)
		fmt.Println(err)
		return
	}
}

func terminate_and_wait_nodegroup(eksclient *eks.Client, nodegroupName string, clusterName string, timeout_min time.Duration) {
	defer wg.Done()

	fmt.Printf("  %s \n", nodegroupName)
	_, err := eksclient.DeleteNodegroup(context.TODO(), &eks.DeleteNodegroupInput{
		ClusterName:   aws.String(clusterName),
		NodegroupName: aws.String(nodegroupName),
	})

	if err != nil {
		fmt.Println("error deleting EKS nodegroup:")
		fmt.Println(err)
		return
	}

	waiter := eks.NewNodegroupDeletedWaiter(eksclient)

	params := &eks.DescribeNodegroupInput{
		ClusterName:   aws.String(clusterName),
		NodegroupName: aws.String(nodegroupName),
	}

	maxWaitTime := timeout_min * time.Minute
	err = waiter.Wait(context.TODO(), params, maxWaitTime)
	if err != nil {
		fmt.Println("waiter error:", err)
		return
	}
}

func aws_show_iamkey_age(config *Config) {
	// Skip IAM key age check if using temporary credentials (session token)
	// Temporary credentials don't have a username and can't be queried via ListAccessKeys
	if config.Aws_Session_Token != "" {
		return
	}

	cfg := aws_load_config(config)
	iamclient := iam.NewFromConfig(cfg)
	iamKeys, err := iamclient.ListAccessKeys(context.TODO(), &iam.ListAccessKeysInput{})

	if err != nil {
		fmt.Printf("Error getting information about AWS IAM key age: %s \n", err.Error())
		return
	}

	for _, iamKey := range iamKeys.AccessKeyMetadata {
		if iamKey.Status == "Active" && *iamKey.AccessKeyId == config.Aws_Access_Key_Id {
			duration := time.Since(*iamKey.CreateDate)
			if math.Floor(duration.Hours()/24) > 70 {
				fmt.Printf("%sHint: your AWS IAM access key %s is older than 70 days (%.0f)\n%s", Yellow, *iamKey.AccessKeyId, math.Floor(duration.Hours()/24), Reset)
			} else {
				fmt.Printf("AWS IAM access key %s age: %.0f days\n", *iamKey.AccessKeyId, math.Floor(duration.Hours()/24))
			}
		}
	}
}
