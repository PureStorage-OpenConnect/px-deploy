SERVICE_ACCOUNT=pxmigration
NAMESPACE=kube-system
SERVER=$1

SERVICE_ACCOUNT_TOKEN_COUNT=$(kubectl -n ${NAMESPACE} get secret -o=jsonpath='{.items[?(@.metadata.annotations.kubernetes\.io/service-account\.name=="'${SERVICE_ACCOUNT}'")].metadata.name}' | wc -w)
if [ ${SERVICE_ACCOUNT_TOKEN_COUNT} -gt 1 ]
then
  SERVICE_ACCOUNT_TOKEN_NAME=$(kubectl -n ${NAMESPACE} get secret -o=jsonpath='{.items[?(@.metadata.annotations.kubernetes\.io/service-account\.name=="'${SERVICE_ACCOUNT}'")].metadata.name}' | awk '{for(i=1;i<=NF;i++){ if($i ~ /-token/){print $i} } }' | head -n 1)
else
  SERVICE_ACCOUNT_TOKEN_NAME=$(kubectl -n ${NAMESPACE} get secret -o=jsonpath='{.items[?(@.metadata.annotations.kubernetes\.io/service-account\.name=="'${SERVICE_ACCOUNT}'")].metadata.name}')
fi
SERVICE_ACCOUNT_TOKEN=$(kubectl -n ${NAMESPACE} get secret ${SERVICE_ACCOUNT_TOKEN_NAME} -o "jsonpath={.data.token}" | base64 --decode)
SERVICE_ACCOUNT_CERTIFICATE=$(kubectl -n ${NAMESPACE} get secret ${SERVICE_ACCOUNT_TOKEN_NAME} -o "jsonpath={.data['ca\.crt']}")

cat <<END
apiVersion: v1
kind: Config
clusters:
- name: default-cluster
  cluster:
    certificate-authority-data: ${SERVICE_ACCOUNT_CERTIFICATE}
    server: ${SERVER}
contexts:
- name: default-context
  context:
    cluster: default-cluster
    namespace: ${NAMESPACE}
    user: ${SERVICE_ACCOUNT}
current-context: default-context
users:
- name: ${SERVICE_ACCOUNT}
  user:
    token: ${SERVICE_ACCOUNT_TOKEN}
END
