#cloud-config
users:
  - default
  - name: rocky
    primary_group: rocky
    sudo: ALL=(ALL) NOPASSWD:ALL
    groups: sudo, wheel
    ssh_import_id: None
    lock_passwd: true
    ssh_authorized_keys: ${tpl_pub_key}

write_files:
  - encoding: b64
    content: ${tpl_priv_key}
    path: /tmp/id_rsa
    permissions: '0600'
  - content: |
     [default]
     aws_access_key_id = ${tpl_aws_access_key_id}
     aws_secret_access_key = ${tpl_aws_secret_access_key}
     %{ if tpl_aws_session_token != "" ~}
     aws_session_token = ${tpl_aws_session_token}
     %{ endif ~}
    path: /tmp/credentials
    permissions: '0600'
  
runcmd:
- while [ ! -f "/tmp/env.sh" ]; do sleep 5; done
- sleep 5
- source /tmp/env.sh
- export cloud="vsphere"
- export cluster="${tpl_cluster}"
- export KUBECONFIG=/root/.kube/config
- export HOME=/root
- while [ ! -f "/tmp/${tpl_name}_scripts.sh" ]; do sleep 5; done
- sleep 5
- chmod +x /tmp/${tpl_name}_scripts.sh
- /tmp/${tpl_name}_scripts.sh
