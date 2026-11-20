package main

// Helpers that turn failures into messages a user can act on: what happened,
// the likely cause, and the next command. Expected conditions (a missing
// deployment, a VM that is still booting, rejected cloud credentials) end with
// die(), never panic(), so the user sees one sentence instead of a stack trace.

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const pxdHome = "/px-deploy/.px-deploy"

// cloud-init's disable_root stub ("Please login as the user ... rather than
// the user root") exits with this code until px-deploy's setup replaces
// root's authorized_keys.
const cloudInitDisabledRootExit = 142

// sshOpts are used for every non-interactive ssh px-deploy runs. LogLevel=ERROR
// (not -q) keeps ssh's own reason for a failure; BatchMode stops ssh from
// prompting for a password; ConnectTimeout stops an unreachable VM from
// holding a command for the TCP timeout.
var sshOpts = []string{"-oLogLevel=ERROR", "-oBatchMode=yes", "-oConnectTimeout=10", "-oStrictHostKeyChecking=no"}

// user_path shows a path the way the user sees it on their machine.
func user_path(p string) string {
	if strings.HasPrefix(p, pxdHome) {
		return "~/.px-deploy" + strings.TrimPrefix(p, pxdHome)
	}
	if !filepath.IsAbs(p) {
		return "~/.px-deploy/" + p
	}
	return p
}

func deployment_path(name string) string {
	return pxdHome + "/deployments/" + name + ".yml"
}

func key_path(config *Config) string {
	return "keys/id_rsa." + config.Cloud + "." + config.Name
}

// load_deployment reads a deployment by name, or explains why it can't.
func load_deployment(name string) Config {
	if name == "" {
		die("Specify a deployment with -n <name>. 'px-deploy list' shows your deployments.")
	}
	if _, err := os.Stat(deployment_path(name)); errors.Is(err, os.ErrNotExist) {
		die(fmt.Sprintf("No deployment named '%s'. 'px-deploy list' shows your deployments.", name))
	}
	return parse_yaml(deployment_path(name))
}

// created_ago says how long ago a deployment was created, from its file.
func created_ago(name string) string {
	info, err := os.Stat(deployment_path(name))
	if err != nil {
		return ""
	}
	return "created " + human_duration(time.Since(info.ModTime())) + " ago"
}

func human_duration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "less than a minute"
	case d < 2*time.Minute:
		return "1 minute"
	case d < time.Hour:
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	case d < 2*time.Hour:
		return "1 hour"
	case d < 48*time.Hour:
		return fmt.Sprintf("%d hours", int(d.Hours()))
	default:
		return fmt.Sprintf("%d days", int(d.Hours()/24))
	}
}

func in_parens(s string) string {
	if s == "" {
		return ""
	}
	return " (" + s + ")"
}

// reach_problem checks what can be checked before calling ssh. It returns ""
// when ssh is worth trying.
func reach_problem(config *Config, node string, ip string) string {
	if _, err := os.Stat(key_path(config)); errors.Is(err, os.ErrNotExist) {
		return fmt.Sprintf("cannot reach the VMs: the deployment's SSH key %s is missing.", user_path(key_path(config)))
	}
	if ip == "" {
		return fmt.Sprintf("%s has no running instance%s. It may still be starting, it may be stopped ('px-deploy start -n %s' on AWS), or it was removed outside px-deploy.", node, in_parens(created_ago(config.Name)), config.Name)
	}
	return ""
}

// ssh_exit_code returns the exit code of a finished ssh, or -1 if it did not run.
func ssh_exit_code(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

// explain_ssh_failure turns a failed ssh to a master into one sentence. It is
// a pure function of ssh's exit code and output so it can be tested without
// a VM.
func explain_ssh_failure(node string, ip string, age string, code int, output string) string {
	where := fmt.Sprintf("%s (%s)", node, ip)
	when := in_parens(age)
	out := strings.TrimSpace(output)
	lower := strings.ToLower(out)
	switch {
	case code == cloudInitDisabledRootExit:
		return fmt.Sprintf("%s is booting; px-deploy's setup has not started yet%s. Check again in a minute.", where, when)
	case code == 255 && (strings.Contains(lower, "connection refused") || strings.Contains(lower, "timed out") || strings.Contains(lower, "no route to host") || strings.Contains(lower, "network is unreachable")):
		return fmt.Sprintf("%s is not accepting SSH yet%s. New VMs usually answer within 1 to 2 minutes.", where, when)
	case code == 255 && strings.Contains(lower, "permission denied"):
		return fmt.Sprintf("%s is up but does not accept the deployment's key for root yet%s. This is normal for the first minutes; if it lasts more than 10 minutes, look at /var/log/px-deploy on the VM through the cloud console.", where, when)
	case code == 1 && strings.Contains(lower, "no such file"):
		return fmt.Sprintf("%s is up; setup is starting%s.", where, when)
	}
	last := out
	if i := strings.LastIndex(out, "\n"); i >= 0 {
		last = strings.TrimSpace(out[i+1:])
	}
	if last == "" {
		last = fmt.Sprintf("ssh exited with status %d", code)
	}
	return fmt.Sprintf("could not read setup progress from %s: %s", where, last)
}

// cloud_error explains a failed cloud API call, keeping the API's own message.
func cloud_error(cloud string, action string, err error) string {
	hint := ""
	switch cloud {
	case "aws":
		hint = "Check aws_access_key_id, aws_secret_access_key and aws_region in ~/.px-deploy/defaults.yml. Commands after create read the keys from there, so a rotated or removed key shows up here."
	case "gcp":
		hint = "Check ~/.px-deploy/gcp.json and gcp_project in ~/.px-deploy/defaults.yml."
	case "azure":
		hint = "Check azure_client_id, azure_client_secret, azure_tenant_id and azure_subscription_id in ~/.px-deploy/defaults.yml."
	}
	name := map[string]string{"aws": "AWS", "gcp": "GCP", "azure": "Azure"}[cloud]
	if name == "" {
		name = cloud
	}
	return fmt.Sprintf("%s API error while %s: %v\n%s", name, action, err, hint)
}

func cloud_fail(cloud string, action string, err error) {
	die(cloud_error(cloud, action, err))
}
