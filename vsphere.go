package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type VmGuestNetworkingInterfacesInfo struct {
	Mac string                                  `json:"mac_address"`
	Nic string                                  `json:"nic"`
	Ip  VmGuestNetworkingInterfacesIpConfigInfo `json:"ip"`
}

type VmGuestNetworkingInterfacesIpConfigInfo struct {
	Ip_addresses []VmGuestNetworkingInterfacesIpAddressInfo `json:"ip_addresses"`
}

type VmGuestNetworkingInterfacesIpAddressInfo struct {
	Ip_address    string `json:"ip_address"`
	Prefix_length int64  `json:"prefix_length"`
	State         string `json:"state"`
}

type VsphereRestClient interface{}

type vsphereRestClient struct {
	server    string
	user      string
	password  string
	sessionid string
}

type VsphereRestApiError struct {
	Error_type string `json:"error_type"`
}
type VsphereVmPwrResponseObj struct {
	State string `json:"state"`
}

type VmInfo struct {
	Disks map[string]VmHardwareDiskInfo `json:"disks"`
}

type VmHardwareDiskInfo struct {
	Backing  VmHardwareDiskBackingType `json:"backing"`
	Label    string                    `json:"label"`
	Type     string                    `json:"type"`
	Capacity int64                     `json:"capacity"`
}

type VmHardwareDiskBackingType struct {
	Type      string `json:"type"`
	Vmdk_file string `json:"vmdk_file"`
}

type Vsphere_Px_Clouddrive struct {
	mobid  string
	diskid string
	path   string
}

type Govc_Vm_Info struct {
	Vms []Govc_Vm_Entry `json:"VirtualMachines"`
}

type Govc_Vm_Entry struct {
	Name   string         `json:"Name"`
	Config Govc_Vm_Config `json:"Config"`
}

type Govc_Vm_Config struct {
	ExtraConfig []Govc_Extra_Config `json:"ExtraConfig"`
}

type Govc_Extra_Config struct {
	Key   string `json:"Key"`
	Value string `json:"Value"`
}

func NewVsphereRestClient(server string, user string, password string) vsphereRestClient {
	return vsphereRestClient{server, user, password, ""}
}

func (v *vsphereRestClient) Login() error {

	transCfg := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := http.Client{Transport: transCfg}

	req, err := http.NewRequest("POST", "https://"+v.server+"/api/session", nil)
	if err != nil {
		fmt.Println(err)
		return error(err)
	}

	req.SetBasicAuth(v.user, v.password)
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return error(err)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return error(err)
	}

	if resp.StatusCode == 201 {
		v.sessionid = strings.Trim(string(respBody), "\"")
		return nil
	} else {
		fmt.Printf("Login failed. Code %s resp '%s'", resp.Status, string(respBody))
		return fmt.Errorf(resp.Status)
	}
}

func (v *vsphereRestClient) Logout() error {

	transCfg := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := http.Client{Transport: transCfg}

	req, err := http.NewRequest("DELETE", "https://"+v.server+"/api/session", nil)
	if err != nil {
		fmt.Println(err)
		return error(err)
	}

	req.Header = http.Header{
		"vmware-api-session-id": {v.sessionid},
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return error(err)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return error(err)
	}

	if resp.StatusCode == 204 {
		//fmt.Printf("Logout successful\n")
		v.sessionid = ""
		return nil
	} else {
		fmt.Printf("Logout failed. Code %s resp '%s'", resp.Status, string(respBody))
		return fmt.Errorf(resp.Status)
	}
}

func getvSphereNodeIp(mobId string, mac string, v *vsphereRestClient) (ip string, err error) {
	var responseObj []VmGuestNetworkingInterfacesInfo

	transCfg := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := http.Client{Transport: transCfg}

	req, err := http.NewRequest("GET", "https://"+v.server+"/api/vcenter/vm/"+mobId+"/guest/networking/interfaces", nil)
	if err != nil {
		fmt.Println(err)
		return "", error(err)
	}

	req.Header = http.Header{
		"vmware-api-session-id": {v.sessionid},
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return "", error(err)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return "", error(err)
	}

	if resp.StatusCode == 200 {
		json.Unmarshal(respBody, &responseObj)
		for _, val := range responseObj {
			if val.Mac == mac {
				for _, ip := range val.Ip.Ip_addresses {
					if ip.State == "PREFERRED" {
						return ip.Ip_address, nil
					}
				}
			}
		}
	} else {
		fmt.Printf("Request for VM IP failed. code %s resp '%s' \n", resp.Status, string(respBody))
		return "", fmt.Errorf(resp.Status)
	}

	return "", fmt.Errorf("API did not get IP for node %s \n request body: %v \n", mobId, responseObj)
}

func vsphere_create_variables(config *Config) []string {
	var tf_variables []string
	var tf_cluster_nodes string

	Clusters, _ := strconv.Atoi(config.Clusters)
	Nodes, _ := strconv.Atoi(config.Nodes)

	// build terraform variable file
	tf_variables = append(tf_variables, "config_name = \""+config.Name+"\"")
	tf_variables = append(tf_variables, "clusters = "+config.Clusters)
	tf_variables = append(tf_variables, "vsphere_host = \""+config.Vsphere_Host+"\"")
	tf_variables = append(tf_variables, "vsphere_compute_resource = \""+config.Vsphere_Compute_Resource+"\"")
	tf_variables = append(tf_variables, "vsphere_resource_pool = \""+config.Vsphere_Resource_Pool+"\"")
	tf_variables = append(tf_variables, "vsphere_datacenter = \""+config.Vsphere_Datacenter+"\"")
	tf_variables = append(tf_variables, "vsphere_template = \""+config.Vsphere_Template+"\"")
	tf_variables = append(tf_variables, "vsphere_folder = \""+config.Vsphere_Folder+"\"")
	tf_variables = append(tf_variables, "vsphere_user = \""+config.Vsphere_User+"\"")
	tf_variables = append(tf_variables, "vsphere_password = \""+config.Vsphere_Password+"\"")
	tf_variables = append(tf_variables, "vsphere_datastore = \""+config.Vsphere_Datastore+"\"")
	tf_variables = append(tf_variables, "vsphere_network = \""+config.Vsphere_Network+"\"")
	tf_variables = append(tf_variables, "vsphere_memory = \""+config.Vsphere_Memory+"\"")
	tf_variables = append(tf_variables, "vsphere_cpu = \""+config.Vsphere_Cpu+"\"")
	tf_variables = append(tf_variables, "aws_access_key_id = \""+config.Aws_Access_Key_Id+"\"")
	tf_variables = append(tf_variables, "aws_secret_access_key = \""+config.Aws_Secret_Access_Key+"\"")
	tf_variables = append(tf_variables, "aws_session_token = \""+config.Aws_Session_Token+"\"")

	if (config.Vsphere_Dns != "") && (config.Vsphere_Gw != "") && (config.Vsphere_Node_Ip != "") {
		tf_variables = append(tf_variables, "vsphere_dns = \""+config.Vsphere_Dns+"\"")
		tf_variables = append(tf_variables, "vsphere_gw = \""+config.Vsphere_Gw+"\"")
		tf_variables = append(tf_variables, "vsphere_ip = \""+config.Vsphere_Node_Ip+"\"")
	}

	tf_variables = append(tf_variables, "nodeconfig = [")

	// loop clusters (masters and nodes) to build tfvars and master/node scripts
	for c := 1; c <= Clusters; c++ {
		masternum := strconv.Itoa(c)
		tf_cluster_nodes = strconv.Itoa(Nodes)

		// if exist, apply individual scripts/settings for nodes of a cluster
		for _, clusterconf := range config.Cluster {
			if clusterconf.Id == c {
				if clusterconf.Nodes != "" {
					tf_cluster_nodes = clusterconf.Nodes
				}
			}
		}
		// process .tfvars file for deployment
		tf_variables = append(tf_variables, "  {")
		tf_variables = append(tf_variables, "    role = \"master\"")
		tf_variables = append(tf_variables, "    nodecount = 1")
		tf_variables = append(tf_variables, "    cluster = "+masternum)
		tf_variables = append(tf_variables, "  },")

		tf_variables = append(tf_variables, "  {")
		tf_variables = append(tf_variables, "    role = \"node\"")
		tf_variables = append(tf_variables, "    nodecount = "+tf_cluster_nodes)
		tf_variables = append(tf_variables, "    cluster = "+masternum)
		tf_variables = append(tf_variables, "  },")
	}
	tf_variables = append(tf_variables, "]")
	return tf_variables
}

// displays a message if local pxd template version differs from repository
// checked against an already deployed master-1 instance
// we cannot check pxd-templateid of a vsphere template as rest api does not show these as VM
// so we'll check after deployment when VMs have been created
// benefit: we already have mobid of master-1
func vsphere_check_templateversion(deployment string) {
	config := parse_yaml("/px-deploy/.px-deploy/deployments/" + deployment + ".yml")

	fmt.Printf("Checking versions of local & remote px-deploy templates. This may take some seconds\n")
	rtc := make(chan string)
	ltc := make(chan string)
	go vsphere_get_localtemplate(&config, ltc)
	go vsphere_get_remotetemplate(&config, rtc)
	rt, lt := <-rtc, <-ltc

	rt = strings.TrimSpace(rt)
	lt = strings.TrimSpace(lt)

	if (rt == "0") || (lt == "0") {
		if rt == "0" {
			fmt.Printf("%s Warning: Could not get remote px-deploy template version\nLocal version is %s %s \n", Yellow, lt, Reset)
		}
		if lt == "0" {
			fmt.Printf("%s Warning: Could not get local px-deploy template version\nRemote version is %s %s \n", Yellow, rt, Reset)
		}
		return
	}
	if rt != lt {
		fmt.Printf("%s px-deploy template (current version %s) update available (version %s).\nPlease run px-deploy vsphere-init to update%s \n", Yellow, lt, rt, Reset)
	} else {
		fmt.Printf("%s Info: local px-deploy template up-to-date (Version %s)%s \n", Green, lt, Reset)
	}
}

func vsphere_get_remotetemplate(config *Config, c chan string) {
	client := http.Client{}
	req, err := http.NewRequest("GET", config.Vsphere_Repo+"pxdid.txt", nil)
	if err != nil {
		fmt.Printf("CheckTemplateVersion: error building http s3 request %s \n", error(err))
		c <- "0"
		return
	}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("CheckTemplateVersion: error submitting http s3 request %s \n", error(err))
		c <- "0"
		return
	}
	remoteTemplateVersion, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("CheckTemplateVersion: error reading http s3 request body %s \n", error(err))
		c <- "0"
		return
	}
	if resp.StatusCode == 200 {
		//fmt.Printf("Remote Template Version\t%s \n", remoteTemplateVersion)
		c <- string(remoteTemplateVersion)
	} else {
		fmt.Printf("CheckTemplateVersion: Error: s3 request returned statuscode %v \n", resp.StatusCode)
		c <- "0"
		return
	}
}

func vsphere_get_localtemplate(config *Config, c chan string) {
	var govc_vm_info Govc_Vm_Info
	var govc_opts []string
	var localtemplateid string
	// vsphere REST api does not provide VM.config.extraconfig Information
	// so we need to run govc
	govc_opts = append(govc_opts, fmt.Sprintf("GOVC_URL=%s", config.Vsphere_Host))
	govc_opts = append(govc_opts, fmt.Sprintf("GOVC_INSECURE=1"))
	govc_opts = append(govc_opts, fmt.Sprintf("GOVC_USERNAME=%s", config.Vsphere_User))
	govc_opts = append(govc_opts, fmt.Sprintf("GOVC_PASSWORD=%s", config.Vsphere_Password))
	govc_opts = append(govc_opts, fmt.Sprintf("GOVC_DATACENTER=%s", config.Vsphere_Datacenter))
	cmd := exec.Command("govc", "vm.info", "-json", config.Name+"-master-1")
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, govc_opts...)
	govcresponse, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(Yellow + "Cannot get local pxd templateid. govc reported stderr" + Reset)
		c <- "0"
		return
	}
	json.Unmarshal(govcresponse, &govc_vm_info)
	if len(govc_vm_info.Vms) == 1 {
		for _, val := range govc_vm_info.Vms[0].Config.ExtraConfig {
			if val.Key == "pxd.templateid" {
				localtemplateid = val.Value
				//fmt.Printf("Local Template Version\t%s\n", val.Value)
			}
		}
		if localtemplateid != "" {
			c <- localtemplateid
			return
		} else {
			fmt.Printf("Warning: found local installed px-deploy template, but did not get value pxd.templateid\nPlease run px-deploy vsphere-init\n\n")
			c <- "0"
		}
	} else {
		fmt.Printf("cannot get local template version (result %v)\n", len(govc_vm_info.Vms))
		c <- "0"
		return
	}
}

func vsphere_get_node_ip(config *Config, node string) string {
	vrest := NewVsphereRestClient(config.Vsphere_Host, config.Vsphere_User, config.Vsphere_Password)
	nodeid := strings.Split(config.Vsphere_Nodemap[node], ",")

	if len(nodeid) != 2 {
		return ""
	}
	err := vrest.Login()
	defer vrest.Logout()

	if err != nil {
		die("error creating vsphere session\n")
	} else {
		nip, err := getvSphereNodeIp(nodeid[0], nodeid[1], &vrest)
		if err != nil {
			fmt.Printf("IP error %s \n", error(err))
		} else {
			return nip
		}
	}
	return ""
}

func vsphere_destroy_vm(nodemap string, v *vsphereRestClient) {
	defer wg.Done()

	mobid := strings.Split(nodemap, ",")[0]
	transCfg := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := http.Client{Transport: transCfg}
	req, err := http.NewRequest("DELETE", "https://"+v.server+"/api/vcenter/vm/"+mobid, nil)
	if err != nil {
		fmt.Println(err)
	}

	req.Header = http.Header{
		"vmware-api-session-id": {v.sessionid},
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
	}

	if resp.StatusCode == 200 {
		return
	} else {
		fmt.Printf("VM %s delete failed. Code %s \n", mobid, resp.Status)
	}
}

// rest api only disconnects vmdk from vm. no actual deletion
func vsphere_delete_clouddrive(clouddrive Vsphere_Px_Clouddrive, v *vsphereRestClient) {
	defer wg.Done()

	transCfg := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := http.Client{Transport: transCfg}
	req, err := http.NewRequest("DELETE", "https://"+v.server+"/api/vcenter/vm/"+clouddrive.mobid+"/hardware/disk/"+clouddrive.diskid, nil)
	if err != nil {
		fmt.Println(err)
	}

	req.Header = http.Header{
		"vmware-api-session-id": {v.sessionid},
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
	}

	if resp.StatusCode == 204 {
		fmt.Printf("  disconnected clouddrive %v from VM %v\n", clouddrive.path, clouddrive.mobid)
		return
	} else {
		fmt.Printf("VM %s clouddrive %s (path %s) delete failed. Code %s \n", clouddrive.mobid, clouddrive.diskid, clouddrive.path, resp.Status)
	}
}

func vsphere_get_clouddrives(nodemap string, v *vsphereRestClient, clouddrives *[]Vsphere_Px_Clouddrive) (err error) {
	var responseobj VmInfo
	var disk Vsphere_Px_Clouddrive
	defer wg.Done()

	mobid := strings.Split(nodemap, ",")[0]

	transCfg := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := http.Client{Transport: transCfg}
	req, err := http.NewRequest("GET", "https://"+v.server+"/api/vcenter/vm/"+mobid, nil)
	if err != nil {
		fmt.Println(err)
	}
	req.Header = http.Header{
		"vmware-api-session-id": {v.sessionid},
	}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
	}
	json.Unmarshal(respBody, &responseobj)

	//fmt.Printf("DEBUG code %v state %v \n", resp.StatusCode, responseobj.Disks)

	if resp.StatusCode == 200 {
		for diskid, val := range responseobj.Disks {
			if regexp.MustCompile(`\[[A-Za-z0-9\-]*\]\ fcd\/[0-9a-f]{32}\.vmdk`).MatchString(val.Backing.Vmdk_file) {
				//fmt.Printf("Disk Path %v \n", val.Backing.Vmdk_file)
				disk.diskid = diskid
				disk.mobid = mobid
				disk.path = val.Backing.Vmdk_file
				*clouddrives = append(*clouddrives, disk)
			}
		}
	} else {
		return fmt.Errorf("Collect px cloud drive api error %v\n Response %v \n", resp.StatusCode, string(respBody))
	}
	return
}

func vsphere_get_vm_powerstate(mobid string, v *vsphereRestClient) (powerstate string, err error) {
	var responseobj VsphereVmPwrResponseObj
	transCfg := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := http.Client{Transport: transCfg}
	req, err := http.NewRequest("GET", "https://"+v.server+"/api/vcenter/vm/"+mobid+"/power", nil)
	if err != nil {
		fmt.Println(err)
	}
	req.Header = http.Header{
		"vmware-api-session-id": {v.sessionid},
	}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
	}
	json.Unmarshal(respBody, &responseobj)
	//fmt.Printf("DEBUG code %v state %v \n", resp.StatusCode, responseobj.State)
	if resp.StatusCode == 200 {
		return responseobj.State, nil
	} else {
		return "ERROR", fmt.Errorf("Power State API request returned %v", resp.StatusCode)
	}
}

func vsphere_poweroff_wait(nodemap string, v *vsphereRestClient) {
	defer wg.Done()
	var responseObj VsphereRestApiError

	mobid := strings.Split(nodemap, ",")[0]

	transCfg := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := http.Client{Transport: transCfg}

	req, err := http.NewRequest("POST", "https://"+v.server+"/api/vcenter/vm/"+mobid+"/power?action=stop", nil)
	if err != nil {
		fmt.Println(err)
	}

	req.Header = http.Header{
		"vmware-api-session-id": {v.sessionid},
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
	}

	// http 204 is perfect
	// 400 error_type "ALREADY_IN_DESIRED_STATE" is also fine
	// anything else show error

	if resp.StatusCode == 204 {
		stopped := false
		for stopped != true {
			time.Sleep(2 * time.Second)
			pwrstate, err := vsphere_get_vm_powerstate(mobid, v)
			if err != nil {
				fmt.Printf(" %v \n", err)
				stopped = true
			}
			if pwrstate == "POWERED_OFF" {
				stopped = true
			}
		}
		return
	} else if resp.StatusCode == 400 {
		json.Unmarshal(respBody, &responseObj)
		if responseObj.Error_type == "ALREADY_IN_DESIRED_STATE" {
			fmt.Printf("  Info: VM %s already powered off\n", mobid)
		} else {
			fmt.Printf("VM Poweroff failed 400. Code %s resp '%s'\n", resp.Status, responseObj.Error_type)
		}
	} else {
		fmt.Printf("VM Poweroff failed. Code %s resp '%s'\n", resp.Status, string(respBody))
	}
}

func vsphere_import_tf_clouddrive(config *Config) {
	fmt.Println(White + "prepare to import clouddrives to tf state" + Reset)
	cmd := exec.Command("terraform", "-chdir=/px-deploy/.px-deploy/tf-deployments/"+config.Name, "plan", "-var-file", ".tfvars", "-out=tfplan", "-input=false", "-parallelism=50", "-generate-config-out=disks.tf")
	//cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	errapply := cmd.Run()
	if errapply != nil {
		fmt.Println(Yellow + "ERROR: terraform plan for clouddrive disk import failed. Check validity of terraform scripts" + Reset)
		//die(errapply.Error())
		return
	}
	fmt.Println(White + "importing clouddrives to tf state" + Reset)
	cmd = exec.Command("terraform", "-chdir=/px-deploy/.px-deploy/tf-deployments/"+config.Name, "apply", "-input=false", "-parallelism=50", "-auto-approve", "tfplan")
	//cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	errapply = cmd.Run()
	if errapply != nil {
		fmt.Println(Yellow + "ERROR: terraform import for clouddrive disk import failed. Check validity of terraform scripts" + Reset)
		//die(errapply.Error())
		return
	}
}

func vsphere_prepare_destroy(config *Config) error {
	vrest := NewVsphereRestClient(config.Vsphere_Host, config.Vsphere_User, config.Vsphere_Password)
	err := vrest.Login()
	defer vrest.Logout()

	if err != nil {
		return fmt.Errorf("prepare destroy: vSphere Login Failed")
	}

	clusters, _ := strconv.Atoi(config.Clusters)
	nodes, _ := strconv.Atoi(config.Nodes)

	fmt.Printf("collecting px clouddrives\n")
	var clouddrives []Vsphere_Px_Clouddrive
	for i := 1; i <= clusters; i++ {
		wg.Add(1)
		go vsphere_get_clouddrives(config.Vsphere_Nodemap[fmt.Sprintf("%s-master-%v", config.Name, i)], &vrest, &clouddrives)

		for j := 1; j <= nodes; j++ {
			wg.Add(1)
			go vsphere_get_clouddrives(config.Vsphere_Nodemap[fmt.Sprintf("%s-node-%v-%v", config.Name, i, j)], &vrest, &clouddrives)
		}
	}
	wg.Wait()
	fmt.Printf("  found %v clouddrives \n", len(clouddrives))
	fmt.Printf("clouddrive collection finished\n")

	if len(clouddrives) > 0 {
		file, err := os.OpenFile("/px-deploy/.px-deploy/tf-deployments/"+config.Name+"/import.tf", os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			die(err.Error())
		}
		defer file.Close()

		// as we cant delete clouddrive vmdk using rest api we import them to tf state
		for i, val := range clouddrives {
			_, err = file.WriteString(fmt.Sprintf("import {\n  to = vsphere_virtual_disk.pxcd%v\n  id = \"/%s/%s\"\n}\n", i, config.Vsphere_Datacenter, val.path))
			if err != nil {
				die(err.Error())
			}
		}

		//import clouddrives into tf state
		vsphere_import_tf_clouddrive(config)
		os.Rename("/px-deploy/.px-deploy/tf-deployments/"+config.Name+"/import.tf", "/px-deploy/.px-deploy/tf-deployments/"+config.Name+"/import.tf.done")

		fmt.Printf("waiting to power off VMs\n")
		for i := 1; i <= clusters; i++ {
			wg.Add(1)
			go vsphere_poweroff_wait(config.Vsphere_Nodemap[fmt.Sprintf("%s-master-%v", config.Name, i)], &vrest)

			for j := 1; j <= nodes; j++ {
				wg.Add(1)
				go vsphere_poweroff_wait(config.Vsphere_Nodemap[fmt.Sprintf("%s-node-%v-%v", config.Name, i, j)], &vrest)
			}
		}
		wg.Wait()
		fmt.Printf("VM poweroff finished\n")
		fmt.Printf("disconnecting clouddrives \n")
		for _, val := range clouddrives {
			wg.Add(1)
			go vsphere_delete_clouddrive(val, &vrest)
		}
		wg.Wait()
		fmt.Printf("disconnect clouddrives finished \n")
	}
	/*
		fmt.Printf("Waiting for VM deletion\n")
		for i := 1; i <= clusters; i++ {
			wg.Add(1)
			go vsphere_destroy_vm(config.Vsphere_Nodemap[fmt.Sprintf("%s-master-%v", config.Name, i)], &vrest)

			for j := 1; j <= nodes; j++ {
				wg.Add(1)
				go vsphere_destroy_vm(config.Vsphere_Nodemap[fmt.Sprintf("%s-node-%v-%v", config.Name, i, j)], &vrest)
			}
		}
		wg.Wait()
		fmt.Printf("VM delete finished\n")
	*/
	return nil
}

func vsphere_init() {
	var govc_opts []string

	config := parse_yaml("defaults.yml")
	fmt.Printf("Hint: there is a way faster way to deploy the base template. \n  Please follow this documentation:\n  https://github.com/purestorage-openconnect/px-deploy/tree/master/docs/cloud/vsphere/README.md \n  Or get a coffee now\n")
	checkvar := []string{"vsphere_compute_resource", "vsphere_datacenter", "vsphere_datastore", "vsphere_host", "vsphere_network", "vsphere_resource_pool", "vsphere_template", "vsphere_user", "vsphere_password", "vsphere_repo"}
	emptyVars := isEmpty(config.Vsphere_Compute_Resource, config.Vsphere_Datacenter, config.Vsphere_Datastore, config.Vsphere_Host, config.Vsphere_Network, config.Vsphere_Resource_Pool, config.Vsphere_Template, config.Vsphere_User, config.Vsphere_Password, config.Vsphere_Repo)
	if len(emptyVars) > 0 {
		for _, i := range emptyVars {
			fmt.Printf("%splease set \"%s\" in defaults.yml %s\n Canceled import \n", Red, checkvar[i], Reset)
		}
		return
	}

	vsphere_template_dir := path.Dir(config.Vsphere_Template)
	vsphere_template_base := path.Base(config.Vsphere_Template)

	path_json_opts := filepath.Join(os.TempDir(), "options.json")
	json_opts := []byte(fmt.Sprintf("{\"NetworkMapping\":[{\"Name\": \"VM Network (Standard)\",\"Network\": \"%s\"}]}", config.Vsphere_Network))

	err := os.WriteFile(path_json_opts, json_opts, 0644)
	if err != nil {
		fmt.Println(Red + "ERROR writing import options file" + Reset)
		return
	}

	govc_opts = append(govc_opts, fmt.Sprintf("GOVC_URL=%s", config.Vsphere_Host))
	govc_opts = append(govc_opts, fmt.Sprintf("GOVC_INSECURE=1"))
	govc_opts = append(govc_opts, fmt.Sprintf("GOVC_RESOURCE_POOL=%s", config.Vsphere_Resource_Pool))
	govc_opts = append(govc_opts, fmt.Sprintf("GOVC_CLUSTER=%s", config.Vsphere_Compute_Resource))
	govc_opts = append(govc_opts, fmt.Sprintf("GOVC_USERNAME=%s", config.Vsphere_User))
	govc_opts = append(govc_opts, fmt.Sprintf("GOVC_PASSWORD=%s", config.Vsphere_Password))
	govc_opts = append(govc_opts, fmt.Sprintf("GOVC_DATASTORE=%s", config.Vsphere_Datastore))
	govc_opts = append(govc_opts, fmt.Sprintf("GOVC_DATACENTER=%s", config.Vsphere_Datacenter))
	if vsphere_template_dir != "." {
		govc_opts = append(govc_opts, fmt.Sprintf("GOVC_FOLDER=%s", vsphere_template_dir))
	}
	fmt.Printf("Importing new template to VM %s_tmp\n  (source %stemplate.ova)\n", config.Vsphere_Template, config.Vsphere_Repo)
	cmd := exec.Command("govc", "import.ova", "-options="+path_json_opts, "-name="+vsphere_template_base+"_tmp", config.Vsphere_Repo+"template.ova")
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, govc_opts...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		fmt.Println(Red + "ERROR importing px-deploy base template" + Reset)
		return
	}
	fmt.Printf("Removing old template VM %s\n", config.Vsphere_Template)
	cmd = exec.Command("govc", "vm.destroy", vsphere_template_base)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, govc_opts...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()

	fmt.Printf("Renaming new template VM to %s\n", config.Vsphere_Template)
	cmd = exec.Command("govc", "vm.change", "-vm="+vsphere_template_base+"_tmp", "-name="+vsphere_template_base)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, govc_opts...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		fmt.Println(Red + "ERROR renaming base template" + Reset)
		return
	}

	fmt.Printf("Converting base template VM %s to vSphere template\n", config.Vsphere_Template)
	cmd = exec.Command("govc", "vm.markastemplate", vsphere_template_base)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, govc_opts...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		fmt.Println(Red + "ERROR migrating base image to template" + Reset)
		return
	}
}
