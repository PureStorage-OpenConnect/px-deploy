apiVersion: v1
baseDomain: ${tpl_ocp4domain}
credentialsMode: ${tpl_ocp4_credentials_mode}
compute:
- hyperthreading: Enabled
  name: worker
  platform:
    aws:
      additionalSecurityGroupIDs:
      - ${tpl_ocp_sg}
      type: ${tpl_awstype}
      iamRole: ${tpl_aws_iamrole}
  replicas: ${tpl_nodes}
controlPlane:
  hyperthreading: Enabled
  name: master
  platform: {}
  replicas: 3
metadata:
  creationTimestamp: null
  name: pxd-${tpl_configname}-${tpl_cluster}
networking:
  clusterNetwork:
  - cidr: 10.128.0.0/14
    hostPrefix: 23
  machineCIDR: ${tpl_cidr}
  networkType: OVNKubernetes
  serviceNetwork:
  - 172.30.0.0/16
platform:
  aws:
    region: ${tpl_aws_region}
    userTags:
%{ for tag_key, tag_value in tpl_aws_tag ~}
      ${tag_key} : ${tag_value}
%{ endfor ~}
    vpc:
      subnets:
      - id: ${tpl_privsubnet1}
        roles:
        - type: ClusterNode
        - type: ControlPlaneInternalLB
      - id: ${tpl_privsubnet2}
        roles:
        - type: ClusterNode
        - type: ControlPlaneInternalLB
      - id: ${tpl_privsubnet3}
        roles:
        - type: ClusterNode
        - type: ControlPlaneInternalLB
      - id: ${tpl_pubsubnet1}
        roles:
        - type: IngressControllerLB
        - type: BootstrapNode
        - type: ControlPlaneExternalLB
      - id: ${tpl_pubsubnet2}
        roles:
        - type: IngressControllerLB
        - type: BootstrapNode
        - type: ControlPlaneExternalLB
      - id: ${tpl_pubsubnet3}
        roles:
        - type: IngressControllerLB
        - type: BootstrapNode
        - type: ControlPlaneExternalLB
pullSecret: '${tpl_ocp4pullsecret}'
sshKey: '${tpl_sshkey}'

