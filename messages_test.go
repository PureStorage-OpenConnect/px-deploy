package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExplainSSHFailure(t *testing.T) {
	cases := []struct {
		name   string
		code   int
		output string
		want   string
	}{
		{"cloud-init root stub", 142, "Please login as the user \"rocky\" rather than the user \"root\".", "is booting; px-deploy's setup has not started yet"},
		{"connection refused", 255, "ssh: connect to host 54.1.2.3 port 22: Connection refused", "is not accepting SSH yet"},
		{"connection timed out", 255, "ssh: connect to host 54.1.2.3 port 22: Connection timed out", "is not accepting SSH yet"},
		{"no route", 255, "ssh: connect to host 54.1.2.3 port 22: No route to host", "is not accepting SSH yet"},
		{"key refused", 255, "root@54.1.2.3: Permission denied (publickey,password).", "does not accept the deployment's key for root yet"},
		{"tracking file missing", 1, "cat: /var/log/px-deploy/completed/tracking: No such file or directory", "setup is starting"},
		{"unknown, keeps ssh's last line", 255, "kex_exchange_identification: read: Connection reset by peer", "could not read setup progress from master-1 (54.1.2.3): kex_exchange_identification: read: Connection reset by peer"},
		{"unknown, no output", 7, "", "ssh exited with status 7"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := explain_ssh_failure("master-1", "54.1.2.3", "created 2 minutes ago", c.code, c.output)
			if !strings.Contains(got, c.want) {
				t.Errorf("got %q, want it to contain %q", got, c.want)
			}
			if !strings.Contains(got, "master-1 (54.1.2.3)") {
				t.Errorf("got %q, want it to name the node and address", got)
			}
		})
	}
}

func TestExplainSSHFailureAge(t *testing.T) {
	if got := explain_ssh_failure("master-1", "1.2.3.4", "created 3 minutes ago", 142, ""); !strings.Contains(got, "(created 3 minutes ago)") {
		t.Errorf("age missing: %q", got)
	}
	if got := explain_ssh_failure("master-1", "1.2.3.4", "", 142, ""); strings.Contains(got, "()") {
		t.Errorf("empty age left empty parentheses: %q", got)
	}
}

func TestReachProblem(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(dir)
	config := Config{Cloud: "aws", Name: "foo"}

	if got := reach_problem(&config, "master-1", "1.2.3.4"); !strings.Contains(got, "~/.px-deploy/keys/id_rsa.aws.foo is missing") {
		t.Errorf("missing key: got %q", got)
	}
	os.MkdirAll(filepath.Join(dir, "keys"), 0755)
	os.WriteFile(filepath.Join(dir, "keys", "id_rsa.aws.foo"), []byte("k"), 0600)
	if got := reach_problem(&config, "master-1", ""); !strings.Contains(got, "master-1 has no running instance") || strings.Contains(got, "()") {
		t.Errorf("no address: got %q", got)
	}
	if got := reach_problem(&config, "master-1", "1.2.3.4"); got != "" {
		t.Errorf("reachable: got %q, want empty", got)
	}
}

func TestUserPath(t *testing.T) {
	cases := map[string]string{
		"/px-deploy/.px-deploy/logs/foo":     "~/.px-deploy/logs/foo",
		"deployments/foo.yml":                "~/.px-deploy/deployments/foo.yml",
		"/px-deploy/.px-deploy/defaults.yml": "~/.px-deploy/defaults.yml",
		"/etc/hosts":                         "/etc/hosts",
	}
	for in, want := range cases {
		if got := user_path(in); got != want {
			t.Errorf("user_path(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHumanDuration(t *testing.T) {
	cases := map[time.Duration]string{
		30 * time.Second: "less than a minute",
		90 * time.Second: "1 minute",
		7 * time.Minute:  "7 minutes",
		90 * time.Minute: "1 hour",
		5 * time.Hour:    "5 hours",
		72 * time.Hour:   "3 days",
	}
	for in, want := range cases {
		if got := human_duration(in); got != want {
			t.Errorf("human_duration(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestCloudError(t *testing.T) {
	got := cloud_error("aws", "listing the deployment's instances", errors.New("AuthFailure: AWS was not able to validate the provided access credentials"))
	for _, want := range []string{"AWS API error while listing the deployment's instances", "AuthFailure", "~/.px-deploy/defaults.yml"} {
		if !strings.Contains(got, want) {
			t.Errorf("got %q, want it to contain %q", got, want)
		}
	}
	if got := cloud_error("azure", "x", errors.New("e")); !strings.HasPrefix(got, "Azure API error") {
		t.Errorf("azure name: %q", got)
	}
}
