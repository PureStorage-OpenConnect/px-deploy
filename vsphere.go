package main

// vSphere helper functions rewritten to use the vSphere SOAP API (vim25)
// through the govmomi library.
//
//   go get github.com/vmware/govmomi
//
// Notes:
//  - The "mobid" stored in Vsphere_Nodemap ("vm-123,<mac>") is the same value as the
//    SOAP ManagedObjectReference.Value of the VM, so the nodemap format is unchanged.
//  - govc is no longer needed for template-version lookup, destroy, rename or
//    markastemplate. It is still used for import.ova, because OVA/OVF import is
//    implemented inside the govc CLI and is not part of the public govmomi API.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
)

// ---------------------------------------------------------------------------
// types
// ---------------------------------------------------------------------------

type VsphereClient interface{}

type vsphereClient struct {
	server   string
	user     string
	password string
	client   *govmomi.Client
}

type Vsphere_Px_Clouddrive struct {
	mobid     string
	device    types.BaseVirtualDevice // disk device, used for detaching
	datastore string
	path      string // path without extension, e.g. fcd/ab/cd/<32hex>
	extension string
}

type cloudDriveResult struct {
	drives []Vsphere_Px_Clouddrive
	err    error
}

// Configurable retry settings
type RetryConfig struct {
	MaxRetries int
	BaseDelay  time.Duration
}

// ---------------------------------------------------------------------------
// session handling
// ---------------------------------------------------------------------------

func NewVsphereClient(server string, user string, password string) *vsphereClient {
	return &vsphereClient{server: server, user: user, password: password}
}

// Login opens a SOAP session against https://<server>/sdk
func (v *vsphereClient) Login() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	u, err := soap.ParseURL(v.server) // adds https:// and /sdk if missing
	if err != nil {
		fmt.Println(err)
		return err
	}
	u.User = url.UserPassword(v.user, v.password)

	// insecure = true (same as InsecureSkipVerify in the REST version)
	c, err := govmomi.NewClient(ctx, u, true)
	if err != nil {
		fmt.Printf("Login failed: %v\n", err)
		return err
	}
	v.client = c
	return nil
}

func (v *vsphereClient) Logout() error {
	if v.client == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	err := v.client.Logout(ctx)
	if err != nil {
		fmt.Printf("Logout failed: %v\n", err)
		return err
	}
	v.client = nil
	return nil
}

// vmFromNodemap builds a VM object from a "mobid,mac" nodemap entry
func (v *vsphereClient) vmFromNodemap(nodemap string) (*object.VirtualMachine, error) {
	mobid := strings.Split(nodemap, ",")[0]
	if mobid == "" {
		return nil, fmt.Errorf("empty mobid in nodemap entry %q", nodemap)
	}
	if v.client == nil {
		return nil, errors.New("no active vSphere session")
	}
	return object.NewVirtualMachine(v.client.Client, types.ManagedObjectReference{
		Type:  "VirtualMachine",
		Value: mobid,
	}), nil
}

// ---------------------------------------------------------------------------
// VM IP lookup
// ---------------------------------------------------------------------------

func getvSphereNodeIp(mobId string, mac string, v *vsphereClient) (ip string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	vm, err := v.vmFromNodemap(mobId)
	if err != nil {
		return "", err
	}

	var props mo.VirtualMachine
	if err := vm.Properties(ctx, vm.Reference(), []string{"guest.net"}, &props); err != nil {
		fmt.Printf("Request for VM IP failed: %v\n", err)
		return "", err
	}

	if props.Guest != nil {
		for _, nic := range props.Guest.Net {
			if !strings.EqualFold(nic.MacAddress, mac) || nic.IpConfig == nil {
				continue
			}
			for _, a := range nic.IpConfig.IpAddress {
				// SOAP reports the state in lower case ("preferred")
				if strings.EqualFold(a.State, "preferred") {
					return a.IpAddress, nil
				}
			}
		}
	}

	return "", fmt.Errorf("API did not get IP for node %s (mac %s)\n", mobId, mac)
}

func vsphere_get_node_ip(config *Config, node string) string {
	vrest := NewVsphereClient(config.Vsphere_Host, config.Vsphere_User, config.Vsphere_Password)
	nodeid := strings.Split(config.Vsphere_Nodemap[node], ",")

	if len(nodeid) != 2 {
		return ""
	}
	if err := vrest.Login(); err != nil {
		die("error creating vsphere session\n")
		return ""
	}
	defer vrest.Logout()

	nip, err := getvSphereNodeIp(nodeid[0], nodeid[1], vrest)
	if err != nil {
		fmt.Printf("IP error %s \n", error(err))
		return ""
	}
	return nip
}

// ---------------------------------------------------------------------------
// terraform variables (unchanged)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// template version check
// ---------------------------------------------------------------------------

// displays a message if local pxd template version differs from repository
// checked against an already deployed master-1 instance
// we cannot check pxd-templateid of a vsphere template as the API does not show these as VM
// so we'll check after deployment when VMs have been created
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
		fmt.Printf("%s px-deploy template (current version %s) update available (version %s).\nYou need to run 'px-deploy vsphere-init' to update and re-create your deployment%s \n", Red, lt, rt, Reset)
	} else {
		fmt.Printf("%s Info: local px-deploy template up-to-date (Version %s)%s \n", Green, lt, Reset)
	}
}

// remote version is fetched from the template repository over plain HTTP (not a vSphere call)
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
	defer resp.Body.Close()
	remoteTemplateVersion, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("CheckTemplateVersion: error reading http s3 request body %s \n", error(err))
		c <- "0"
		return
	}
	if resp.StatusCode == 200 {
		c <- string(remoteTemplateVersion)
	} else {
		fmt.Printf("CheckTemplateVersion: Error: s3 request returned statuscode %v \n", resp.StatusCode)
		c <- "0"
		return
	}
}

// The SOAP API exposes VM.config.extraConfig directly, so govc is no longer required.
func vsphere_get_localtemplate(config *Config, c chan string) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	vrest := NewVsphereClient(config.Vsphere_Host, config.Vsphere_User, config.Vsphere_Password)
	if err := vrest.Login(); err != nil {
		fmt.Println(Yellow + "Cannot get local pxd templateid. vSphere login failed" + Reset)
		c <- "0"
		return
	}
	defer vrest.Logout()

	finder := find.NewFinder(vrest.client.Client, true)
	dc, err := finder.Datacenter(ctx, config.Vsphere_Datacenter)
	if err != nil {
		fmt.Printf("%sCannot get local pxd templateid. Datacenter lookup failed: %v%s\n", Yellow, err, Reset)
		c <- "0"
		return
	}
	finder.SetDatacenter(dc)

	vmName := config.Name + "-master-1"
	if config.Vsphere_Folder != "" {
		vmName = path.Join(config.Vsphere_Folder, vmName)
	}
	vm, err := finder.VirtualMachine(ctx, vmName)
	if err != nil {
		fmt.Printf("cannot get local template version (%v)\n", err)
		c <- "0"
		return
	}

	var props mo.VirtualMachine
	if err := vm.Properties(ctx, vm.Reference(), []string{"config.extraConfig"}, &props); err != nil || props.Config == nil {
		fmt.Println(Yellow + "Cannot get local pxd templateid. Property request failed" + Reset)
		c <- "0"
		return
	}

	var localtemplateid string
	for _, opt := range props.Config.ExtraConfig {
		ov := opt.GetOptionValue()
		if ov != nil && ov.Key == "pxd.templateid" {
			localtemplateid = fmt.Sprint(ov.Value)
		}
	}
	if localtemplateid != "" {
		c <- localtemplateid
		return
	}
	fmt.Printf("Warning: found local installed px-deploy template, but did not get value pxd.templateid\nPlease run px-deploy vsphere-init\n\n")
	c <- "0"
}

// ---------------------------------------------------------------------------
// VM destroy / power / clouddrive collection
// ---------------------------------------------------------------------------

func vsphere_destroy_vm(nodemap string, v *vsphereClient) {
	defer wg.Done()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	vm, err := v.vmFromNodemap(nodemap)
	if err != nil {
		fmt.Println(err)
		return
	}
	task, err := vm.Destroy(ctx)
	if err == nil {
		err = task.Wait(ctx)
	}
	if err != nil {
		fmt.Printf("VM %s delete failed: %v\n", vm.Reference().Value, err)
	}
}

// retryWithBackoff runs fn up to MaxRetries+1 times with exponential backoff.
func retryWithBackoff(ctx context.Context, cfg RetryConfig, fn func(ctx context.Context) error) error {
	var lastErr error

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			// exponential backoff delay (e.g., 500ms, 1s, 2s)
			backoff := cfg.BaseDelay * (1 << (attempt - 1))
			fmt.Printf("Attempt %d failed (%v). Retrying in %v...\n", attempt, lastErr, backoff)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return fmt.Errorf("retry cancelled by context: %w", ctx.Err())
			}
		}

		attemptCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		err := fn(attemptCtx)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
	}
	return fmt.Errorf("request failed after %d retries: %w", cfg.MaxRetries, lastErr)
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not found") || strings.Contains(msg, "filenotfound")
}

func vsphere_get_clouddrives(nodemap string, v *vsphereClient, results chan<- cloudDriveResult) {
	defer wg.Done()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	vm, err := v.vmFromNodemap(nodemap)
	if err != nil {
		results <- cloudDriveResult{err: err}
		return
	}
	mobid := vm.Reference().Value

	var props mo.VirtualMachine
	if err := vm.Properties(ctx, vm.Reference(), []string{"config.hardware.device"}, &props); err != nil {
		results <- cloudDriveResult{err: fmt.Errorf("vm %s: collect px cloud drive api error: %w", mobid, err)}
		return
	}
	if props.Config == nil {
		results <- cloudDriveResult{err: fmt.Errorf("vm %s: no config returned", mobid)}
		return
	}

	re := regexp.MustCompile(`\[[A-Za-z0-9\-]*\]\ fcd\/[_a-f0-9/]*[0-9a-f]{32}\.vmdk`)

	var found []Vsphere_Px_Clouddrive
	devices := object.VirtualDeviceList(props.Config.Hardware.Device)
	for _, d := range devices.SelectByType((*types.VirtualDisk)(nil)) {
		disk := d.(*types.VirtualDisk)

		fileBacking, ok := disk.Backing.(types.BaseVirtualDeviceFileBackingInfo)
		if !ok {
			continue
		}
		vmdk := fileBacking.GetVirtualDeviceFileBackingInfo().FileName
		if !re.MatchString(vmdk) {
			continue
		}

		// format "[datastore] path/filename.vmdk"
		var dp object.DatastorePath
		if !dp.FromString(vmdk) {
			results <- cloudDriveResult{err: fmt.Errorf("vm %s: invalid vmdk disk format: %s", mobid, vmdk)}
			return
		}
		lastDotIdx := strings.LastIndex(dp.Path, ".")
		if lastDotIdx == -1 {
			results <- cloudDriveResult{err: fmt.Errorf("vm %s: invalid vmdk disk format: missing file extension", mobid)}
			return
		}

		found = append(found, Vsphere_Px_Clouddrive{
			mobid:     mobid,
			device:    d,
			datastore: dp.Datastore,
			path:      dp.Path[:lastDotIdx],
			extension: dp.Path[lastDotIdx+1:],
		})
	}
	results <- cloudDriveResult{drives: found}
}

func vsphere_get_vm_powerstate(mobid string, v *vsphereClient) (powerstate string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	vm, err := v.vmFromNodemap(mobid)
	if err != nil {
		return "ERROR", err
	}
	state, err := vm.PowerState(ctx)
	if err != nil {
		return "ERROR", fmt.Errorf("Power State API request failed: %w", err)
	}
	return string(state), nil // poweredOn | poweredOff | suspended
}

func vsphere_poweroff_wait(nodemap string, v *vsphereClient) {
	defer wg.Done()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	vm, err := v.vmFromNodemap(nodemap)
	if err != nil {
		fmt.Println(err)
		return
	}
	mobid := vm.Reference().Value

	// SOAP raises InvalidPowerState if the VM is already off, so check first
	state, err := vm.PowerState(ctx)
	if err != nil {
		fmt.Printf("VM %s: cannot get power state: %v\n", mobid, err)
		return
	}
	if state == types.VirtualMachinePowerStatePoweredOff {
		fmt.Printf("  Info: VM %s already powered off\n", mobid)
		return
	}

	task, err := vm.PowerOff(ctx)
	if err != nil {
		fmt.Printf("VM Poweroff failed for %s: %v\n", mobid, err)
		return
	}
	// task.Wait blocks until the power-off task has finished
	if err := task.Wait(ctx); err != nil {
		fmt.Printf("VM Poweroff failed for %s: %v\n", mobid, err)
	}
}

// ---------------------------------------------------------------------------
// prepare destroy
// ---------------------------------------------------------------------------

func vsphere_prepare_destroy(config *Config) error {
	vrest := NewVsphereClient(config.Vsphere_Host, config.Vsphere_User, config.Vsphere_Password)
	if err := vrest.Login(); err != nil {
		return fmt.Errorf("prepare destroy: vSphere Login Failed")
	}
	defer vrest.Logout()

	clusters, _ := strconv.Atoi(config.Clusters)
	nodes, _ := strconv.Atoi(config.Nodes)

	fmt.Printf("collecting px clouddrives\n")
	results := make(chan cloudDriveResult)

	for i := 1; i <= clusters; i++ {
		wg.Add(1)
		go vsphere_get_clouddrives(config.Vsphere_Nodemap[fmt.Sprintf("%s-master-%v", config.Name, i)], vrest, results)

		for j := 1; j <= nodes; j++ {
			wg.Add(1)
			go vsphere_get_clouddrives(config.Vsphere_Nodemap[fmt.Sprintf("%s-node-%v-%v", config.Name, i, j)], vrest, results)
		}
	}
	go func() {
		wg.Wait()
		close(results)
	}()
	// single consumer: the only place the slice is modified
	var clouddrives []Vsphere_Px_Clouddrive
	var errs []error
	for r := range results {
		if r.err != nil {
			errs = append(errs, r.err)
			continue
		}
		clouddrives = append(clouddrives, r.drives...)
	}

	if len(errs) > 0 {
		return fmt.Errorf("collecting clouddrives failed: %w", errors.Join(errs...))
	}

	fmt.Printf("  found %v clouddrives \n", len(clouddrives))
	fmt.Printf("clouddrive collection finished\n\n")

	if len(clouddrives) == 0 {
		return nil
	}

	fmt.Printf("waiting to power off VMs\n")
	for i := 1; i <= clusters; i++ {
		wg.Add(1)
		go vsphere_poweroff_wait(config.Vsphere_Nodemap[fmt.Sprintf("%s-master-%v", config.Name, i)], vrest)

		for j := 1; j <= nodes; j++ {
			wg.Add(1)
			go vsphere_poweroff_wait(config.Vsphere_Nodemap[fmt.Sprintf("%s-node-%v-%v", config.Name, i, j)], vrest)
		}
	}
	wg.Wait()
	fmt.Printf("VM poweroff finished\n\n")
	fmt.Printf("disconnecting & deleting clouddrives \n")

	ctx := context.Background()
	httpconfig := RetryConfig{
		MaxRetries: 10,
		BaseDelay:  500 * time.Millisecond,
	}

	// datacenter is required by the FileManager
	finder := find.NewFinder(vrest.client.Client, true)
	dc, err := finder.Datacenter(ctx, config.Vsphere_Datacenter)
	if err != nil {
		return fmt.Errorf("prepare destroy: datacenter lookup failed: %w", err)
	}
	fm := object.NewFileManager(vrest.client.Client)

	deleteDatastoreFile := func(name string) error {
		return retryWithBackoff(ctx, httpconfig, func(c context.Context) error {
			task, err := fm.DeleteDatastoreFile(c, name, dc)
			if err != nil {
				return err
			}
			err = task.Wait(c)
			if isNotFound(err) {
				return nil // already gone
			}
			return err
		})
	}

	for _, val := range clouddrives {
		fmt.Printf("  Disconnect & Delete: %s[-flat].%s\n", val.path, val.extension)

		vm := object.NewVirtualMachine(vrest.client.Client, types.ManagedObjectReference{Type: "VirtualMachine", Value: val.mobid})

		// detach disk from VM, keepFiles=true leaves the vmdk on the datastore
		err := retryWithBackoff(ctx, httpconfig, func(c context.Context) error {
			return vm.RemoveDevice(c, true, val.device)
		})
		if err != nil {
			fmt.Printf("  Disk disconnect failed: %v\n", err)
		}

		// delete descriptor (.vmdk) and extent (-flat.vmdk)
		err = deleteDatastoreFile(fmt.Sprintf("[%s] %s.%s", val.datastore, val.path, val.extension))
		if err != nil {
			fmt.Printf("    Disk delete failed: %v\n", err)
		}
		err = deleteDatastoreFile(fmt.Sprintf("[%s] %s-flat.%s", val.datastore, val.path, val.extension))
		if err != nil {
			fmt.Printf("    Disk delete failed: %v\n", err)
		}
	}
	fmt.Printf("disconnect & delete clouddrives finished \n\n")

	seen := make(map[string]struct{})

	fmt.Printf("Deleting %s /fcd/[subdirectories]\n", config.Vsphere_Datastore)
	for _, cd := range clouddrives {
		idx := strings.Index(cd.path, "fcd/")
		if idx == -1 {
			continue
		}
		// need another path component after "fcd/"
		if !strings.Contains(cd.path[idx+len("fcd/"):], "/") {
			continue
		}
		dir := path.Dir(cd.path) // e.g. "fcd/ab/cd"
		if _, ok := seen[dir]; ok {
			continue
		}
		seen[dir] = struct{}{}
		fmt.Printf(" deleting path %s\n", dir)
		if err := deleteDatastoreFile(fmt.Sprintf("[%s] %s", config.Vsphere_Datastore, dir)); err != nil {
			fmt.Printf("    Disk delete failed: %v\n", err)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// template import
// ---------------------------------------------------------------------------

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

	// --- OVA import: still done with govc (OVF import lives in the govc CLI, not in govmomi's public API)
	govc_opts = append(govc_opts, fmt.Sprintf("GOVC_URL=%s", config.Vsphere_Host))
	govc_opts = append(govc_opts, "GOVC_INSECURE=1")
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

	// --- everything below uses the SOAP API
	ctx := context.Background()
	vrest := NewVsphereClient(config.Vsphere_Host, config.Vsphere_User, config.Vsphere_Password)
	if err := vrest.Login(); err != nil {
		fmt.Println(Red + "ERROR creating vsphere session" + Reset)
		return
	}
	defer vrest.Logout()

	finder := find.NewFinder(vrest.client.Client, true)
	dc, err := finder.Datacenter(ctx, config.Vsphere_Datacenter)
	if err != nil {
		fmt.Printf("%sERROR finding datacenter: %v%s\n", Red, err, Reset)
		return
	}
	finder.SetDatacenter(dc)

	fmt.Printf("Removing old template VM %s\n", config.Vsphere_Template)
	if oldVm, err := finder.VirtualMachine(ctx, config.Vsphere_Template); err != nil {
		fmt.Printf("  (no old template found: %v)\n", err)
	} else {
		task, err := oldVm.Destroy(ctx)
		if err == nil {
			err = task.Wait(ctx)
		}
		if err != nil {
			fmt.Printf("  could not remove old template: %v\n", err)
		}
	}

	fmt.Printf("Renaming new template VM to %s\n", config.Vsphere_Template)
	tmpVm, err := finder.VirtualMachine(ctx, path.Join(vsphere_template_dir, vsphere_template_base+"_tmp"))
	if err != nil {
		fmt.Println(Red + "ERROR renaming base template" + Reset)
		return
	}
	task, err := tmpVm.Rename(ctx, vsphere_template_base)
	if err == nil {
		err = task.Wait(ctx)
	}
	if err != nil {
		fmt.Println(Red + "ERROR renaming base template" + Reset)
		return
	}

	fmt.Printf("Converting base template VM %s to vSphere template\n", config.Vsphere_Template)
	if err := tmpVm.MarkAsTemplate(ctx); err != nil {
		fmt.Println(Red + "ERROR migrating base image to template" + Reset)
		return
	}
}
