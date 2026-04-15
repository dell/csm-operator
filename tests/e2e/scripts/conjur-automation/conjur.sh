# Copyright © 2025-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#      http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

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

if command -v podman >/dev/null 2>&1; then
  CONTAINER_RUNTIME="podman"
elif command -v docker >/dev/null 2>&1; then
  CONTAINER_RUNTIME="docker"
else
  echo "podman or docker is not installed" >&2
  exit 1
fi


SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

printf "=== Generating TLS certificates for Conjur ===\n\n"

DB_PASS=$(openssl rand -base64 24 | tr -d '\n/+=')

openssl req -newkey rsa:4096 -nodes \
  -keyout conjur-ca.key -x509 -days 3650 \
  -out conjur-ca.crt -subj "/CN=conjur-oss" 2>/dev/null

openssl genrsa -out conjur-server.key 4096 2>/dev/null
openssl req -new -key conjur-server.key -out conjur-server.csr \
  -subj "/CN=conjur-conjur-oss.default.svc.cluster.local" 2>/dev/null
openssl x509 -req -days 3650 \
  -in conjur-server.csr -CA conjur-ca.crt -CAkey conjur-ca.key -CAcreateserial \
  -out conjur-server.crt \
  -extfile <(printf 'subjectAltName=DNS:conjur-conjur-oss.default.svc.cluster.local,DNS:conjur-conjur-oss,DNS:conjur-conjur-oss.default.svc') 2>/dev/null

openssl req -newkey rsa:4096 -nodes \
  -keyout conjur-db.key -x509 -days 3650 \
  -out conjur-db.crt -subj "/CN=postgres" 2>/dev/null

printf "=== Creating Conjur secrets ===\n\n"

kubectl create secret generic conjur-conjur-data-key \
  --from-literal=key="$DATA_KEY" --namespace default \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl create secret generic conjur-conjur-database-password \
  --from-literal=key="$DB_PASS" --namespace default \
  --dry-run=client -o yaml | kubectl apply -f -

DB_URL="postgres://postgres:${DB_PASS}@conjur-postgres/postgres?sslmode=require"
kubectl create secret generic conjur-conjur-database-url \
  --from-literal=key="$DB_URL" --namespace default \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl create secret tls conjur-conjur-database-ssl \
  --cert=conjur-db.crt --key=conjur-db.key --namespace default \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl create secret tls conjur-conjur-ssl-ca-cert \
  --cert=conjur-ca.crt --key=conjur-ca.key --namespace default \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl create secret tls conjur-conjur-ssl-cert \
  --cert=conjur-server.crt --key=conjur-server.key --namespace default \
  --dry-run=client -o yaml | kubectl apply -f -

printf "=== Installing Conjur Open Source Suite ===\n\n"

kubectl apply -f "$SCRIPT_DIR/manifests/conjur.yaml"
kubectl rollout status statefulset/conjur-postgres --timeout=5m
kubectl rollout status deployment/conjur-conjur-oss --timeout=10m

printf "=== Installing Conjur CSI Provider ===\n\n"

kubectl apply -f "$SCRIPT_DIR/manifests/conjur-csi-provider.yaml"
kubectl rollout status daemonset/conjur-k8s-csi-provider --timeout=5m

rm -rf conjur-csm-authorization
mkdir conjur-csm-authorization
chmod 777 conjur-csm-authorization

printf "=== Adding $CONTROL_NODE $CONJUR_HOST to /etc/hosts ===\n\n"

grep "$CONTROL_NODE $CONJUR_HOST" /etc/hosts || echo "$CONTROL_NODE $CONJUR_HOST" | sudo tee -a /etc/hosts > /dev/null

printf "=== Initializing Conjur at https://$CONJUR_HOST:$CONJUR_PORT ===\n\n"

CONJUR_PORT=$(kubectl get svc conjur-conjur-oss -o jsonpath='{.spec.ports[?(@.port==443)].nodePort}')
yes | $CONTAINER_RUNTIME run --rm -i -v $PWD/conjur-csm-authorization:/home/cli docker.io/cyberark/conjur-cli:8 init --self-signed -a dev-cluster -u https://$CONJUR_HOST:$CONJUR_PORT

printf "=== Logging into Conjur at https://$CONJUR_HOST:$CONJUR_PORT ===\n\n"

$CONTAINER_RUNTIME run --rm -it -v $PWD/conjur-csm-authorization:/home/cli docker.io/cyberark/conjur-cli:8 login -i admin -p $(kubectl exec deploy/conjur-conjur-oss --container=conjur-oss -- conjurctl role retrieve-key dev-cluster:user:admin)

cat <<EOF > conjur-csm-authorization/config.yaml
- !host
  id: system:serviceaccount:authorization:storage-service

- !host
  id: system:serviceaccount:authorization:proxy-server

- !host
  id: system:serviceaccount:authorization:tenant-service

- !host
  id: system:serviceaccount:authorization:redis

- !host
  id: system:serviceaccount:authorization:sentinel

- !policy
  id: csm-authorization
  body:
    - !host
      id: system:serviceaccount:authorization:storage-service
      annotations:
        authn-jwt/kube/kubernetes.io/namespace: "authorization"
        authn-jwt/kube/kubernetes.io/serviceaccount/name: "storage-service"
    - !host
      id: system:serviceaccount:authorization:proxy-server
      annotations:
        authn-jwt/kube/kubernetes.io/namespace: "authorization"
        authn-jwt/kube/kubernetes.io/serviceaccount/name: "proxy-server"
    - !host
      id: system:serviceaccount:authorization:tenant-service
      annotations:
        authn-jwt/kube/kubernetes.io/namespace: "authorization"
        authn-jwt/kube/kubernetes.io/serviceaccount/name: "tenant-service"
    - !host
      id: system:serviceaccount:authorization:redis
      annotations:
        authn-jwt/kube/kubernetes.io/namespace: "authorization"
        authn-jwt/kube/kubernetes.io/serviceaccount/name: "redis"
    - !host
      id: system:serviceaccount:authorization:sentinel
      annotations:
        authn-jwt/kube/kubernetes.io/namespace: "authorization"
        authn-jwt/kube/kubernetes.io/serviceaccount/name: "sentinel"

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
  role:
    - !host csm-authorization/system:serviceaccount:authorization:storage-service
    - !host csm-authorization/system:serviceaccount:authorization:proxy-server
    - !host csm-authorization/system:serviceaccount:authorization:tenant-service
    - !host csm-authorization/system:serviceaccount:authorization:redis
    - !host csm-authorization/system:serviceaccount:authorization:sentinel
  privilege: [ read, authenticate ]
  resource: !webservice conjur/authn-jwt/kube
EOF

printf "=== Loading Conjur configuration ===\n\n"

$CONTAINER_RUNTIME run --rm -it -v $PWD/conjur-csm-authorization:/home/cli docker.io/cyberark/conjur-cli:8 policy load -b root -f /home/cli/config.yaml

printf "=== Configuring Conjur JWT ===\n\n"

kubectl get --raw /openid/v1/jwks > conjur-csm-authorization/jwks.json
ISSUER=$(kubectl get --raw /.well-known/openid-configuration | jq -r '.issuer')

$CONTAINER_RUNTIME run --rm -v $PWD/conjur-csm-authorization:/home/cli docker.io/cyberark/conjur-cli:8 variable set -i conjur/authn-jwt/kube/public-keys -v "{\"type\":\"jwks\", \"value\":$(cat conjur-csm-authorization/jwks.json | jq -c .)}"
$CONTAINER_RUNTIME run --rm -v $PWD/conjur-csm-authorization:/home/cli docker.io/cyberark/conjur-cli:8 variable set -i conjur/authn-jwt/kube/issuer -v $ISSUER
$CONTAINER_RUNTIME run --rm -v $PWD/conjur-csm-authorization:/home/cli docker.io/cyberark/conjur-cli:8 variable set -i conjur/authn-jwt/kube/token-app-property -v "sub"
$CONTAINER_RUNTIME run --rm -v $PWD/conjur-csm-authorization:/home/cli docker.io/cyberark/conjur-cli:8 variable set -i conjur/authn-jwt/kube/identity-path -v csm-authorization
$CONTAINER_RUNTIME run --rm -v $PWD/conjur-csm-authorization:/home/cli docker.io/cyberark/conjur-cli:8 variable set -i conjur/authn-jwt/kube/audience -v "conjur"

printf "=== Loading and configuring Conjur secrets ===\n\n"

# '/' prepended to csm-authorization to ensure the path secret/csm-authorization is valid
if [[ -n "$FILE_CONFIG" ]]; then
cat <<EOF > conjur-csm-authorization/secrets.yaml
- !policy
  id: secrets
  body:
    - &variables []
    - !permit
      role:
        - !host /csm-authorization/system:serviceaccount:authorization:storage-service
        - !host /csm-authorization/system:serviceaccount:authorization:proxy-server
        - !host /csm-authorization/system:serviceaccount:authorization:tenant-service
        - !host /csm-authorization/system:serviceaccount:authorization:redis
        - !host /csm-authorization/system:serviceaccount:authorization:sentinel
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

# '/' prepended to csm-authorization to ensure the path secret/csm-authorization is valid
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
      - !variable powerstore-username
      - !variable powerstore-password
      - !variable redis-username
      - !variable redis-password
      - !variable config-object
    - !permit
      role:
        - !host /csm-authorization/system:serviceaccount:authorization:storage-service
        - !host /csm-authorization/system:serviceaccount:authorization:proxy-server
        - !host /csm-authorization/system:serviceaccount:authorization:tenant-service
        - !host /csm-authorization/system:serviceaccount:authorization:redis
        - !host /csm-authorization/system:serviceaccount:authorization:sentinel
      privilege: [ read, execute ]
      resource: *variables
EOF

  $CONTAINER_RUNTIME run --rm -it -v $PWD/conjur-csm-authorization:/home/cli docker.io/cyberark/conjur-cli:8 policy load -b root -f /home/cli/secrets.yaml

  if [[ -n "$POWERFLEX_USER" && -n "$POWERFLEX_PASS" ]]; then
    $CONTAINER_RUNTIME run --rm -v $PWD/conjur-csm-authorization:/home/cli docker.io/cyberark/conjur-cli:8 variable set -i secrets/powerflex-username -v $POWERFLEX_USER
    $CONTAINER_RUNTIME run --rm -v $PWD/conjur-csm-authorization:/home/cli docker.io/cyberark/conjur-cli:8 variable set -i secrets/powerflex-password -v $POWERFLEX_PASS
  fi

  if [[ -n "$POWERMAX_USER" && -n "$POWERMAX_PASS" ]]; then
    $CONTAINER_RUNTIME run --rm -v $PWD/conjur-csm-authorization:/home/cli docker.io/cyberark/conjur-cli:8 variable set -i secrets/powermax-username -v $POWERMAX_USER
    $CONTAINER_RUNTIME run --rm -v $PWD/conjur-csm-authorization:/home/cli docker.io/cyberark/conjur-cli:8 variable set -i secrets/powermax-password -v $POWERMAX_PASS
  fi

  if [[ -n "$POWERSCALE_USER" && -n "$POWERSCALE_PASS" ]]; then
    $CONTAINER_RUNTIME run --rm -v $PWD/conjur-csm-authorization:/home/cli docker.io/cyberark/conjur-cli:8 variable set -i secrets/powerscale-username -v $POWERSCALE_USER
    $CONTAINER_RUNTIME run --rm -v $PWD/conjur-csm-authorization:/home/cli docker.io/cyberark/conjur-cli:8 variable set -i secrets/powerscale-password -v $POWERSCALE_PASS
  fi

  if [[ -n "$POWERSTORE_USER" && -n "$POWERSTORE_PASS" ]]; then
    $CONTAINER_RUNTIME run --rm -v $PWD/conjur-csm-authorization:/home/cli docker.io/cyberark/conjur-cli:8 variable set -i secrets/powerstore-username -v $POWERSTORE_USER
    $CONTAINER_RUNTIME run --rm -v $PWD/conjur-csm-authorization:/home/cli docker.io/cyberark/conjur-cli:8 variable set -i secrets/powerstore-password -v $POWERSTORE_PASS
  fi

  if [[ -n "$REDIS_USER" && -n "$REDIS_PASS" ]]; then
    $CONTAINER_RUNTIME run --rm -v $PWD/conjur-csm-authorization:/home/cli docker.io/cyberark/conjur-cli:8 variable set -i secrets/redis-username -v $REDIS_USER
    $CONTAINER_RUNTIME run --rm -v $PWD/conjur-csm-authorization:/home/cli docker.io/cyberark/conjur-cli:8 variable set -i secrets/redis-password -v $REDIS_PASS
  fi

  if [[ -n "$JWT_SIGNING_SECRET" ]]; then
    # JWT_SIGNING_SECRET is often provided as a single-line env var with literal '\n' escapes.
    # Convert it into real newlines before storing in Conjur so it can be mounted as valid YAML.
    printf '%b' "$JWT_SIGNING_SECRET" > conjur-csm-authorization/config-object.yaml
    $CONTAINER_RUNTIME run --rm -v $PWD/conjur-csm-authorization:/home/cli docker.io/cyberark/conjur-cli:8 variable set -i secrets/config-object -v "$(cat conjur-csm-authorization/config-object.yaml)"
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
  secretObjects:
    - secretName: redis-secret
      type: kubernetes.io/basic-auth
      data:
        - objectName: secrets/redis-username
          key: username
        - objectName: secrets/redis-password
          key: password
    - secretName: config-secret
      type: Opaque
      data:
        - objectName: secrets/config-object
          key: config.yaml
  parameters:
    conjur.org/configurationVersion: 0.2.0
    account: dev-cluster
    applianceUrl: 'https://conjur-conjur-oss.default.svc.cluster.local'
    authnId: authn-jwt/kube
    sslCertificate: |
$CONJUR_CERT
EOF
