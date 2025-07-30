#!/bin/bash
set -e

DATA_KEY="39OH+M3FGVJxzdhJKZTR4OYHgE5XpY2QziJpo9LuDVQ="
VERSION="2.0.7"
CONTROL_NODE=""
CONTAINER_RUNTIME=""
CONJUR_HOST="conjur-conjur-oss.default.svc.cluster.local"
ENV_CONFIG=""
FILE_CONFIG=""

  while getopts ":-:" optchar "$@"; do
    case "${optchar}" in
      -)
        case "${OPTARG}" in
          control-node)
            CONTROL_NODE="${!OPTIND}"
            OPTIND=$((OPTIND + 1))
            ;;
          file-config)
            FILE_CONFIG="${!OPTIND}"
            OPTIND=$((OPTIND + 1))
            ;;
          env-config)
            ENV_CONFIG="true"
            ;;
          *)
            echo "Error: Unknown option --${OPTARG}" >&2
            exit 1
            ;;
        esac
        ;;
    esac
  done

if [[ -z "$CONTROL_NODE" ]]; then
  echo "--control-node is required (bastion or master node ip)"
  exit 1
fi

if command -v docker >/dev/null 2>&1; then
  CONTAINER_RUNTIME="docker"
elif command -v podman >/dev/null 2>&1; then
  CONTAINER_RUNTIME="podman"
else
  echo "docker or podman is not installed" >&2
  exit 1
fi


printf "=== Installing Conjur Open Source Suite Helm Chart ===\n\n"

helm repo add cyberark https://cyberark.github.io/helm-charts
helm install \
   --set dataKey="$DATA_KEY" \
   --set account.create=true \
   --set account.name=dev-cluster \
   --set-literal authenticators="authn-jwt/kube" \
   --set postgres.persistentVolume.create=false \
   --set securityContext.privileged=true \
   --set securityContext.allowPrivilegeEscalation=true \
   --set service.external.enabled=false \
   --wait \
   --debug \
   conjur \
   https://github.com/cyberark/conjur-oss-helm-chart/releases/download/v"$VERSION"/conjur-oss-"$VERSION".tgz

printf "=== Installing Conjur CSI Provider Helm Chart ===\n\n"

helm install conjur-csi-provider \
  cyberark/conjur-k8s-csi-provider \
  --wait \
  --set daemonSet.image.tag="0.2.0" \
  --set provider.name="conjur" \
  --set provider.healthPort="8080" \
  --set provider.socketDir="/var/run/secrets-store-csi-providers"

rm -rf conjur-csm-authorization
mkdir conjur-csm-authorization
chmod 777 conjur-csm-authorization

printf "=== Adding $CONTROL_NODE $CONJUR_HOST to /etc/hosts ===\n\n"

grep "$CONTROL_NODE $CONJUR_HOST" /etc/hosts || echo "$CONTROL_NODE $CONJUR_HOST" >> /etc/hosts

printf "=== Initializing Conjur at https://$CONJUR_HOST:$CONJUR_PORT ===\n\n"

CONJUR_PORT=$(kubectl get svc conjur-conjur-oss -o jsonpath='{.spec.ports[?(@.port==443)].nodePort}')
yes | $CONTAINER_RUNTIME run --rm -i -v $PWD/conjur-csm-authorization:/home/cli docker.io/cyberark/conjur-cli:8 init --self-signed -a dev-cluster -u https://$CONJUR_HOST:$CONJUR_PORT

printf "=== Logging into Conjur at https://$CONJUR_HOST:$CONJUR_PORT ===\n\n"

$CONTAINER_RUNTIME run --rm -it -v $PWD/conjur-csm-authorization:/home/cli docker.io/cyberark/conjur-cli:8 login -i admin -p $(kubectl exec deploy/conjur-conjur-oss --container=conjur-oss -- conjurctl role retrieve-key dev-cluster:user:admin)

cat <<EOF > conjur-csm-authorization/config.yaml
- !host
  id: system:serviceaccount:authorization:storage-service

- !policy
  id: csm-authorization
  body:
    - !host
      id: system:serviceaccount:authorization:storage-service
      annotations:
        authn-jwt/kube/kubernetes.io/namespace: "authorization"
        authn-jwt/kube/kubernetes.io/serviceaccount/name: "storage-service"

- !policy
  id: conjur/authn-jwt/kube
  body:
    # Webservice without an ID means the authenticator is at /authn-jwt/kube
    - !webservice

    - !variable
      id: issuer

    - !variable
      id: public-keys

    - !variable
      id: audience

    - !variable
      id: token-app-property

    - !variable
      id: identity-path

- !permit
  role: !host csm-authorization/system:serviceaccount:authorization:storage-service
  privilege: [ read, authenticate ]
  resource: !webservice conjur/authn-jwt/kube
EOF

printf "=== Loading Conjur configuration ===\n\n"

$CONTAINER_RUNTIME run --rm -it -v $PWD/conjur-csm-authorization:/home/cli docker.io/cyberark/conjur-cli:8 policy load -b root -f /home/cli/config.yaml

printf "=== Confuring Conjur JWT ===\n\n"

kubectl get --raw /openid/v1/jwks > conjur-csm-authorization/jwks.json
ISSUER=$(kubectl get --raw /.well-known/openid-configuration | jq -r '.issuer')

$CONTAINER_RUNTIME run --rm -v $PWD/conjur-csm-authorization:/home/cli docker.io/cyberark/conjur-cli:8 variable set -i conjur/authn-jwt/kube/public-keys -v "{\"type\":\"jwks\", \"value\":$(cat conjur-csm-authorization/jwks.json | jq -c .)}"
$CONTAINER_RUNTIME run --rm -v $PWD/conjur-csm-authorization:/home/cli docker.io/cyberark/conjur-cli:8 variable set -i conjur/authn-jwt/kube/issuer -v $ISSUER
$CONTAINER_RUNTIME run --rm -v $PWD/conjur-csm-authorization:/home/cli docker.io/cyberark/conjur-cli:8 variable set -i conjur/authn-jwt/kube/token-app-property -v "sub"
$CONTAINER_RUNTIME run --rm -v $PWD/conjur-csm-authorization:/home/cli docker.io/cyberark/conjur-cli:8 variable set -i conjur/authn-jwt/kube/identity-path -v csm-authorization
$CONTAINER_RUNTIME run --rm -v $PWD/conjur-csm-authorization:/home/cli docker.io/cyberark/conjur-cli:8 variable set -i conjur/authn-jwt/kube/audience -v "conjur"

printf "=== Loading and configuring Conjur secrets ===\n\n"

if [[ -n "$FILE_CONFIG" ]]; then  
cat <<EOF > conjur-csm-authorization/secrets.yaml
- !policy
  id: secrets
  body:
    - &variables []
    - !permit
      role: !host /csm-authorization/system:serviceaccount:authorization:storage-service
      privilege: [ read, execute ]
      resource: *variables
EOF

  $CONTAINER_RUNTIME run --user="root" -v $PWD/conjur-csm-authorization:/workdir -v $PWD/$FILE_CONFIG:/fileconfig/credential-config.yaml --rm mikefarah/yq -i '.[] |= (select(.id == "secrets") | .body[0] = (load("/fileconfig/credential-config.yaml") | map(.variable | . tag = "!variable" | . style="")))' /workdir/secrets.yaml
  $CONTAINER_RUNTIME run --rm -it -v $PWD/conjur-csm-authorization:/home/cli docker.io/cyberark/conjur-cli:8 policy load -b root -f /home/cli/secrets.yaml

  for i in $($CONTAINER_RUNTIME run --user="root" -v $PWD/$FILE_CONFIG:/fileconfig/credential-config.yaml --rm mikefarah/yq 'keys | .[]' /fileconfig/credential-config.yaml); do
    variable=$($CONTAINER_RUNTIME run --user="root" -v $PWD/$FILE_CONFIG:/fileconfig/credential-config.yaml --rm mikefarah/yq ".[$i].variable" /fileconfig/credential-config.yaml)
    value=$($CONTAINER_RUNTIME run --user="root" -v $PWD/$FILE_CONFIG:/fileconfig/credential-config.yaml --rm mikefarah/yq ".[$i].value" /fileconfig/credential-config.yaml)
    $CONTAINER_RUNTIME run --rm -v $PWD/conjur-csm-authorization:/home/cli docker.io/cyberark/conjur-cli:8 variable set -i secrets/$variable -v $value
  done

elif [[ -n "$ENV_CONFIG" ]]; then
cat <<EOF > conjur-csm-authorization/secrets.yaml
- !policy
  id: secrets
  body:
    - &variables
      - !variable powerflex-username
      - !variable powerflex-password
      - !variable powermax-username
      - !variable powermax-password
      - !variable powerscale-username
      - !variable powerscale-password
    - !permit
      role: !host /csm-authorization/system:serviceaccount:authorization:storage-service
      privilege: [ read, execute ]
      resource: *variables
EOF

  $CONTAINER_RUNTIME run --rm -it -v $PWD/conjur-csm-authorization:/home/cli docker.io/cyberark/conjur-cli:8 policy load -b root -f /home/cli/secrets.yaml

  if [[ -n "$PFLEX_USER" && -n "$PFLEX_PASS" ]]; then
    $CONTAINER_RUNTIME run --rm -v $PWD/conjur-csm-authorization:/home/cli docker.io/cyberark/conjur-cli:8 variable set -i secrets/powerflex-username -v $PFLEX_USER
    $CONTAINER_RUNTIME run --rm -v $PWD/conjur-csm-authorization:/home/cli docker.io/cyberark/conjur-cli:8 variable set -i secrets/powerflex-password -v $PFLEX_PASS
  fi

  if [[ -n "$PMAX_USER" && -n "$PMAX_PASS" ]]; then
    $CONTAINER_RUNTIME run --rm -v $PWD/conjur-csm-authorization:/home/cli docker.io/cyberark/conjur-cli:8 variable set -i secrets/powermax-username -v $PMAX_USER
    $CONTAINER_RUNTIME run --rm -v $PWD/conjur-csm-authorization:/home/cli docker.io/cyberark/conjur-cli:8 variable set -i secrets/powermax-password -v $PMAX_PASS
  fi

  if [[ -n "$PSCALE_USER" && -n "$PSCALE_PASS" ]]; then
    $CONTAINER_RUNTIME run --rm -v $PWD/conjur-csm-authorization:/home/cli docker.io/cyberark/conjur-cli:8 variable set -i secrets/powerscale-username -v $PSCALE_USER
    $CONTAINER_RUNTIME run --rm -v $PWD/conjur-csm-authorization:/home/cli docker.io/cyberark/conjur-cli:8 variable set -i secrets/powerscale-password -v $PSCALE_PASS
  fi
fi

printf "=== Creating SecretProviderClass at conjur-spc.yaml ===\n\n"

CONJUR_CERT=$(kubectl get secret conjur-conjur-ssl-ca-cert -o jsonpath="{.data['tls\.crt']}" | base64 -d | sed 's/^/      /')

cat <<EOF > conjur-spc.yaml
apiVersion: secrets-store.csi.x-k8s.io/v1
kind: SecretProviderClass
metadata:
  name: conjur
  namespace: authorization
spec:
  provider: conjur
  parameters:
    conjur.org/configurationVersion: 0.2.0
    account: dev-cluster
    applianceUrl: 'https://conjur-conjur-oss.default.svc.cluster.local'
    authnId: authn-jwt/kube
    sslCertificate: |
$CONJUR_CERT
EOF
