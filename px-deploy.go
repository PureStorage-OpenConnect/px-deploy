package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/go-yaml/yaml"
	"github.com/google/uuid"
	version_hashicorp "github.com/hashicorp/go-version"
	"github.com/imdario/mergo"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

type Config struct {
	Name                     string
	Template                 string
	Cloud                    string
	Aws_Region               string
	Gcp_Region               string
	Azure_Region             string
	Platform                 string
	Clusters                 string
	Nodes                    string
	K8s_Version              string
	Px_Version               string
	Stop_After               string
	Post_Script              string
	DryRun                   bool
	NoSync                   bool
	IgnoreVersion            bool
	Lock                     bool
	Aws_Type                 string
	Aws_Ebs                  string
	Aws_Tags                 string
	Eks_Version              string
	Tags                     string
	Aws_Access_Key_Id        string
	Aws_Secret_Access_Key    string
	Gcp_Type                 string
	Gcp_Disks                string
	Gcp_Zone                 string
	Gcp_Project              string
	Gcp_Auth_Json            string
	Gke_Version              string
	Azure_Type               string
	Azure_Disks              string
	Azure_Client_Id          string
	Azure_Client_Secret      string
	Azure_Subscription_Id    string
	Azure_Tenant_Id          string
	Aks_Version              string
	Rancher_K3s_Version      string
	Rancher_K8s_Version      string
	Rancher_Version          string
	Vsphere_Host             string
	Vsphere_Compute_Resource string
	Vsphere_Resource_Pool    string
	Vsphere_User             string
	Vsphere_Password         string
	Vsphere_Template         string
	Vsphere_Datastore        string
	Vsphere_Datacenter       string
	Vsphere_Folder           string
	Vsphere_Disks            string
	Vsphere_Network          string
	Vsphere_Memory           string
	Vsphere_Cpu              string
	Vsphere_Repo             string
	Vsphere_Dns              string
	Vsphere_Gw               string
	Vsphere_Node_Ip          string
	Vsphere_Nodemap          map[string]string
	Scripts                  []string
	Description              string
	Env                      map[string]string
	Cluster                  []Config_Cluster
	Ocp4_Version             string
	Ocp4_Pull_Secret         string
	Ocp4_Domain              string
	Ocp4_Credentials_Mode    string
	Aws__Vpc                 string `yaml:"aws__vpc,omitempty"`
	Gcp__Vpc                 string `yaml:"gcp__vpc,omitempty"`
	Aws__Sg                  string `yaml:"aws__sg,omitempty"`
	Aws__Subnet              string `yaml:"aws__subnet,omitempty"`
	Aws__Gw                  string `yaml:"aws__gw,omitempty"`
	Aws__Routetable          string `yaml:"aws__routetable,omitempty"`
	Aws__Ami                 string `yaml:"aws__ami,omitempty"`
	Gcp__Project             string `yaml:"gcp__project,omitempty"`
	Gcp__Key                 string `yaml:"gcp__key,omitempty"`
	Azure__Group             string `yaml:"azure__group,omitempty"`
	Vsphere__Userdata        string `yaml:"vsphere__userdata,omitempty"`
	Ssh_Pub_Key              string
	Run_Predelete            bool
}

type Config_Cluster struct {
	Id            int
	Scripts       []string
	Instance_Type string
	Nodes         string
}

type Deployment_Status_Return struct {
	cluster int
	status  string
}

type Predelete_Status_Return struct {
	node    string
	success bool
}

var Reset = "\033[0m"
var White = "\033[97m"
var Red = "\033[31m"
var Green = "\033[32m"
var Yellow = "\033[33m"
var Blue = "\033[34m"

var testingName, testingTemplate string

var wg sync.WaitGroup

func main() {
	var createName, createTemplate, createRegion, createEnv, connectName, kubeconfigName, destroyName, statusName, historyNumber, stopName, startName string
	var destroyAll, destroyClear, destroyForce bool
	var flags Config
	os.Chdir("/px-deploy/.px-deploy")
	rootCmd := &cobra.Command{Use: "px-deploy"}

	cmdCreate := &cobra.Command{
		Use:   "create",
		Short: "Creates a deployment",
		Long:  "Creates a deployment",
		Run: func(cmd *cobra.Command, args []string) {

			if !latest_version() {
				if flags.IgnoreVersion {
					fmt.Println("ignore_version set. Please update to latest release.")
				} else {
					die("Please update to latest release. (use --ignore_version to continue)")
				}
			}
			if len(args) > 0 {
				die("Invalid arguments")
			}
			config := parse_yaml("defaults.yml")

			if flags.NoSync {
				fmt.Printf("skipping file sync from container to local dir\n")
			} else {
				sync_repository()
			}
			// should be there by default
			// we dont put in into defaults.yml as it defines a path within container
			config.Gcp_Auth_Json = "/px-deploy/.px-deploy/gcp.json"

			if config.Aws_Tags != "" {
				fmt.Printf("Parameter 'aws_tags: %s' is deprecated and will be ignored. Please change to 'tags: %s'  in ~/.px-deploy/defaults.yml \n", config.Aws_Tags, config.Aws_Tags)
			}

			check_for_recommended_settings(&config)
			prepare_error := prepare_deployment(&config, &flags, createName, createEnv, createTemplate, createRegion)
			if prepare_error != "" {
				die(prepare_error)
			} else {
				if !create_deployment(config) {
					fmt.Printf("%s creation of deployment failed %s \n", Red, Reset)
					os.Exit(1)
				}
				os.Chdir("/px-deploy/.px-deploy/infra")
				os.Setenv("deployment", config.Name)

			}
		},
	}

	cmdDestroy := &cobra.Command{
		Use:   "destroy",
		Short: "Destroys a deployment",
		Long:  "Destroys a deployment",
		Run: func(cmd *cobra.Command, args []string) {
			if destroyAll {
				if destroyClear {
					die("--clear is not supported with -a")
				}

				if destroyName != "" {
					die("Specify either -a or -n, not both")
				}
				filepath.Walk("deployments", func(file string, info os.FileInfo, err error) error {
					if info.Mode()&os.ModeDir != 0 {
						return nil
					}
					config := parse_yaml(file)
					if _, err := os.Stat("tf-deployments/" + config.Name + "/" + config.Name + ".lock"); err == nil {
						fmt.Printf("%s deployment %s is locked.\nPlease unlock using 'px-deploy unlock -n %s' first%s\n", Yellow, config.Name, config.Name, Reset)
					} else if errors.Is(err, os.ErrNotExist) {
						destroy_deployment(config.Name, destroyForce)
					} else {
						die(err.Error())
					}
					return nil
				})
			} else {
				if destroyName == "" {
					die("Must specify deployment to destroy")
				}

				if _, err := os.Stat("tf-deployments/" + destroyName + "/" + destroyName + ".lock"); err == nil {
					fmt.Printf("%s deployment %s is locked.\nPlease unlock using 'px-deploy unlock -n %s' first%s\n", Yellow, destroyName, destroyName, Reset)
				} else if errors.Is(err, os.ErrNotExist) {
					if destroyClear {
						destroy_clear(destroyName)
					} else {
						destroy_deployment(destroyName, destroyForce)
					}
				} else {
					die(err.Error())
				}

			}
		},
	}

	cmdConnect := &cobra.Command{
		Use:   "connect -n name [ command ]",
		Short: "Connects to a deployment",
		Long:  "Connects to the first master node as root, and executes optional command",
		Run: func(cmd *cobra.Command, args []string) {
			config := parse_yaml("deployments/" + connectName + ".yml")
			ip := get_ip(connectName)
			command := ""
			if len(args) > 0 {
				command = args[0]
			}
			syscall.Exec("/usr/bin/ssh", []string{"ssh", "-oLoglevel=ERROR", "-oStrictHostKeyChecking=no", "-i", "keys/id_rsa." + config.Cloud + "." + config.Name, "root@" + ip, command}, os.Environ())
		},
	}

	cmdStop := &cobra.Command{
		Use:   "stop -n name",
		Short: "Stops a deployment",
		Long:  "Stops all of the instances in a deployment",
		Run: func(cmd *cobra.Command, args []string) {
			config := parse_yaml("deployments/" + stopName + ".yml")
			if config.Cloud == "aws" {
				if config.Platform == "ocp4" || config.Platform == "eks" {
					panic("Stop not supported for " + config.Platform)
				}
				defaultConfig := parse_yaml("/px-deploy/.px-deploy/defaults.yml")
				config.Aws_Access_Key_Id = defaultConfig.Aws_Access_Key_Id
				config.Aws_Secret_Access_Key = defaultConfig.Aws_Secret_Access_Key

				cfg := aws_load_config(&config)
				client := aws_connect_ec2(&cfg)

				aws_instances, err := aws_get_instances(&config, client)
				if err != nil {
					panic(fmt.Sprintf("error listing aws instances %v \n", err.Error()))
				}
				stop_ec2_instances(client, aws_instances)
				//fmt.Printf("%v", aws_instances)
			} else {
				panic("Stop only supported on AWS")
			}
		},
	}

	cmdStart := &cobra.Command{
		Use:   "start -n name",
		Short: "Starts a deployment",
		Long:  "Starts all of the instances in a deployment",
		Run: func(cmd *cobra.Command, args []string) {
			config := parse_yaml("deployments/" + startName + ".yml")
			if config.Cloud == "aws" {
				if config.Platform == "ocp4" || config.Platform == "eks" {
					panic("Start not supported for " + config.Platform)
				}
				defaultConfig := parse_yaml("/px-deploy/.px-deploy/defaults.yml")
				config.Aws_Access_Key_Id = defaultConfig.Aws_Access_Key_Id
				config.Aws_Secret_Access_Key = defaultConfig.Aws_Secret_Access_Key

				cfg := aws_load_config(&config)
				client := aws_connect_ec2(&cfg)

				aws_instances, err := aws_get_instances(&config, client)
				if err != nil {
					panic(fmt.Sprintf("error listing aws instances %v \n", err.Error()))
				}
				start_ec2_instances(client, aws_instances)
				//fmt.Printf("%v", aws_instances)
			} else {
				panic("Stop only supported on AWS")
			}
		},
	}

	cmdKubeconfig := &cobra.Command{
		Use:   "kubeconfig -n name",
		Short: "Downloads kubeconfigs from clusters",
		Long:  "Downloads kubeconfigs from clusters",
		Run: func(cmd *cobra.Command, args []string) {
			config := parse_yaml("deployments/" + kubeconfigName + ".yml")
			ip := get_ip(kubeconfigName)
			clusters, _ := strconv.Atoi(config.Clusters)
			for c := 1; c <= clusters; c++ {
				cmd := exec.Command("bash", "-c", "ssh -oLoglevel=ERROR -oStrictHostKeyChecking=no -i keys/id_rsa."+config.Cloud+"."+config.Name+" root@"+ip+" ssh master-"+strconv.Itoa(c)+" cat /root/.kube/config")
				kubeconfig, err := cmd.Output()
				if err != nil {
					die(err.Error())
				}
				err = os.WriteFile("kubeconfig/"+config.Name+"."+strconv.Itoa(c), kubeconfig, 0644)
				if err != nil {
					die(err.Error())
				}
			}
		},
	}

	cmdList := &cobra.Command{
		Use:   "list",
		Short: "Lists available deployments",
		Long:  "Lists available deployments",
		Run: func(cmd *cobra.Command, args []string) {
			var data [][]string
			filepath.Walk("deployments", func(file string, info os.FileInfo, err error) error {
				if info.Mode()&os.ModeDir != 0 {
					return nil
				}
				if !strings.HasSuffix(file, ".yml") {
					return nil
				}
				config := parse_yaml(file)
				var region string
				switch config.Cloud {
				case "aws":
					region = config.Aws_Region
				case "gcp":
					region = config.Gcp_Region
				case "azure":
					region = config.Azure_Region
				case "vsphere":
					region = config.Vsphere_Compute_Resource
				}
				if config.Name == "" {
					config.Name = Red + "UNKNOWN" + Reset
				} else {
					if config.Template == "" {
						config.Template = "<None>"
					}
				}
				data = append(data, []string{config.Name, config.Cloud, region, config.Platform, config.Template, config.Clusters, config.Nodes, info.ModTime().Format(time.RFC3339)})

				return nil
			})
			print_table([]string{"Deployment", "Cloud", "Region", "Platform", "Template", "Clusters", "Nodes/Cl", "Created"}, data)
		},
	}

	cmdTemplates := &cobra.Command{
		Use:   "templates",
		Short: "Lists available templates",
		Long:  "Lists available templates",
		Run: func(cmd *cobra.Command, args []string) {
			list_templates()
		},
	}

	cmdUnlock := &cobra.Command{
		Use:   "unlock",
		Short: "Unlock deployment",
		Long:  "Unlock deployment",
		Run: func(cmd *cobra.Command, args []string) {
			if err := os.Remove("tf-deployments/" + statusName + "/" + statusName + ".lock"); err == nil {
				fmt.Printf("%sunlocked deployment %s\nyou can run 'px-deploy destroy -n %s' now%s\n", Green, statusName, statusName, Reset)
			} else if errors.Is(err, os.ErrNotExist) {
				fmt.Printf("%slockfile for deployment %s does not exist%s\n", Yellow, statusName, Reset)
			} else {
				panic(err)
			}

		},
	}

	cmdStatus := &cobra.Command{
		Use:   "status",
		Short: "Returns status / IP of a deployment",
		Long:  "Returns status / IP of a deployment",
		Run: func(cmd *cobra.Command, args []string) {
			config := parse_yaml("deployments/" + statusName + ".yml")
			Clusters, _ := strconv.Atoi(config.Clusters)

			clusterstatus := make(chan Deployment_Status_Return, Clusters)
			wg.Add(Clusters)
			for c := 1; c <= Clusters; c++ {
				go get_deployment_status(&config, c, clusterstatus)
			}
			wg.Wait()
			close(clusterstatus)
			m := make(map[int]string)

			for elem := range clusterstatus {
				m[elem.cluster] = elem.status
			}

			for i := 1; i <= Clusters; i++ {
				fmt.Printf(m[i])
			}
		},
	}

	cmdCompletion := &cobra.Command{
		Use:   "completion",
		Short: "Generates bash completion scripts",
		Long: `To load completion run

  . <(px-deploy completion)`,
		Run: func(cmd *cobra.Command, args []string) {
			rootCmd.GenBashCompletion(os.Stdout)
		},
	}

	cmdVsphereInit := &cobra.Command{
		Use:   "vsphere-init",
		Short: "Creates vSphere template",
		Long:  "Creates vSphere template",
		Run: func(cmd *cobra.Command, args []string) {
			vsphere_init()
		},
	}

	cmdVsphereCheckTemplateVersion := &cobra.Command{
		Use:   "vsphere-check-template",
		Short: "Checks version of vSphere template",
		Long:  "Checks version of vSphere template",
		Run: func(cmd *cobra.Command, args []string) {
			vsphere_check_templateversion(statusName)
		},
	}

	cmdVersion := &cobra.Command{
		Use:   "version",
		Short: "Displays version",
		Long:  "Displays version",
		Run: func(cmd *cobra.Command, args []string) {
			version()
		},
	}

	cmdHistory := &cobra.Command{
		Use:   "history [ -n <ID> ]",
		Short: "Displays history or inspects historical deployment",
		Long:  "Displays history or inspects historical deployment",
		Run: func(cmd *cobra.Command, args []string) {
			history(historyNumber)
		},
	}

	defaults := parse_yaml("defaults.yml")
	cmdCreate.Flags().StringVarP(&createName, "name", "n", "", "name of deployment to be created (if blank, generate UUID)")
	cmdCreate.Flags().StringVarP(&flags.Platform, "platform", "p", "", "k8s | none | ocp4 | rancher | eks | gke | aks (default "+defaults.Platform+")")
	cmdCreate.Flags().StringVarP(&flags.Clusters, "clusters", "c", "", "number of clusters to be deployed (default "+defaults.Clusters+")")
	cmdCreate.Flags().StringVarP(&flags.Nodes, "nodes", "N", "", "number of nodes to be deployed in each cluster (default "+defaults.Nodes+")")
	cmdCreate.Flags().StringVarP(&flags.K8s_Version, "k8s_version", "k", "", "Kubernetes version to be deployed (default "+defaults.K8s_Version+")")
	cmdCreate.Flags().StringVarP(&flags.Px_Version, "px_version", "P", "", "Portworx version to be deployed (default "+defaults.Px_Version+")")
	cmdCreate.Flags().StringVarP(&flags.Stop_After, "stop_after", "s", "", "Stop instances after this many hours (default "+defaults.Stop_After+")")
	cmdCreate.Flags().StringVarP(&flags.Aws_Type, "aws_type", "", "", "AWS type for each node (default "+defaults.Aws_Type+")")
	cmdCreate.Flags().StringVarP(&flags.Aws_Ebs, "aws_ebs", "", "", "space-separated list of EBS volumes to be attached to worker nodes, eg \"gp2:20 standard:30\" (default "+defaults.Aws_Ebs+")")
	cmdCreate.Flags().StringVarP(&flags.Aws_Region, "aws_region", "", "", "AWS Region (default "+defaults.Aws_Region+")")
	cmdCreate.Flags().StringVarP(&flags.Aws_Access_Key_Id, "aws_access_key_id", "", "", "your AWS API access key id (default \""+defaults.Aws_Access_Key_Id+"\")")
	cmdCreate.Flags().StringVarP(&flags.Aws_Secret_Access_Key, "aws_secret_access_key", "", "", "your AWS API secret access key (default \""+defaults.Aws_Secret_Access_Key+"\")")
	cmdCreate.Flags().StringVarP(&flags.Tags, "tags", "", "", "comma-separated list of tags to be applies to cloud nodes, eg \"Owner=Bob,Purpose=Demo\"")
	cmdCreate.Flags().StringVarP(&flags.Gcp_Type, "gcp_type", "", "", "GCP type for each node (default "+defaults.Gcp_Type+")")
	cmdCreate.Flags().StringVarP(&flags.Gcp_Project, "gcp_project", "", "", "GCP Project")
	cmdCreate.Flags().StringVarP(&flags.Gcp_Disks, "gcp_disks", "", "", "space-separated list of EBS volumes to be attached to worker nodes, eg \"pd-standard:20 pd-ssd:30\" (default "+defaults.Gcp_Disks+")")
	cmdCreate.Flags().StringVarP(&flags.Gcp_Zone, "gcp_zone", "", defaults.Gcp_Zone, "GCP zone (a, b or c)")
	cmdCreate.Flags().StringVarP(&flags.Aks_Version, "aks_version", "", "", "AKS Version (default "+defaults.Aks_Version+")")
	cmdCreate.Flags().StringVarP(&flags.Eks_Version, "eks_version", "", "", "EKS Version (default "+defaults.Eks_Version+")")
	cmdCreate.Flags().StringVarP(&flags.Azure_Type, "azure_type", "", "", "Azure type for each node (default "+defaults.Azure_Type+")")
	cmdCreate.Flags().StringVarP(&flags.Azure_Client_Secret, "azure_client_secret", "", "", "Azure Client Secret (default "+defaults.Azure_Client_Secret+")")
	cmdCreate.Flags().StringVarP(&flags.Azure_Client_Id, "azure_client_id", "", "", "Azure client ID (default "+defaults.Azure_Client_Id+")")
	cmdCreate.Flags().StringVarP(&flags.Azure_Tenant_Id, "azure_tenant_id", "", "", "Azure tenant ID (default "+defaults.Azure_Tenant_Id+")")
	cmdCreate.Flags().StringVarP(&flags.Azure_Subscription_Id, "azure_subscription_id", "", "", "Azure subscription ID (default "+defaults.Azure_Subscription_Id+")")
	cmdCreate.Flags().StringVarP(&flags.Azure_Disks, "azure_disks", "", "", "space-separated list of Azure disks to be attached to worker nodes, eg \"Standard_LRS:20 Premium_LRS:30\" (default "+defaults.Azure_Disks+")")
	cmdCreate.Flags().StringVarP(&flags.Vsphere_Disks, "vsphere_disks", "", "", "space-separated list of size of px vsphere clouddrives to be attached to worker nodes, eg \"40 50\" (default "+defaults.Vsphere_Disks+")")
	cmdCreate.Flags().StringVarP(&createTemplate, "template", "t", "", "name of template to be deployed")
	cmdCreate.Flags().StringVarP(&createRegion, "region", "r", "", "AWS, GCP or Azure region (default "+defaults.Aws_Region+", "+defaults.Gcp_Region+" or "+defaults.Azure_Region+")")
	cmdCreate.Flags().StringVarP(&flags.Cloud, "cloud", "C", "", "aws | gcp | azure | vsphere (default "+defaults.Cloud+")")
	cmdCreate.Flags().StringVarP(&flags.Ssh_Pub_Key, "ssh_pub_key", "", "", "ssh public key which will be added for root access on each node")
	cmdCreate.Flags().BoolVarP(&flags.Run_Predelete, "predelete", "", false, "run predelete scripts on destruction (true/false)")
	cmdCreate.Flags().StringVarP(&createEnv, "env", "e", "", "Comma-separated list of environment variables to be passed, for example foo=bar,abc=123")
	cmdCreate.Flags().BoolVarP(&flags.DryRun, "dry_run", "d", false, "dry-run, create local files only. Works only on aws / azure")
	cmdCreate.Flags().BoolVarP(&flags.NoSync, "no_sync", "", false, "do not sync assets/infra/scripts/templates from container to local dir, allows to change local scripts")
	cmdCreate.Flags().BoolVarP(&flags.IgnoreVersion, "ignore_version", "", false, "ignore if not running the latest px-deploy release")
	cmdCreate.Flags().BoolVarP(&flags.Lock, "lock", "", false, "protect deployment from deletion. run px-deploy unlock -n ... before deletion")
	cmdDestroy.Flags().BoolVarP(&destroyAll, "all", "a", false, "destroy all deployments")
	cmdDestroy.Flags().BoolVarP(&destroyClear, "clear", "c", false, "destroy local deployment files (use with caution!)")
	cmdDestroy.Flags().BoolVarP(&destroyForce, "force", "f", false, "destroy even if predelete script exec fails")
	cmdDestroy.Flags().StringVarP(&destroyName, "name", "n", "", "name of deployment to be destroyed")

	cmdConnect.Flags().StringVarP(&connectName, "name", "n", "", "name of deployment to connect to")
	cmdConnect.MarkFlagRequired("name")

	cmdStop.Flags().StringVarP(&stopName, "name", "n", "", "name of deployment to stop")
	cmdStop.MarkFlagRequired("name")
	cmdStart.Flags().StringVarP(&startName, "name", "n", "", "name of deployment to start")
	cmdStart.MarkFlagRequired("name")

	cmdKubeconfig.Flags().StringVarP(&kubeconfigName, "name", "n", "", "name of deployment to connect to")
	cmdKubeconfig.MarkFlagRequired("name")

	cmdStatus.Flags().StringVarP(&statusName, "name", "n", "", "name of deployment")
	cmdStatus.MarkFlagRequired("name")

	cmdTesting.Flags().StringVarP(&testingName, "name", "n", "", "name of test run")
	cmdTesting.MarkFlagRequired("name")
	cmdTesting.Flags().StringVarP(&testingTemplate, "template", "t", "", "name of template to test")
	cmdTesting.MarkFlagRequired("template")

	cmdUnlock.Flags().StringVarP(&statusName, "name", "n", "", "name of deployment")
	cmdUnlock.MarkFlagRequired("name")

	cmdVsphereCheckTemplateVersion.Flags().StringVarP(&statusName, "name", "n", "", "name of deployment")
	cmdVsphereCheckTemplateVersion.MarkFlagRequired("name")

	cmdHistory.Flags().StringVarP(&historyNumber, "number", "n", "", "deployment ID")

	rootCmd.AddCommand(cmdCreate, cmdDestroy, cmdConnect, cmdTesting, cmdKubeconfig, cmdList, cmdTemplates, cmdStop, cmdStart, cmdStatus, cmdCompletion, cmdVsphereInit, cmdVsphereCheckTemplateVersion, cmdVersion, cmdHistory, cmdUnlock)
	rootCmd.Execute()
}

func validate_config(config *Config) []string {
	var errormsg []string
	if config.Cloud != "aws" && config.Cloud != "gcp" && config.Cloud != "azure" && config.Cloud != "vsphere" {
		errormsg = append(errormsg, "Cloud must be 'aws', 'gcp', 'azure' or 'vsphere' (not '"+config.Cloud+"')")
	}

	// check parameter validity for each cloud
	switch config.Cloud {
	case "aws":

		checkvar := []string{"aws_access_key_id", "aws_secret_access_key"}
		emptyVars := isEmpty(config.Aws_Access_Key_Id, config.Aws_Secret_Access_Key)
		if len(emptyVars) > 0 {
			for _, i := range emptyVars {
				errormsg = append(errormsg, fmt.Sprintf("please set %s in defaults.yml", checkvar[i]))
			}
		}

		if (config.Ocp4_Credentials_Mode != "Mint") && (config.Ocp4_Credentials_Mode != "Passthrough") {
			config.Ocp4_Credentials_Mode = "Mint"
		}

		if !regexp.MustCompile(`^[a-zA-Z0-9_\-]+$`).MatchString(config.Aws_Region) {
			errormsg = append(errormsg, "Invalid region '"+config.Aws_Region+"'")
		}

		if !regexp.MustCompile(`^[0-9a-z\.]+$`).MatchString(config.Aws_Type) {
			errormsg = append(errormsg, "Invalid AWS type '"+config.Aws_Type+"'")
		}

		if !regexp.MustCompile(`^[0-9a-z\ :]+$`).MatchString(config.Aws_Ebs) {
			errormsg = append(errormsg, "Invalid AWS EBS volumes '"+config.Aws_Ebs+"'")
		}

		if !regexp.MustCompile(`^[0-9\.]+$`).MatchString(config.Eks_Version) {
			errormsg = append(errormsg, "Invalid EKS version '"+config.Eks_Version+"'")
		}

	case "gcp":

		checkvar := []string{"gcp_project"}
		emptyVars := isEmpty(config.Gcp_Project)
		if len(emptyVars) > 0 {
			for _, i := range emptyVars {
				errormsg = append(errormsg, fmt.Sprintf("please set %s in defaults.yml\n", checkvar[i]))
			}
		}

		if config.Gcp_Project == "" {
			errormsg = append(errormsg, "Please set gcp_project in defaults.yml")
		}

		if _, err := os.Stat("/px-deploy/.px-deploy/gcp.json"); os.IsNotExist(err) {
			errormsg = append(errormsg, "~/.px-deploy/gcp.json not found. refer to readme.md how to create it")
		}

		if !regexp.MustCompile(`^[a-zA-Z0-9_\-]+$`).MatchString(config.Gcp_Region) {
			errormsg = append(errormsg, "Invalid region '"+config.Gcp_Region+"'")
		}
		if !regexp.MustCompile(`^[0-9a-z\-]+$`).MatchString(config.Gcp_Type) {
			errormsg = append(errormsg, "Invalid GCP type '"+config.Gcp_Type+"'")
		}

		if !regexp.MustCompile(`^[0-9a-z\ :\-]+$`).MatchString(config.Gcp_Disks) {
			errormsg = append(errormsg, "Invalid GCP disks '"+config.Gcp_Disks+"'")
		}

		if config.Gcp_Zone != "a" && config.Gcp_Zone != "b" && config.Gcp_Zone != "c" {
			errormsg = append(errormsg, "Invalid GCP zone '"+config.Gcp_Zone+"'")
		}

		//if !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+`).MatchString(config.Gke_Version) {
		if !regexp.MustCompile(`^[0-9]+\.[0-9]+`).MatchString(config.Gke_Version) {
			errormsg = append(errormsg, "Invalid GKE version '"+config.Gke_Version+"'")
		}

	case "azure":
		checkvar := []string{"azure_client_id", "azure_client_secret", "azure_tenant_id", "azure_subscription_id"}
		emptyVars := isEmpty(config.Azure_Client_Id, config.Azure_Client_Secret, config.Azure_Tenant_Id, config.Azure_Subscription_Id)
		if len(emptyVars) > 0 {
			for _, i := range emptyVars {
				errormsg = append(errormsg, fmt.Sprintf("please set %s in defaults.yml ", checkvar[i]))
			}
		}

		if !regexp.MustCompile(`^[a-zA-Z0-9_\-]+$`).MatchString(config.Azure_Region) {
			errormsg = append(errormsg, "Invalid region '"+config.Azure_Region+"'")
		}
		if !regexp.MustCompile(`^[0-9\.]+$`).MatchString(config.Aks_Version) {
			errormsg = append(errormsg, "Invalid AKS version '"+config.Aks_Version+"'")
		}
		if !regexp.MustCompile(`^[0-9a-zA-Z\-\_]+$`).MatchString(config.Azure_Type) {
			errormsg = append(errormsg, "Invalid Azure type '"+config.Azure_Type+"'")
		}
		if !regexp.MustCompile(`^[0-9a-zA-Z\ \_:]+$`).MatchString(config.Azure_Disks) {
			errormsg = append(errormsg, "Invalid Azure disks '"+config.Azure_Disks+"'")
		}
	case "vsphere":
		checkvar := []string{"vsphere_compute_resource", "vsphere_datacenter", "vsphere_datastore", "vsphere_host", "vsphere_network", "vsphere_resource_pool", "vsphere_template", "vsphere_user", "vsphere_password", "vsphere_repo"}
		emptyVars := isEmpty(config.Vsphere_Compute_Resource, config.Vsphere_Datacenter, config.Vsphere_Datastore, config.Vsphere_Host, config.Vsphere_Network, config.Vsphere_Resource_Pool, config.Vsphere_Template, config.Vsphere_User, config.Vsphere_Password, config.Vsphere_Repo)
		if len(emptyVars) > 0 {
			for _, i := range emptyVars {
				errormsg = append(errormsg, fmt.Sprintf("please set %s in defaults.yml ", checkvar[i]))
			}
		}

		config.Vsphere_Template = strings.TrimLeft(config.Vsphere_Template, "/")
		config.Vsphere_Folder = strings.TrimLeft(config.Vsphere_Folder, "/")
		config.Vsphere_Folder = strings.TrimRight(config.Vsphere_Folder, "/")
		if !regexp.MustCompile(`^[0-9\ ]+$`).MatchString(config.Vsphere_Disks) {
			errormsg = append(errormsg, "Invalid vsphere_disks option '"+config.Vsphere_Disks+"'")
		}

	}

	if config.Platform != "k8s" && config.Platform != "none" && config.Platform != "ocp4" && config.Platform != "rancher" && config.Platform != "eks" && config.Platform != "gke" && config.Platform != "aks" {
		errormsg = append(errormsg, "Invalid platform '"+config.Platform+"'")
	}

	if !regexp.MustCompile(`^[0-9]+$`).MatchString(config.Clusters) {
		errormsg = append(errormsg, "Invalid number of clusters")
	}

	if !regexp.MustCompile(`^[0-9]+$`).MatchString(config.Nodes) {
		errormsg = append(errormsg, "Invalid number of nodes")
	}

	if !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(config.K8s_Version) {
		errormsg = append(errormsg, "Invalid Kubernetes version '"+config.K8s_Version+"'")
	}

	if !regexp.MustCompile(`^[0-9\.]+$`).MatchString(config.Px_Version) {
		errormsg = append(errormsg, "Invalid Portworx version '"+config.Px_Version+"'")
	}

	if !regexp.MustCompile(`^[0-9]+$`).MatchString(config.Stop_After) {
		errormsg = append(errormsg, "Invalid number of hours")
	}

	if !regexp.MustCompile(`^((([\p{L}\p{Z}\p{N}_.:+\-]*)=([\p{L}\p{Z}\p{N}_.:+\-]*),)*(([\p{L}\p{Z}\p{N}_.:+\-]*)=([\p{L}\p{Z}\p{N}_.:+\-]*)){1})*$`).MatchString(config.Tags) {
		errormsg = append(errormsg, "Invalid tags '"+config.Tags+"'")
	}

	for _, c := range config.Cluster {
		for _, s := range c.Scripts {
			if _, err := os.Stat("scripts/" + s); os.IsNotExist(err) {
				errormsg = append(errormsg, "Script '"+s+"' does not exist")
			}
			cmd := exec.Command("bash", "-n", "scripts/"+s)
			err := cmd.Run()
			if err != nil {
				errormsg = append(errormsg, "Script '"+s+"' is not valid Bash")
			}
		}
	}
	for _, s := range config.Scripts {
		if _, err := os.Stat("scripts/" + s); os.IsNotExist(err) {
			errormsg = append(errormsg, "Script '"+s+"' does not exist")
		}
		cmd := exec.Command("bash", "-n", "scripts/"+s)
		err := cmd.Run()
		if err != nil {
			errormsg = append(errormsg, "Script '"+s+"' is not valid Bash")
		}
	}
	if config.Post_Script != "" {
		if _, err := os.Stat("scripts/" + config.Post_Script); os.IsNotExist(err) {
			errormsg = append(errormsg, "Postscript '"+config.Post_Script+"' does not exist")
		}
		cmd := exec.Command("bash", "-n", "scripts/"+config.Post_Script)
		err := cmd.Run()
		if err != nil {
			errormsg = append(errormsg, "Postscript '"+config.Post_Script+"' is not valid Bash")
		}
	}

	if config.Platform == "ocp4" {
		checkvar := []string{"ocp4_domain", "ocp4_pull_secret"}
		emptyVars := isEmpty(config.Ocp4_Domain, config.Ocp4_Pull_Secret)
		if len(emptyVars) > 0 {
			for _, i := range emptyVars {
				errormsg = append(errormsg, fmt.Sprintf("please set \"%s\" in defaults.yml", checkvar[i]))
			}
		}
	}

	if config.Platform == "rancher" {
		checkvar := []string{"rancher_k3s_version", "rancher_k8s_version", "rancher_version"}
		emptyVars := isEmpty(config.Rancher_K3s_Version, config.Rancher_K8s_Version, config.Rancher_Version)
		if len(emptyVars) > 0 {
			for _, i := range emptyVars {
				errormsg = append(errormsg, fmt.Sprintf("please set \"%s\" in defaults.yml", checkvar[i]))
			}
		}
	}

	if config.Platform == "eks" && !(config.Cloud == "aws") {
		errormsg = append(errormsg, "EKS only makes sense with AWS (not "+config.Cloud+")")
	}
	if config.Platform == "ocp4" && config.Cloud != "aws" {
		errormsg = append(errormsg, "Openshift 4 only supported on AWS (not "+config.Cloud+")")
	}
	if config.Platform == "rancher" && config.Cloud != "aws" {
		errormsg = append(errormsg, "Rancher only supported on AWS (not "+config.Cloud+")")
	}
	if config.Platform == "gke" && config.Cloud != "gcp" {
		errormsg = append(errormsg, "GKE only makes sense with GCP (not "+config.Cloud+")")
	}
	if config.Platform == "aks" && config.Cloud != "azure" {
		errormsg = append(errormsg, "AKS only makes sense with Azure (not "+config.Cloud+")")
	}

	return errormsg
}

// merge defaults.yml / template settings / flag parameters and create deployment.yml file
func prepare_deployment(config *Config, flags *Config, createName string, createEnv string, createTemplate string, createRegion string) string {
	var env_template map[string]string

	env := config.Env

	if createTemplate != "" {
		config.Template = createTemplate
		config_template := parse_yaml("templates/" + createTemplate + ".yml")
		env_template = config_template.Env
		mergo.MergeWithOverwrite(config, config_template)
		mergo.MergeWithOverwrite(&env, env_template)
	}

	if createEnv != "" {
		env_flags := make(map[string]string)
		for _, kv := range strings.Split(createEnv, ",") {
			s := strings.Split(kv, "=")
			env_flags[s[0]] = s[1]
		}
		mergo.MergeWithOverwrite(&flags.Env, env_flags)
	}
	config.Env = env

	mergo.MergeWithOverwrite(config, flags)
	configerr := validate_config(config)
	if configerr != nil {
		var validate_error []byte
		validate_error = append(validate_error, fmt.Sprint(Red)...)
		validate_error = append(validate_error, fmt.Sprintf("Found %v errors in config \n", len(configerr))...)
		for elem := range configerr {
			validate_error = append(validate_error, fmt.Sprintln(configerr[elem])...)
		}
		validate_error = append(validate_error, fmt.Sprint(Reset)...)
		return string(validate_error)
	}

	if createName != "" {
		if !regexp.MustCompile(`^[a-z0-9_\-\.]+$`).MatchString(createName) {
			return fmt.Sprintf("Invalid deployment name %s\n", createName)
		}
		if _, err := os.Stat("deployments/" + createName + ".yml"); !os.IsNotExist(err) {
			return fmt.Sprintf("%sDeployment '%s' already exists%s\n Please delete it by running 'px-deploy destroy -n %s' \n If this fails, remove cloud resources manually and run 'px-deploy destroy --clear -n %s'\n", Red, createName, Reset, createName, createName)
		}
	} else {
		createName = uuid.New().String()
	}
	config.Name = createName

	if createRegion != "" {
		switch config.Cloud {
		case "aws":
			config.Aws_Region = createRegion
		case "gcp":
			config.Gcp_Region = createRegion
		case "azure":
			config.Azure_Region = createRegion
		default:
			return fmt.Sprintf("setting cloud region not supported on %s\n", config.Cloud)
		}
	}

	// remove AWS credentials from deployment specific yml as we should not rely on it later
	cleanConfig := *config
	if cleanConfig.Cloud == "aws" {
		cleanConfig.Aws_Access_Key_Id = ""
		cleanConfig.Aws_Secret_Access_Key = ""
	}
	y, _ := yaml.Marshal(cleanConfig)

	log("[ " + strings.Join(os.Args[1:], " ") + " ] " + base64.StdEncoding.EncodeToString(y))

	err := os.WriteFile("deployments/"+createName+".yml", y, 0644)
	if err != nil {
		return (err.Error())
	}
	return ""
}

func get_deployment_status(config *Config, cluster int, c chan Deployment_Status_Return) {
	defer wg.Done()
	var ip string
	var Nodes int
	var returnvalue string

	if (config.Platform == "ocp4") || (config.Platform == "rancher") || (config.Platform == "eks") || (config.Platform == "aks") || (config.Platform == "gke") {
		Nodes = 0
	} else {
		Nodes, _ = strconv.Atoi(config.Nodes)
		//check for cluster specific node # overrides
		for _, cl_entry := range config.Cluster {
			if (cl_entry.Id == cluster) && (cl_entry.Nodes != "") {
				Nodes, _ = strconv.Atoi(cl_entry.Nodes)
			}
		}
	}

	switch config.Cloud {
	case "aws":
		ip = aws_get_node_ip(config.Name, fmt.Sprintf("master-%v-1", cluster))
	case "azure":
		ip = azure_get_node_ip(config.Name, fmt.Sprintf("master-%v-1", cluster))
	case "gcp":
		ip = gcp_get_node_ip(config.Name, fmt.Sprintf("%v-master-%v-1", config.Name, cluster))
	case "vsphere":
		ip = vsphere_get_node_ip(config, fmt.Sprintf("%v-master-%v", config.Name, cluster))
	}
	// get content of node tracking file (-> each node will add its entry when finished cloud-init/infra scripts)
	cmd := exec.Command("ssh", "-q", "-oStrictHostKeyChecking=no", "-i", "keys/id_rsa."+config.Cloud+"."+config.Name, "root@"+ip, "cat /var/log/px-deploy/completed/tracking")
	out, err := cmd.CombinedOutput()
	if err != nil {
		returnvalue = fmt.Sprintf("Error get status of cluster %v\n Message: %v\n", cluster, err.Error())
		//c <- returnvalue
	} else {
		scanner := bufio.NewScanner(strings.NewReader(string(out)))
		ready_nodes := make(map[string]string)
		for scanner.Scan() {
			entry := strings.Fields(scanner.Text())
			ready_nodes[entry[0]] = entry[1]
		}
		if ready_nodes[fmt.Sprintf("master-%v", cluster)] != "" {
			returnvalue = fmt.Sprintf("%vReady\tmaster-%v \t  %v\n", returnvalue, cluster, ip)
		} else {
			returnvalue = fmt.Sprintf("%vNotReady\tmaster-%v \t (%v)\n", returnvalue, cluster, ip)
		}
		if config.Platform == "ocp4" {
			if ready_nodes["url"] != "" {
				returnvalue = fmt.Sprintf("%v  URL: %v \n", returnvalue, ready_nodes["url"])
			} else {
				returnvalue = fmt.Sprintf("%v  OCP4 URL not yet available\n", returnvalue)
			}
			if ready_nodes["cred"] != "" {
				returnvalue = fmt.Sprintf("%v  Credentials: kubeadmin / %v \n", returnvalue, ready_nodes["cred"])
			} else {
				returnvalue = fmt.Sprintf("%v  OCP4 credentials not yet available\n", returnvalue)
			}
		}

		if config.Platform == "rancher" {
			if ready_nodes["url"] != "" {
				returnvalue = fmt.Sprintf("%v  URL: %v \n", returnvalue, ready_nodes["url"])
			} else {
				returnvalue = fmt.Sprintf("%v  Rancher Server URL not yet available\n", returnvalue)
			}
			if ready_nodes["cred"] != "" {
				returnvalue = fmt.Sprintf("%v  Credentials: admin / %v \n", returnvalue, ready_nodes["cred"])
			} else {
				returnvalue = fmt.Sprintf("%v  Rancher Server credentials not yet available\n", returnvalue)
			}
		}
		for n := 1; n <= Nodes; n++ {
			if ready_nodes[fmt.Sprintf("node-%v-%v", cluster, n)] != "" {
				returnvalue = fmt.Sprintf("%vReady\t node-%v-%v\n", returnvalue, cluster, n)
			} else {
				returnvalue = fmt.Sprintf("%vNotReady\t node-%v-%v\n", returnvalue, cluster, n)
			}
		}
	}
	c <- Deployment_Status_Return{cluster, returnvalue}
}

func create_deployment(config Config) bool {
	var err error

	fmt.Println(White + "Provisioning infrastructure..." + Reset)
	switch config.Cloud {
	case "aws":
		{
			// create directory for deployment and copy terraform scripts
			err = os.Mkdir("/px-deploy/.px-deploy/tf-deployments/"+config.Name, 0755)
			if err != nil {
				fmt.Println(err.Error())
				return false
			}
			//maybe there is a better way to copy templates to working dir ?
			exec.Command("cp", "-a", `/px-deploy/terraform/aws/main.tf`, `/px-deploy/.px-deploy/tf-deployments/`+config.Name).Run()
			exec.Command("cp", "-a", `/px-deploy/terraform/aws/variables.tf`, `/px-deploy/.px-deploy/tf-deployments/`+config.Name).Run()
			exec.Command("cp", "-a", `/px-deploy/terraform/aws/cloud-init.tpl`, `/px-deploy/.px-deploy/tf-deployments/`+config.Name).Run()
			exec.Command("cp", "-a", `/px-deploy/terraform/aws/aws-returns.tpl`, `/px-deploy/.px-deploy/tf-deployments/`+config.Name).Run()
			// also copy terraform modules
			//exec.Command("cp", "-a", `/px-deploy/terraform/aws/.terraform`, `/px-deploy/.px-deploy/tf-deployments/`+config.Name).Run()
			// creating symlink for .terraform as performance on mac significantly improves when not on bind mount issue #397
			exec.Command("ln", "-s", `/px-deploy/terraform/aws/.terraform`, `/px-deploy/.px-deploy/tf-deployments/`+config.Name+`/.terraform`).Run()
			exec.Command("cp", "-a", `/px-deploy/terraform/aws/.terraform.lock.hcl`, `/px-deploy/.px-deploy/tf-deployments/`+config.Name).Run()

			switch config.Platform {
			case "ocp4":
				{
					exec.Command("cp", "-a", `/px-deploy/terraform/aws/ocp4/ocp4.tf`, `/px-deploy/.px-deploy/tf-deployments/`+config.Name).Run()
					exec.Command("cp", "-a", `/px-deploy/terraform/aws/ocp4/ocp4-install-config.tpl`, `/px-deploy/.px-deploy/tf-deployments/`+config.Name).Run()
				}
			case "eks":
				{
					exec.Command("cp", "-a", `/px-deploy/terraform/aws/eks/eks.tf`, `/px-deploy/.px-deploy/tf-deployments/`+config.Name).Run()
					exec.Command("cp", "-a", `/px-deploy/terraform/aws/eks/eks_run_everywhere.tpl`, `/px-deploy/.px-deploy/tf-deployments/`+config.Name).Run()
				}
			case "rancher":
				{
					exec.Command("cp", "-a", `/px-deploy/terraform/aws/rancher/rancher-server.tf`, `/px-deploy/.px-deploy/tf-deployments/`+config.Name).Run()
					exec.Command("cp", "-a", `/px-deploy/terraform/aws/rancher/rancher-variables.tf`, `/px-deploy/.px-deploy/tf-deployments/`+config.Name).Run()
				}
			}
			write_nodescripts(config)
			write_tf_file(config.Name, ".tfvars", aws_create_variables(&config))
			tf_error := run_terraform_apply(&config)
			if tf_error != "" {
				fmt.Printf("%s\n", tf_error)
				return false
			}
			aws_show_iamkey_age(&config)
		}
	case "gcp":
		{
			// create directory for deployment and copy terraform scripts
			err = os.Mkdir("/px-deploy/.px-deploy/tf-deployments/"+config.Name, 0755)
			if err != nil {
				fmt.Println(err.Error())
				return false
			}
			//maybe there is a better way to copy templates to working dir ?
			exec.Command("cp", "-a", `/px-deploy/terraform/gcp/main.tf`, `/px-deploy/.px-deploy/tf-deployments/`+config.Name).Run()
			exec.Command("cp", "-a", `/px-deploy/terraform/gcp/variables.tf`, `/px-deploy/.px-deploy/tf-deployments/`+config.Name).Run()
			exec.Command("cp", "-a", `/px-deploy/terraform/gcp/startup-script.tpl`, `/px-deploy/.px-deploy/tf-deployments/`+config.Name).Run()
			exec.Command("cp", "-a", `/px-deploy/terraform/gcp/gcp-returns.tpl`, `/px-deploy/.px-deploy/tf-deployments/`+config.Name).Run()

			// also copy terraform modules
			//exec.Command("cp", "-a", `/px-deploy/terraform/gcp/.terraform`, `/px-deploy/.px-deploy/tf-deployments/`+config.Name).Run()
			// creating symlink for .terraform as performance on mac significantly improves when not on bind mount issue #397
			exec.Command("ln", "-s", `/px-deploy/terraform/gcp/.terraform`, `/px-deploy/.px-deploy/tf-deployments/`+config.Name+`/.terraform`).Run()
			exec.Command("cp", "-a", `/px-deploy/terraform/gcp/.terraform.lock.hcl`, `/px-deploy/.px-deploy/tf-deployments/`+config.Name).Run()

			switch config.Platform {
			case "gke":
				{
					exec.Command("cp", "-a", `/px-deploy/terraform/gcp/gke/gke.tf`, `/px-deploy/.px-deploy/tf-deployments/`+config.Name).Run()
				}
			}

			write_nodescripts(config)

			write_tf_file(config.Name, ".tfvars", gcp_create_variables(&config))
			tf_error := run_terraform_apply(&config)
			if tf_error != "" {
				fmt.Printf("%s\n", tf_error)
				return false
			}
		}
	case "azure":
		{
			// create directory for deployment and copy terraform scripts
			err = os.Mkdir("/px-deploy/.px-deploy/tf-deployments/"+config.Name, 0755)
			if err != nil {
				fmt.Println(err.Error())
				return false
			}
			//maybe there is a better way to copy templates to working dir ?
			exec.Command("cp", "-a", `/px-deploy/terraform/azure/main.tf`, `/px-deploy/.px-deploy/tf-deployments/`+config.Name).Run()
			exec.Command("cp", "-a", `/px-deploy/terraform/azure/variables.tf`, `/px-deploy/.px-deploy/tf-deployments/`+config.Name).Run()
			exec.Command("cp", "-a", `/px-deploy/terraform/azure/cloud-init.tpl`, `/px-deploy/.px-deploy/tf-deployments/`+config.Name).Run()
			// also copy terraform modules
			//exec.Command("cp", "-a", `/px-deploy/terraform/azure/.terraform`, `/px-deploy/.px-deploy/tf-deployments/`+config.Name).Run()
			// creating symlink for .terraform as performance on mac significantly improves when not on bind mount issue #397
			exec.Command("ln", "-s", `/px-deploy/terraform/azure/.terraform`, `/px-deploy/.px-deploy/tf-deployments/`+config.Name+`/.terraform`).Run()
			exec.Command("cp", "-a", `/px-deploy/terraform/azure/.terraform.lock.hcl`, `/px-deploy/.px-deploy/tf-deployments/`+config.Name).Run()

			switch config.Platform {
			case "aks":
				{
					exec.Command("cp", "-a", `/px-deploy/terraform/azure/aks/aks.tf`, `/px-deploy/.px-deploy/tf-deployments/`+config.Name).Run()
					//exec.Command("cp", "-a", `/px-deploy/terraform/azure/aks/aks_run_everywhere.tpl`,`/px-deploy/.px-deploy/tf-deployments/`+ config.Name).Run()
				}
			}
			write_nodescripts(config)

			write_tf_file(config.Name, ".tfvars", azure_create_variables(&config))

			tf_error := run_terraform_apply(&config)
			if tf_error != "" {
				fmt.Printf("%s\n", tf_error)
				return false
			}
		}
	case "vsphere":
		{
			// create directory for deployment and copy terraform scripts
			err = os.Mkdir("/px-deploy/.px-deploy/tf-deployments/"+config.Name, 0755)
			if err != nil {
				fmt.Println(err.Error())
				return false
			}
			//maybe there is a better way to copy templates to working dir ?
			exec.Command("cp", "-a", `/px-deploy/terraform/vsphere/main.tf`, `/px-deploy/.px-deploy/tf-deployments/`+config.Name).Run()
			exec.Command("cp", "-a", `/px-deploy/terraform/vsphere/variables.tf`, `/px-deploy/.px-deploy/tf-deployments/`+config.Name).Run()
			exec.Command("cp", "-a", `/px-deploy/terraform/vsphere/cloud-init.tpl`, `/px-deploy/.px-deploy/tf-deployments/`+config.Name).Run()
			exec.Command("cp", "-a", `/px-deploy/terraform/vsphere/metadata.tpl`, `/px-deploy/.px-deploy/tf-deployments/`+config.Name).Run()

			// also copy terraform modules
			//exec.Command("cp", "-a", `/px-deploy/terraform/gcp/.terraform`, `/px-deploy/.px-deploy/tf-deployments/`+config.Name).Run()
			// creating symlink for .terraform as performance on mac significantly improves when not on bind mount issue #397
			exec.Command("ln", "-s", `/px-deploy/terraform/vsphere/.terraform`, `/px-deploy/.px-deploy/tf-deployments/`+config.Name+`/.terraform`).Run()
			exec.Command("cp", "-a", `/px-deploy/terraform/vsphere/.terraform.lock.hcl`, `/px-deploy/.px-deploy/tf-deployments/`+config.Name).Run()

			write_nodescripts(config)

			write_tf_file(config.Name, ".tfvars", vsphere_create_variables(&config))
			tf_error := run_terraform_apply(&config)
			if tf_error != "" {
				fmt.Printf("%s\n", tf_error)
				return false
			}
			vsphere_check_templateversion(config.Name)

		}
	default:
		fmt.Println("Invalid cloud '" + config.Cloud + "'")
		return false
	}

	if config.Lock {
		lockfile, err := os.OpenFile("tf-deployments/"+config.Name+"/"+config.Name+".lock", os.O_RDONLY|os.O_CREATE, 0644)
		if err != nil {
			panic(err.Error())
		}
		lockfile.Close()
	}
	return true
}

func run_terraform_apply(config *Config) string {
	var cloud_auth []string
	fmt.Println(White + "running terraform PLAN" + Reset)
	cmd := exec.Command("terraform", "-chdir=/px-deploy/.px-deploy/tf-deployments/"+config.Name, "plan", "-input=false", "-parallelism=50", "-out=tfplan", "-var-file", ".tfvars")
	cmd.Stderr = os.Stderr

	switch config.Cloud {
	case "aws":
		cloud_auth = append(cloud_auth, fmt.Sprintf("AWS_ACCESS_KEY_ID=%s", config.Aws_Access_Key_Id))
		cloud_auth = append(cloud_auth, fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", config.Aws_Secret_Access_Key))
		// make aws keys consumeable within the terraform scripts
		cloud_auth = append(cloud_auth, fmt.Sprintf("TF_VAR_AWS_ACCESS_KEY_ID=%s", config.Aws_Access_Key_Id))
		cloud_auth = append(cloud_auth, fmt.Sprintf("TF_VAR_AWS_SECRET_ACCESS_KEY=%s", config.Aws_Secret_Access_Key))
	}
	cmd.Env = append(cmd.Env, cloud_auth...)
	err := cmd.Run()
	if err != nil {
		fmt.Println(Yellow + "ERROR: terraform plan failed. Check validity of terraform scripts" + Reset)
		return err.Error()
	} else {
		if config.DryRun {
			fmt.Printf("Dry run only. No deployment on target cloud. Run 'px-deploy destroy -n %s' to remove local files\n", config.Name)
			return ""
		}
		fmt.Println(White + "running terraform APPLY" + Reset)
		cmd := exec.Command("terraform", "-chdir=/px-deploy/.px-deploy/tf-deployments/"+config.Name, "apply", "-input=false", "-parallelism=50", "-auto-approve", "tfplan")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = append(cmd.Env, cloud_auth...)
		errapply := cmd.Run()
		if errapply != nil {
			fmt.Println(Yellow + "ERROR: terraform apply failed. Check validity of terraform scripts" + Reset)
			return errapply.Error()
		}

		switch config.Cloud {
		case "aws":
			// apply the terraform aws-returns-generated to deployment yml file (maintains compatibility to px-deploy behaviour, maybe not needed any longer)
			content, err := os.ReadFile("/px-deploy/.px-deploy/tf-deployments/" + config.Name + "/aws-returns-generated.yaml")
			if err != nil {
				return err.Error()
			}

			file, err := os.OpenFile("/px-deploy/.px-deploy/deployments/"+config.Name+".yml", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return err.Error()
			}
			defer file.Close()
			_, err = file.WriteString(string(content))
			if err != nil {
				return err.Error()
			}
		case "gcp":
			// apply the terraform gcp-returns-generated to deployment yml file (network name needed for different functions)
			content, err := os.ReadFile("/px-deploy/.px-deploy/tf-deployments/" + config.Name + "/gcp-returns-generated.yaml")
			if err != nil {
				return err.Error()
			}
			file, err := os.OpenFile("/px-deploy/.px-deploy/deployments/"+config.Name+".yml", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return err.Error()
			}
			defer file.Close()
			_, err = file.WriteString(string(content))
			if err != nil {
				return err.Error()
			}
		case "vsphere":
			// apply the terraform nodemap.txt to deployment yml file (list of VM name / VM mobId / mac address)
			readFile, err := os.Open("/px-deploy/.px-deploy/tf-deployments/" + config.Name + "/nodemap.txt")
			if err != nil {
				return err.Error()
			}
			fileScanner := bufio.NewScanner(readFile)
			fileScanner.Split(bufio.ScanLines)
			var fileLines []string
			fileLines = append(fileLines, "vsphere_nodemap:\n")

			for fileScanner.Scan() {
				fileLines = append(fileLines, fmt.Sprintf("  %s\n", strings.TrimSpace(fileScanner.Text())))
			}
			readFile.Close()

			file, err := os.OpenFile("/px-deploy/.px-deploy/deployments/"+config.Name+".yml", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return err.Error()
			}
			defer file.Close()

			for _, line := range fileLines {
				_, err = file.WriteString(line)
				if err != nil {
					return err.Error()
				}
			}
		}
		fmt.Println(Yellow + "Terraform infrastructure creation done. Please check master/node readiness/credentials using: px-deploy status -n " + config.Name + Reset)
	}
	return ""
}

func destroy_clear(name string) {
	os.Chdir("/px-deploy/.px-deploy")
	config := parse_yaml("deployments/" + name + ".yml")

	fmt.Println(Yellow + "Deleting local files for deployment " + name)
	fmt.Println("If there is any deployment in cloud " + config.Cloud + " you need to remove manually now" + Reset)

	os.Chdir("/px-deploy/.px-deploy")
	os.Remove("deployments/" + name + ".yml")
	os.RemoveAll("tf-deployments/" + name)
	os.Remove("keys/id_rsa." + config.Cloud + "." + name)
	os.Remove("keys/id_rsa." + config.Cloud + "." + name + ".pub")

	clusters, _ := strconv.Atoi(config.Clusters)
	for c := 0; c <= clusters; c++ {
		os.Remove("kubeconfig/" + name + "." + strconv.Itoa(c))
	}
}

func destroy_deployment(name string, destroyForce bool) {
	os.Chdir("/px-deploy/.px-deploy")
	config := parse_yaml("deployments/" + name + ".yml")
	var output []byte

	fmt.Println(White + "Destroying deployment '" + config.Name + "'..." + Reset)
	c, _ := strconv.Atoi(config.Clusters)

	if _, err := os.Stat("/px-deploy/.px-deploy/logs"); os.IsNotExist(err) {
		err := os.Mkdir("/px-deploy/.px-deploy/logs", 0755)
		if err != nil {
			die("Cannot create directory ~/.px-deploy/logs : " + err.Error())
		}
	}

	logdir := "/px-deploy/.px-deploy/logs/" + name + "_" + time.Now().Format(time.RFC3339)
	err := os.Mkdir(logdir, 0755)
	if err != nil {
		die("Cannot create directory " + logdir + ": " + err.Error())
	}
	cmd := exec.Command("bash", "-c", "(cd logs ; ls -t | tail -n +11 | xargs -n 1 rm -rf)")
	_, err = cmd.Output()
	if err != nil {
		die("Cannot purge old logs: " + err.Error())
	}
	for i := 1; i <= c; i++ {
		fmt.Println(White + "Collecting logs for cluster " + strconv.Itoa(i) + "..." + Reset)
		err = os.Mkdir(logdir+"/"+strconv.Itoa(i), 0755)
		if err != nil {
			die("Cannot create directory " + logdir + "/" + strconv.Itoa(i) + ": " + err.Error())
		}
		cmd := exec.Command("bash", "-c", "rsync -rtz -e 'ssh -oLoglevel=ERROR -oStrictHostKeyChecking=no -i keys/id_rsa."+config.Cloud+"."+name+" root@"+get_ip(name)+" ssh' master-"+strconv.Itoa(i)+":/var/log/px-deploy/ "+logdir+"/"+strconv.Itoa(i))
		_, err := cmd.Output()
		if err != nil {
			fmt.Println("Failed to collect logs: " + err.Error())
		}
	}

	if config.Cloud == "aws" {
		defaultConfig := parse_yaml("/px-deploy/.px-deploy/defaults.yml")
		config.Aws_Access_Key_Id = defaultConfig.Aws_Access_Key_Id
		config.Aws_Secret_Access_Key = defaultConfig.Aws_Secret_Access_Key

		cfg := aws_load_config(&config)
		client := aws_connect_ec2(&cfg)

		aws_instances, err := aws_get_instances(&config, client)
		if err != nil {
			panic(fmt.Sprintf("error listing aws instances %v \n", err.Error()))
		}

		// split aws_instances into chunks of 196 elements
		// because of the aws DescribeVolumes Filter limit of 200 (196 instances + pxtype: data/kvdb/journal/metadata)
		// build slice of slices
		aws_instances_split := make([]([]string), len(aws_instances)/196+1)
		for i, val := range aws_instances {
			aws_instances_split[i/196] = append(aws_instances_split[i/196], val)
		}

		aws_volumes, err := aws_get_clouddrives(aws_instances_split, client)
		if err != nil {
			panic(fmt.Sprintf("error listing aws clouddrives %v \n", err.Error()))
		}

		fmt.Printf("Found %d portworx clouddrive volumes. \n", len(aws_volumes))

		switch config.Platform {
		case "ocp4":
			{
				prepare_predelete(&config, "script", destroyForce)
				prepare_predelete(&config, "platform", destroyForce)
			}
		case "rancher":
			{
				var cloud_auth []string
				cloud_auth = append(cloud_auth, fmt.Sprintf("AWS_ACCESS_KEY_ID=%s", config.Aws_Access_Key_Id))
				cloud_auth = append(cloud_auth, fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", config.Aws_Secret_Access_Key))
				fmt.Println(Red + "FIXME: removing helm deployments from rancher state." + Reset)
				cmd1 := exec.Command("terraform", "-chdir=/px-deploy/.px-deploy/tf-deployments/"+config.Name, "state", "rm", "helm_release.rancher_server")
				cmd1.Stdout = os.Stdout
				cmd1.Stderr = os.Stderr
				cmd1.Env = append(cmd1.Env, cloud_auth...)
				errstate1 := cmd1.Run()
				if errstate1 != nil {
					fmt.Println(Yellow + "ERROR: Terraform state rm helm_release.rancher_server failed. Check validity of terraform scripts" + Reset)
				}
				cmd2 := exec.Command("terraform", "-chdir=/px-deploy/.px-deploy/tf-deployments/"+config.Name, "state", "rm", "helm_release.cert_manager")
				cmd2.Stdout = os.Stdout
				cmd2.Stderr = os.Stderr
				cmd2.Env = append(cmd2.Env, cloud_auth...)
				errstate2 := cmd2.Run()
				if errstate2 != nil {
					fmt.Println(Yellow + "ERROR: Terraform state rm helm_release.cert_manager failed. Check validity of terraform scripts" + Reset)
				}

			}
		case "eks":
			{
				prepare_predelete(&config, "script", destroyForce)

				err := aws_delete_nodegroups(&config)
				if err != nil {
					fmt.Printf("%v error deleting eks nodegroups \n %v \n You might need to remove PX clouddrives manually %v\n", Red, err.Error(), Reset)
				}

			}
		default:
			{
				// if there are no px clouddrive volumes
				// terraform will terminate instances
				// otherwise terminate instances to enable volume deletion

				prepare_predelete(&config, "script", destroyForce)

				if len(aws_volumes) > 0 {
					fmt.Printf("Waiting for termination of %v instances: (timeout 5min) \n", len(aws_instances))
					// terminate instances in chunks of 197 to prevent API rate limiting
					for i := range aws_instances_split {
						go terminate_ec2_instances(client, aws_instances_split[i])
						// create waiter for each instance
						for j := range aws_instances_split[i] {
							wg.Add(1)
							go wait_ec2_termination(client, aws_instances_split[i][j], 5)
						}

					}
					wg.Wait()
					fmt.Println("EC2 instances terminated")
				}
			}
		}

		// delete elb instances & attached SGs (+referencing rules) from VPC
		delete_elb_instances(config.Aws__Vpc, cfg)

		// remove all terraform based infra
		tf_error := run_terraform_destroy(&config)
		if tf_error != "" {
			fmt.Printf("%s\n", tf_error)
			return
		}

		// at this point px clouddrive volumes must no longer be attached
		// as instances are terminated
		if len(aws_volumes) > 0 {
			fmt.Println(White + "Deleting px clouddrive volumes:" + Reset)
			for _, i := range aws_volumes {

				fmt.Println("  " + i)
				_, err = client.DeleteVolume(context.TODO(), &ec2.DeleteVolumeInput{
					VolumeId: aws.String(i),
				})
				if err != nil {
					fmt.Println("Error deleting Volume:")
					fmt.Println(err)
					return
				}
			}
		}

		aws_show_iamkey_age(&config)

	} else if config.Cloud == "gcp" {
		drivelist := make(map[string]string)
		if _, err := os.Stat("/px-deploy/.px-deploy/tf-deployments/" + config.Name); os.IsNotExist(err) {
			fmt.Println("Terraform Config for GCP deployment missing. If this has been created with a px-deploy Version <5.3.0 you need to destroy with the older version")
			die("Error: outdated deployment")
		}

		prepare_predelete(&config, "script", destroyForce)

		instances, err := gcp_get_instances(&config)
		if err != nil {
			die("error listing gcp instances")
		}

		wg := new(sync.WaitGroup)
		fmt.Printf("checking for px clouddrives \n")

		for _, val := range instances {
			drives, err := gcp_get_clouddrives(val, &config)
			if err != nil {
				die("error getting clouddrive listing for instance " + val)
			}
			if len(drives) > 0 {
				for _, drive := range drives {
					drivelist[drive] = val
				}
				if config.Platform == "k8s" {
					fmt.Printf("\t waiting 2min for %v clouddrive-attached instance %v to stop \n", len(drives), val)
					wg.Add(1)
					go gcp_stop_wait_instance(val, &config, wg)
				}
			}
		}

		if config.Platform == "gke" {
			fmt.Printf("found %v px clouddrives \n", len(drivelist))
			clusters, _ := strconv.Atoi(config.Clusters)
			for c := 1; c <= clusters; c++ {
				nodepools, err := gcp_get_nodepools(&config, fmt.Sprintf("px-deploy-%v-%v", config.Name, c))
				if err != nil {
					fmt.Printf("Warning: error listing gke nodepools\n")
				}

				for _, n := range nodepools {
					fmt.Printf("  deleting nodepool '%v' of cluster 'px-deploy-%v-%v' (can take 5min or longer)\n", n, config.Name, c)
					wg.Add(1)
					go gcp_delete_wait_nodepools(&config, fmt.Sprintf("px-deploy-%v-%v", config.Name, c), n, wg)
				}
			}
		}

		wg.Wait()

		if len(drivelist) > 0 {
			fmt.Printf("all clouddrive-attached nodes stopped.\nstarting disk detach/delete (default timeout 2min)\n")
		}

		for drive, inst := range drivelist {

			switch config.Platform {
			case "k8s":
				{
					wg.Add(1)
					fmt.Printf("\t detach and delete drive %s from host %s\n", drive, inst)
					go gcp_detach_delete_wait_clouddrive(inst, drive, &config, wg)
				}
			case "gke":
				{
					wg.Add(1)
					fmt.Printf("\t deleting clouddrive %s \n", drive)
					go gcp_delete_wait_clouddrive(drive, &config, wg)
				}
			}
		}
		wg.Wait()

		tf_error := run_terraform_destroy(&config)
		if tf_error != "" {
			fmt.Printf("%s\n", tf_error)
			return
		}

	} else if config.Cloud == "azure" {
		prepare_predelete(&config, "script", destroyForce)

		tf_error := run_terraform_destroy(&config)
		if tf_error != "" {
			fmt.Printf("%s\n", tf_error)
			return
		}
	} else if config.Cloud == "vsphere" {
		if _, err := os.Stat("/px-deploy/.px-deploy/tf-deployments/" + config.Name); os.IsNotExist(err) {
			fmt.Println("Terraform Config for vSphere deployment missing. If this has been created with a px-deploy Version <5.4.0 you need to destroy with the older version")
			die("Error: outdated deployment")
		}

		prepare_predelete(&config, "script", destroyForce)

		vsphere_prepare_destroy(&config)

		tf_error := run_terraform_destroy(&config)
		if tf_error != "" {
			fmt.Printf("%s\n", tf_error)
			return
		}
	} else {
		die("Bad cloud")
	}
	if err != nil {
		die("Failed to destroy")
	}
	fmt.Print(string(output))
	os.Remove("deployments/" + name + ".yml")
	os.Remove("keys/id_rsa." + config.Cloud + "." + name)
	os.Remove("keys/id_rsa." + config.Cloud + "." + name + ".pub")
	clusters, _ := strconv.Atoi(config.Clusters)
	for c := 0; c <= clusters; c++ {
		os.Remove("kubeconfig/" + name + "." + strconv.Itoa(c))
	}
	fmt.Println(White + "Destroyed." + Reset)
}
func run_terraform_destroy(config *Config) string {
	var cmd *exec.Cmd
	var cmdinit *exec.Cmd
	var buffer bytes.Buffer
	var cloud_auth []string
	var tf_plan_args []string

	switch config.Cloud {
	case "aws":
		cloud_auth = append(cloud_auth, fmt.Sprintf("AWS_ACCESS_KEY_ID=%s", config.Aws_Access_Key_Id))
		cloud_auth = append(cloud_auth, fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", config.Aws_Secret_Access_Key))
	}

	fmt.Println(White + "running Terraform PLAN" + Reset)
	// vsphere terraform must refresh, otherwise complains about missing disks
	// other clouds do no refresh as this saves time @scale
	if config.Cloud == "vsphere" {
		tf_plan_args = []string{"-chdir=/px-deploy/.px-deploy/tf-deployments/" + config.Name, "plan", "-destroy", "-input=false", "-refresh=true", "-parallelism=50", "-out=tfplan", "-var-file", ".tfvars"}
		//cmd = exec.Command("terraform", "-chdir=/px-deploy/.px-deploy/tf-deployments/"+config.Name, "plan", "-destroy", "-input=false", "-refresh=true", "-parallelism=50", "-out=tfplan", "-var-file", ".tfvars")
	} else {
		tf_plan_args = []string{"-chdir=/px-deploy/.px-deploy/tf-deployments/" + config.Name, "plan", "-destroy", "-input=false", "-refresh=false", "-parallelism=50", "-out=tfplan", "-var-file", ".tfvars"}
		//cmd = exec.Command("terraform", "-chdir=/px-deploy/.px-deploy/tf-deployments/"+config.Name, "plan", "-destroy", "-input=false", "-refresh=false", "-parallelism=50", "-out=tfplan", "-var-file", ".tfvars")
	}
	cmd = exec.Command("terraform", tf_plan_args...)
	cmd.Stderr = &buffer
	cmd.Env = append(cmd.Env, cloud_auth...)
	err := cmd.Run()

	if (err != nil) && strings.Contains(buffer.String(), "Required plugins are not installed") {
		// after updating with a px-deploy version containing newer modules the old modules for existing deployments might be missing
		// if we catch the "Required plugins are not installed" error on delete, lets try to re-init the deployment and re-run terraform plan
		fmt.Println(Yellow + "tf modules missing. re-running terraform init" + Reset)
		cmdinit = exec.Command("terraform", "-chdir=/px-deploy/.px-deploy/tf-deployments/"+config.Name, "init")
		cmdinit.Stderr = os.Stderr
		cmdinit.Stdout = os.Stdout
		err := cmdinit.Run()
		if err != nil {
			fmt.Println(Red + "ERROR: Terraform init failed. Check validity of terraform scripts" + Reset)
			die(err.Error())
		} else {
			fmt.Println(Yellow + "Terraform modules updated. Re-run Terraform PLAN" + Reset)
			cmd = exec.Command("terraform", tf_plan_args...)
			cmd.Stderr = os.Stderr
			cmd.Env = append(cmd.Env, cloud_auth...)
			err = cmd.Run()
			if err != nil {
				fmt.Println(Red + "ERROR: second try Terraform PLAN failed. Check validity of terraform scripts" + Reset)
				die(err.Error())
			}
		}
	} else if err != nil {
		fmt.Printf("%s", buffer.String())
		fmt.Println(Red + "ERROR: Terraform plan failed. Check validity of terraform scripts" + Reset)
		die(err.Error())
	}

	fmt.Println(White + "running Terraform DESTROY" + Reset)
	cmd = exec.Command("terraform", "-chdir=/px-deploy/.px-deploy/tf-deployments/"+config.Name, "apply", "-input=false", "-parallelism=50", "-auto-approve", "tfplan")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(cmd.Env, cloud_auth...)
	errdestroy := cmd.Run()

	if errdestroy != nil {
		fmt.Println(Yellow + "ERROR: Terraform destroy failed. Check validity of terraform scripts" + Reset)
		die(errdestroy.Error())
	} else {
		os.RemoveAll("tf-deployments/" + config.Name)
	}

	return ""
}

func azure_get_node_ip(deployment string, node string) string {
	config := parse_yaml("/px-deploy/.px-deploy/deployments/" + deployment + ".yml")
	var output = []byte("")

	cred, err := azidentity.NewClientSecretCredential(config.Azure_Tenant_Id, config.Azure_Client_Id, config.Azure_Client_Secret, nil)
	if err != nil {
		panic("failed to create azure credential" + err.Error())
	}

	ctx := context.Background()

	// Create and authorize a ResourceGraph client
	client, err := armresourcegraph.NewClient(cred, nil)
	if err != nil {
		panic("failed to create azure client: " + err.Error())
	}

	results, err := client.Resources(ctx,
		armresourcegraph.QueryRequest{
			Query: to.Ptr(fmt.Sprintf("Resources | where type =~ 'Microsoft.Network/publicIPAddresses' | where resourceGroup == 'px-deploy.%s' | where name == 'px-deploy.%s.%s' | limit 1", config.Name, config.Name, node)),
			Subscriptions: []*string{
				to.Ptr(config.Azure_Subscription_Id)},
		},
		nil)
	if err != nil {
		panic("failed to search azure public ip:" + err.Error())
	} else {
		// https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph#QueryResponse
		for _, value := range results.Data.([]interface{}) {
			//fmt.Printf("Data result %v, type %s \n\n", key, reflect.TypeOf(value))
			for field, result := range value.(map[string]interface{}) {
				if field == "properties" {
					for prop, prop_val := range result.(map[string]interface{}) {
						if prop == "ipAddress" {
							//fmt.Printf("  %v : %v\n", prop, prop_val)
							output = []byte(fmt.Sprint(prop_val))
						}
					}
				}
			}
		}
	}
	return strings.TrimSuffix(string(output), "\n")
}

// get node ip following old naming scheme (master-1)
// terraform based deployments changed naming to master-1-1
// this function can be replaced after all clouds run terraform based
func get_ip(deployment string) string {
	config := parse_yaml("/px-deploy/.px-deploy/deployments/" + deployment + ".yml")
	var output []byte
	if config.Cloud == "aws" {
		output = []byte(aws_get_node_ip(deployment, "master-1-1"))
	} else if config.Cloud == "gcp" {
		output = []byte(gcp_get_node_ip(deployment, config.Name+"-master-1-1"))
	} else if config.Cloud == "azure" {
		output = []byte(azure_get_node_ip(deployment, "master-1-1"))
	} else if config.Cloud == "vsphere" {
		output = []byte(vsphere_get_node_ip(&config, config.Name+"-master-1"))
	}
	return strings.TrimSuffix(string(output), "\n")
}

func prepare_predelete(config *Config, runtype string, destroyForce bool) {
	// master node naming scheme on clouds:
	//aws:     master-[clusternum]-1
	//azure:   master-[clusternum]-1
	//gcp:     [configname]-master-[clusternum]-1
	//vsphere: [configname]-master-[clusternum]

	var name_pre, name_post string

	// script predelete only executes if set in config
	if !config.Run_Predelete && runtype == "script" {
		return
	}

	clusters, _ := strconv.Atoi(config.Clusters)
	predelete_status := make(chan Predelete_Status_Return, clusters)

	switch config.Cloud {
	case "aws":
		{
			name_pre = "master-"
			name_post = "-1"
		}
	case "azure":
		{
			name_pre = "master-"
			name_post = "-1"
		}
	case "gcp":
		{
			name_pre = fmt.Sprintf("%v-master-", config.Name)
			name_post = "-1"
		}
	case "vsphere":
		{
			name_pre = fmt.Sprintf("%v-master-", config.Name)
			name_post = ""
		}
	}

	if config.Platform == "ocp4" && runtype == "platform" {
		fmt.Printf("Destroying OCP4 cluster(s), wait about 15 minutes (per cluster)... Output is mixed\n")
	} else {
		fmt.Printf("Running pre-delete scripts on all master nodes. Output is mixed\n")
	}

	wg.Add(clusters)
	for i := 1; i <= clusters; i++ {
		go exec_predelete(config, fmt.Sprintf("%v%v%v", name_pre, i, name_post), runtype, predelete_status)
	}
	wg.Wait()
	close(predelete_status)

	for elem := range predelete_status {
		if !elem.success {
			if destroyForce {
				fmt.Printf("%v %v failed %v predelete. --force parmeter set. Continuing delete%v\n", Red, elem.node, runtype, Reset)
			} else {
				fmt.Printf("%v %v failed %v predelete. canceled deletion process.\nensure %v is powered on and can be accessed by ssh, then retry\n(to enforce deletion use --force parameter)%v\n", Red, elem.node, runtype, elem.node, Reset)
				os.Exit(1)
			}
		}
	}

	if config.Platform == "ocp4" && runtype == "platform" {
		fmt.Printf("OCP4 cluster delete done\n")
	} else {
		fmt.Printf("pre-delete %v done\n", runtype)
	}
}

func exec_predelete(config *Config, confNode string, confPath string, success chan Predelete_Status_Return) {
	var ip string

	defer wg.Done()

	switch config.Cloud {
	case "aws":
		{
			ip = aws_get_node_ip(config.Name, confNode)
		}
	case "gcp":
		{
			ip = gcp_get_node_ip(config.Name, confNode)
		}
	case "vsphere":
		{
			ip = vsphere_get_node_ip(config, confNode)
		}
	case "azure":
		{
			ip = azure_get_node_ip(config.Name, confNode)
		}
	default:
		{
			panic(fmt.Sprintf("pre_delete not implemented for cloud %v", config.Cloud))
		}

	}
	fmt.Printf("Running pre-delete scripts on %v (%v)\n", confNode, ip)

	cmd := exec.Command("/usr/bin/ssh", "-oStrictHostKeyChecking=no", "-q", "-i", "keys/id_rsa."+config.Cloud+"."+config.Name, "root@"+ip, `
		for i in /px-deploy/`+confPath+`-delete/*.sh; do bash $i ;done; exit 0
	`)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		fmt.Println(Yellow + "Failed to run pre-delete script:" + err.Error() + Reset)
		success <- Predelete_Status_Return{confNode, false}
	} else {
		success <- Predelete_Status_Return{confNode, true}
	}

}

// return which variables are empty
func isEmpty(vars ...interface{}) []int {
	emptyVars := []int{}
	for i, v := range vars {
		if v == nil || reflect.ValueOf(v).IsZero() {
			emptyVars = append(emptyVars, i)
		}
	}
	return emptyVars
}

func write_nodescripts(config Config) {
	var tf_node_scripts []string
	var tf_master_scripts []string
	var tf_common_master_script []byte
	var tf_post_script []byte
	var tf_node_script []byte
	var tf_individual_node_script []byte
	var tf_master_script []byte
	var tf_env_script []byte
	var tf_cluster_nodes []byte

	Clusters, _ := strconv.Atoi(config.Clusters)
	Nodes, _ := strconv.Atoi(config.Nodes)

	// prepare ENV variables for node/master scripts
	// to maintain compatibility, create a env variable of everything from the yml spec which is from type string
	e := reflect.ValueOf(&config).Elem()
	for i := 0; i < e.NumField(); i++ {
		if e.Type().Field(i).Type.Name() == "string" {
			if strings.ToLower(strings.TrimSpace(e.Type().Field(i).Name)) == "vsphere_password" {
				tf_env_script = append(tf_env_script, "export "+strings.ToLower(strings.TrimSpace(e.Type().Field(i).Name))+"='"+strings.TrimSpace(e.Field(i).Interface().(string))+"'\n"...)
			} else {
				tf_env_script = append(tf_env_script, "export "+strings.ToLower(strings.TrimSpace(e.Type().Field(i).Name))+"=\""+strings.TrimSpace(e.Field(i).Interface().(string))+"\"\n"...)
			}
		}
	}

	// get cluster specific node overrides
	// not possible to set in env as cloud-init doesnt seem to run on full bash, so array not working
	tf_cluster_nodes = append(tf_cluster_nodes, "export clusternodes=( \"0\" "...)
	for c := 1; c <= Clusters; c++ {
		clusternodes := config.Nodes
		for _, i := range config.Cluster {
			if i.Id == c {
				if i.Nodes != "" {
					clusternodes = i.Nodes
				}
			}
		}
		tf_cluster_nodes = append(tf_cluster_nodes, fmt.Sprintf("\"%v\" ", clusternodes)...)
	}
	tf_cluster_nodes = append(tf_cluster_nodes, ")\n"...)

	// set env variables from env spec
	for key, val := range config.Env {
		tf_env_script = append(tf_env_script, "export "+key+"=\""+val+"\"\n"...)
	}
	err := os.WriteFile("/px-deploy/.px-deploy/tf-deployments/"+config.Name+"/env.sh", tf_env_script, 0666)
	if err != nil {
		die(err.Error())
	}

	// prepare (single) cloud-init script for all nodes
	tf_node_scripts = []string{"all-common", config.Platform + "-common", config.Platform + "-node"}
	tf_node_script = append(tf_node_script, "#!/bin/bash\n"...)
	tf_node_script = append(tf_node_script, tf_cluster_nodes...)
	tf_node_script = append(tf_node_script, "mkdir /var/log/px-deploy\n"...)

	for _, filename := range tf_node_scripts {
		content, err := os.ReadFile("/px-deploy/.px-deploy/infra/" + filename)
		if err == nil {
			tf_node_script = append(tf_node_script, "(\n"...)
			tf_node_script = append(tf_node_script, "echo \"Started $(date)\"\n"...)
			tf_node_script = append(tf_node_script, "echo \""+filename+"_start,$(date +%s)\" >>/var/log/px-deploy/script_tracking\n"...)
			tf_node_script = append(tf_node_script, content...)
			tf_node_script = append(tf_node_script, "\necho \"Finished $(date)\"\n"...)
			tf_node_script = append(tf_node_script, "echo \""+filename+"_stop,$(date +%s)\" >>/var/log/px-deploy/script_tracking\n"...)
			tf_node_script = append(tf_node_script, "\n) >&/var/log/px-deploy/"+filename+"\n"...)
		}
	}

	// prepare common base script for all master nodes
	// prepare common cloud-init script for all master nodes
	tf_master_scripts = []string{"all-common", config.Platform + "-common", "all-master", config.Platform + "-master"}
	tf_common_master_script = append(tf_common_master_script, "#!/bin/bash\n"...)
	tf_common_master_script = append(tf_common_master_script, tf_cluster_nodes...)
	tf_common_master_script = append(tf_common_master_script, "mkdir /px-deploy\n"...)
	tf_common_master_script = append(tf_common_master_script, "mkdir /px-deploy/platform-delete\n"...)
	tf_common_master_script = append(tf_common_master_script, "mkdir /px-deploy/script-delete\n"...)
	tf_common_master_script = append(tf_common_master_script, "touch /px-deploy/script-delete/dummy.sh\n"...)
	tf_common_master_script = append(tf_common_master_script, "touch /px-deploy/platform-delete/dummy.sh\n"...)
	tf_common_master_script = append(tf_common_master_script, "mkdir /var/log/px-deploy\n"...)
	tf_common_master_script = append(tf_common_master_script, "mkdir /var/log/px-deploy/completed\n"...)
	tf_common_master_script = append(tf_common_master_script, "touch /var/log/px-deploy/completed/tracking\n"...)

	for _, filename := range tf_master_scripts {
		content, err := os.ReadFile("/px-deploy/.px-deploy/infra/" + filename)
		if err == nil {
			tf_common_master_script = append(tf_common_master_script, "(\n"...)
			tf_common_master_script = append(tf_common_master_script, "echo \"Started $(date)\"\n"...)
			tf_common_master_script = append(tf_common_master_script, "echo \""+filename+"_start,$(date +%s)\" >>/var/log/px-deploy/script_tracking\n"...)
			tf_common_master_script = append(tf_common_master_script, content...)
			tf_common_master_script = append(tf_common_master_script, "\necho \"Finished $(date)\"\n"...)
			tf_common_master_script = append(tf_common_master_script, "echo \""+filename+"_stop,$(date +%s)\" >>/var/log/px-deploy/script_tracking\n"...)
			tf_common_master_script = append(tf_common_master_script, "\n) >&/var/log/px-deploy/"+filename+"\n"...)
		}
	}

	// add scripts from the "scripts" section of config.yaml to common master node script
	for _, filename := range config.Scripts {
		content, err := os.ReadFile("/px-deploy/.px-deploy/scripts/" + filename)
		if err == nil {
			tf_common_master_script = append(tf_common_master_script, "(\n"...)
			tf_common_master_script = append(tf_common_master_script, "echo \"Started $(date)\"\n"...)
			tf_common_master_script = append(tf_common_master_script, "echo \""+filename+"_start,$(date +%s)\" >>/var/log/px-deploy/script_tracking\n"...)
			tf_common_master_script = append(tf_common_master_script, content...)
			tf_common_master_script = append(tf_common_master_script, "\necho \"Finished $(date)\"\n"...)
			tf_common_master_script = append(tf_common_master_script, "echo \""+filename+"_stop,$(date +%s)\" >>/var/log/px-deploy/script_tracking\n"...)
			tf_common_master_script = append(tf_common_master_script, "\n) >&/var/log/px-deploy/"+filename+"\n"...)
		}
	}

	// add post_script if defined
	if config.Post_Script != "" {
		content, err := os.ReadFile("/px-deploy/.px-deploy/scripts/" + config.Post_Script)
		if err == nil {
			tf_post_script = append(tf_post_script, "(\n"...)
			tf_post_script = append(tf_post_script, "echo \"Started $(date)\"\n"...)
			tf_post_script = append(tf_post_script, "echo \""+config.Post_Script+"_start,$(date +%s)\" >>/var/log/px-deploy/script_tracking\n"...)
			tf_post_script = append(tf_post_script, content...)
			tf_post_script = append(tf_post_script, "\necho \"Finished $(date)\"\n"...)
			tf_post_script = append(tf_post_script, "echo \""+config.Post_Script+"_stop,$(date +%s)\" >>/var/log/px-deploy/script_tracking\n"...)
			tf_post_script = append(tf_post_script, "\n) >&/var/log/px-deploy/"+config.Post_Script+"\n"...)
		}
	} else {
		tf_post_script = nil
	}

	// loop clusters (masters and nodes) to build tfvars and master/node scripts
	for c := 1; c <= Clusters; c++ {
		masternum := strconv.Itoa(c)

		tf_master_script = tf_common_master_script

		// if exist, apply individual scripts/aws_type settings for nodes of a cluster
		for _, clusterconf := range config.Cluster {
			if clusterconf.Id == c {
				for _, filename := range clusterconf.Scripts {
					content, err := os.ReadFile("/px-deploy/.px-deploy/scripts/" + filename)
					if err == nil {
						tf_master_script = append(tf_master_script, "(\n"...)
						tf_master_script = append(tf_master_script, "echo \"Started $(date)\"\n"...)
						tf_master_script = append(tf_master_script, "echo \""+filename+"_start,$(date +%s)\" >>/var/log/px-deploy/script_tracking\n"...)
						tf_master_script = append(tf_master_script, content...)
						tf_master_script = append(tf_master_script, "\necho \"Finished $(date)\"\n"...)
						tf_master_script = append(tf_master_script, "echo \""+filename+"_stop,$(date +%s)\" >>/var/log/px-deploy/script_tracking\n"...)
						tf_master_script = append(tf_master_script, "\n) >&/var/log/px-deploy/"+filename+"\n"...)
					}
				}
			}
		}

		// add post_script if defined
		if tf_post_script != nil {
			tf_master_script = append(tf_master_script, tf_post_script...)
		}

		// after running all scripts create file in /var/log/px-deploy/completed
		tf_master_script = append(tf_master_script, "export IP=$(curl -s https://ipinfo.io/ip)\n"...)
		tf_master_script = append(tf_master_script, "echo \"master-"+masternum+" $IP \" >> /var/log/px-deploy/completed/tracking \n"...)

		//write master script for cluster
		err := os.WriteFile("/px-deploy/.px-deploy/tf-deployments/"+config.Name+"/master-"+masternum+"-1", tf_master_script, 0666)
		if err != nil {
			die(err.Error())
		}

		cmd := exec.Command("bash", "-n", "/px-deploy/.px-deploy/tf-deployments/"+config.Name+"/master-"+masternum+"-1")
		err = cmd.Run()
		if err != nil {
			die("PANIC: generated script '.px-deploy/tf-deployments/" + config.Name + "/master-" + masternum + "-1' is not valid Bash")
		}
		// set cluster specfic node # to config node #
		CL_Nodes := Nodes
		// check if there is a cluster specific node # override
		for _, cl_entry := range config.Cluster {
			if (cl_entry.Id == c) && (cl_entry.Nodes != "") {
				CL_Nodes, _ = strconv.Atoi(cl_entry.Nodes)
			}
		}
		// loop nodes of cluster, add node name/ip to tf var and write individual cloud-init scripts file
		if CL_Nodes > 0 {
			for n := 1; n <= CL_Nodes; n++ {
				nodenum := strconv.Itoa(n)
				tf_individual_node_script = tf_node_script
				tf_individual_node_script = append(tf_individual_node_script, "export IP=$(curl -s https://ipinfo.io/ip)\n"...)
				tf_individual_node_script = append(tf_individual_node_script, "echo \"echo 'node-"+masternum+"-"+nodenum+" $IP' >> /var/log/px-deploy/completed/tracking \" | ssh root@master-"+masternum+" \n"...)
				tf_individual_node_script = append(tf_individual_node_script, "scp /var/log/px-deploy/script_tracking root@master-"+masternum+":/var/log/px-deploy/script_tracking_node-"+masternum+"-"+nodenum+"\n"...)
				err := os.WriteFile("/px-deploy/.px-deploy/tf-deployments/"+config.Name+"/node-"+masternum+"-"+nodenum, tf_individual_node_script, 0666)
				if err != nil {
					die(err.Error())
				}

				cmd := exec.Command("bash", "-n", "/px-deploy/.px-deploy/tf-deployments/"+config.Name+"/node-"+masternum+"-"+nodenum)
				err = cmd.Run()
				if err != nil {
					die("PANIC: generated script '.px-deploy/tf-deployments/" + config.Name + "/node-" + masternum + "-" + nodenum + "' is not valid Bash")
				}

			}
		}
	}
}

func version() {
	fmt.Println(get_version_current())
}

func history(number string) {
	f, err := os.Open("log")
	if err != nil {
		die(err.Error())
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	if number != "" {
		if num, err := strconv.Atoi(number); err == nil {
			for i := 0; i <= num; i++ {
				scanner.Scan()
			}
			i := strings.Index(scanner.Text(), " ] ")
			c, err := base64.StdEncoding.DecodeString(scanner.Text()[i+3:])
			if err != nil {
				die(err.Error())
			}
			fmt.Println(string(c))
			os.Exit(0)
		} else {
			die("Invalid deployment ID")
		}

	} else {
		var data [][]string
		n := 0
		for scanner.Scan() {
			i1 := strings.Index(scanner.Text(), " [ ")
			i2 := strings.Index(scanner.Text(), " ] ")
			data = append(data, []string{strconv.Itoa(n), scanner.Text()[0:i1], scanner.Text()[i1+3 : i2]})
			n++
		}
		if err := scanner.Err(); err != nil {
			die(err.Error())
		}
		print_table([]string{"Number", "Timestamp", "Parameters"}, data)
	}
}

func list_templates() {
	var data [][]string
	os.Chdir("templates")
	var foo = _list_templates(".")
	data = append(data, foo...)
	print_table([]string{"Name", "Description"}, data)
}

func _list_templates(dir string) [][]string {
	var temp [][]string
	filepath.Walk(dir, func(file string, info os.FileInfo, err error) error {
		if info.Mode()&os.ModeDir != 0 && dir != file {
			_list_templates(file)
		}
		if path.Ext(file) != ".yml" {
			return nil
		}
		config := parse_yaml(file)
		file = strings.TrimSuffix(file, ".yml")
		temp = append(temp, []string{file, config.Description})
		return nil
	})
	return temp
}

func die(msg string) {
	fmt.Println(Red + msg + Reset)
	os.Exit(1)
}

func log(msg string) {
	file, err := os.OpenFile("/px-deploy/.px-deploy/log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		die("Cannot open log: " + err.Error())
	}
	defer file.Close()
	if _, err := file.WriteString(time.Now().Format(time.RFC3339) + " " + msg + "\n"); err != nil {
		die("Cannot write log: " + err.Error())
	}
}

func parse_yaml(filename string) Config {
	b, err := os.ReadFile(filename)
	if err != nil {
		die(err.Error())
	}
	if len(b) != utf8.RuneCount(b) {
		die("Non-ASCII values found in " + filename)
	}
	var d Config
	err = yaml.Unmarshal(b, &d)
	if err != nil {
		die("Broken YAML in " + filename + ": " + err.Error())
	}
	return d
}

func print_table(header []string, data [][]string) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(header)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetColWidth(80)
	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.SetTablePadding("  ")
	table.SetNoWhiteSpace(true)
	table.AppendBulk(data)
	table.Render()
}

func write_tf_file(deployment string, filename string, data []string) {
	file, err := os.OpenFile("/px-deploy/.px-deploy/tf-deployments/"+deployment+"/"+filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		die("Cannot open file: " + err.Error())
	}
	defer file.Close()
	for _, value := range data {
		if _, err := file.WriteString(value + "\n"); err != nil {
			die("Cannot write file: " + err.Error())
		}
	}
}
func latest_version() bool {
	version_current := get_version_current()
	version_latest := get_version_latest()
	if version_latest == "" {
		fmt.Println(Yellow + "Current version is " + version_current + ", cannot determine latest version" + Reset)
		return true
	} else {
		if version_current != version_latest {
			fmt.Println(Yellow + "Current version is " + version_current + ", latest version is " + version_latest + Reset)
			return false
		} else {
			fmt.Println(Green + "Current version is " + version_current + " (latest)" + Reset)
			return true
		}
	}
}

func get_version_current() string {
	v, err := os.ReadFile("/VERSION")
	if err != nil {
		die(err.Error())
	}
	return strings.TrimSpace(string(v))
}

func get_version_latest() string {
	resp, err := http.Get("https://raw.githubusercontent.com/PureStorage-OpenConnect/px-deploy/master/VERSION")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	v := strings.TrimSpace(string(body))
	if regexp.MustCompile(`^[0-9\.]+$`).MatchString(v) {
		return v
	}
	return ""
}

func check_for_recommended_settings(config *Config) {
	// check for "recommended" version in default.yaml.[version_current]
	if _, err := os.Stat("versions.yml"); os.IsNotExist(err) {
		fmt.Printf("%sversions.yml not found. No recommended versions available to be shown%s \n", Yellow, Reset)
	} else {
		fmt.Printf("checking your defaults.yml for recommended version settings (from versions.yml) \n")
		recommended_versions := parse_yaml("versions.yml")
		recVers := reflect.ValueOf(recommended_versions)
		curDef := reflect.ValueOf(config)
		typeOfS := recVers.Type()
		for i := 0; i < recVers.NumField(); i++ {
			// get all fields from recommended defaults.yml.VERSION with name "version" inside and check against current default settings
			if strings.Contains(strings.ToLower(typeOfS.Field(i).Name), "version") {
				versioning_field := strings.ToLower(typeOfS.Field(i).Name)
				versioning_current := fmt.Sprintf("%s", reflect.Indirect(curDef).FieldByName(typeOfS.Field(i).Name))
				versioning_recommended := fmt.Sprintf("%s", recVers.Field(i).Interface())
				//fmt.Printf("(Field: %s\t Value: %s \t\t Recommended: %s)\n", versioning_field , versioning_current, versioning_recommended)
				if versioning_recommended != "" && versioning_current != "" {
					if versioning_recommended != versioning_current {

						v1, err := version_hashicorp.NewVersion(versioning_current)
						if err != nil {
							fmt.Printf("Error processing current Versioning %s : %s\n", versioning_field, versioning_current)
						}
						v2, err := version_hashicorp.NewVersion(versioning_recommended)
						if err != nil {
							fmt.Printf("Error processing recommended Versioning %s : %s", versioning_field, versioning_recommended)
						}
						if v1.LessThan(v2) {
							fmt.Printf("%sWarning:%s %s: %s %sin defaults.yml is lower than recommended setting%s %s\n", Yellow, Reset, versioning_field, versioning_current, Yellow, Reset, versioning_recommended)
						}
					}
				} else if versioning_recommended == "" {
					fmt.Printf("Field %s has no recommended version information available\n", versioning_field)
				} else if versioning_current == "" {
					fmt.Printf("%s please add%s %s: \"%s\" %sto defaults.yml (recommended setting) %s\n", Red, Reset, versioning_field, versioning_recommended, Red, Reset)
				}
			}
		}
	}
}

func sync_repository() {
	fmt.Printf("syncing container repo to local dir\n")
	cmd := exec.Command("rsync", "-a", "/px-deploy/assets", "/px-deploy/scripts", "/px-deploy/templates", "/px-deploy/infra", "/px-deploy/versions.yml", "/px-deploy/.px-deploy/")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	errapply := cmd.Run()
	if errapply != nil {
		fmt.Println(Red + "ERROR: failed to sync container repo to .px-deploy repo" + Reset)
	}
}
