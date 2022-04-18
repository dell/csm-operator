
export DRIVER="powerscale"
kubectl delete cm ${DRIVER}-driver-v2.2.0,driver-sidecars,authorization-v2.2.0,authorization-common -n dell-csm-operator

kubectl create cm ${DRIVER}-driver-v2.2.0 --from-file=powerscale/v2.2.0/ -n dell-csm-operator
kubectl get cm ${DRIVER}-driver-v2.2.0 -n dell-csm-operator

kubectl create cm driver-sidecars --from-file=sidecars/ -n dell-csm-operator
kubectl get cm driver-sidecars -n dell-csm-operator

kubectl create cm authorization-v1.2.0 --from-file=authorization/v1.2.0 -n dell-csm-operator
kubectl get cm authorization-v2.2.0 -n dell-csm-operator

kubectl create cm authorization-common --from-file=auth-common/ -n dell-csm-operator
kubectl get cm authorization-common -n dell-csm-operator


export DRIVER="powerflex"

kubectl delete cm ${DRIVER}-driver-v2.2.0 -n dell-csm-operator

kubectl create cm ${DRIVER}-driver-v2.2.0 --from-file=${DRIVER}/v2.2.0/ -n dell-csm-operator
kubectl get cm ${DRIVER}-driver-v2.2.0 -n dell-csm-operator



# powerflex values file enable all sidecars : podmon , health , vgs sidecars and certCount=1
# todo : handle init container
# todo : handle variables

# generate helm dry-run from pflex helm files
# helm install --dry-run --values csi-vxflexos/values.yaml --namespace vxflexos vxflexos ./csi-vxflexos/ > h.yaml

# sed -i '/^[[:space:]]*$/d' h.yaml

# save to 4 different yamls : controller, node , csidriver , driver-config-params

# edit these customizations

sed -i 's/name: vxflexos-controller/name: <DriverDefaultReleaseName>-controller/' controller.yaml
sed -i 's/namespace: vxflexos/namespace: <DriverDefaultReleaseNamespace>/' controller.yaml
sed -i 's/secretName: vxflexos-config/secretName: <DriverDefaultReleaseName>-config/' controller.yaml
sed -i 's/name: vxflexos-config-params/name: <DriverDefaultReleaseName>-config-params/' controller.yaml
sed -i 's/serviceAccountName: vxflexos-controller/serviceAccountName: <DriverDefaultReleaseName>-controller/' controller.yaml
 
sed -i 's/name: vxflexos-certs/name: <DriverDefaultReleaseName>-certs/' controller.yaml
sed -i 's/- vxflexos-controller/- <DriverDefaultReleaseName>-controller/' controller.yaml

sed -i 's/name: vxflexos-node/name: <DriverDefaultReleaseName>-node/' node.yaml
sed -i 's/namespace: vxflexos/namespace: <DriverDefaultReleaseNamespace>/' node.yaml
sed -i 's/app: vxflexos-node/app: <DriverDefaultReleaseName>-node/' node.yaml

sed -i 's/serviceAccount: vxflexos-node/serviceAccountName: <DriverDefaultReleaseName>-node/' node.yaml
sed -i 's/serviceAccountName: vxflexos-node/serviceAccountName: <DriverDefaultReleaseName>-node/' node.yaml

sed -i 's/secretName: vxflexos-config/secretName: <DriverDefaultReleaseName>-config/' node.yaml
sed -i 's/name: vxflexos-config-params/name: <DriverDefaultReleaseName>-config-params/' node.yaml

sed -i 's/name: vxflexos-certs/name: <DriverDefaultReleaseName>-certs/' node.yaml

sed -i 's/name: vxflexos-config-params/name: <DriverDefaultReleaseName>-config-params/' driver-config-params.yaml
sed -i 's/namespace: vxflexos/namespace: <DriverDefaultReleaseNamespace>/' driver-config-params.yaml

# install yamllint and run , fix indent/space issues
yamllint *

