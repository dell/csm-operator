# Download config tgz -- you will be required to put your personal access token in here as long as the csm-operator-config repo is private
wget --header 'Authorization: token <insert token>' https://raw.githubusercontent.com/dell/csm-operator-config/fix-module-common/powerscale/powerscale-v2.2.0/downloads/pscale-v220-cfg.tgz

# untar the config files, this will produce a folder called csmconfig
tar -xzvf pscale-v220-cfg.tgz

# remove the unnecessary tar file
rm -f pscale-v220-cfg.tgz

kubectl delete cm powerscale-driver-v2.2.0 driver-sidecars authorization-module-v1.2.0 authorization-common -n dell-csm-operator
kubectl create cm powerscale-driver-v2.2.0 --from-file=csmconfig/driver/ -n dell-csm-operator
kubectl get cm powerscale-driver-v2.2.0 -n dell-csm-operator

kubectl create cm driver-sidecars --from-file=csmconfig/sidecars/ -n dell-csm-operator
kubectl get cm driver-sidecars -n dell-csm-operator

kubectl create cm authorization-v1.2.0 --from-file=csmconfig/authorization-module-v1.2.0/ -n dell-csm-operator
kubectl get cm authorization-module-v1.2.0 -n dell-csm-operator

kubectl create cm module-common-v2.2.0 --from-file=csmconfig/module-common-v2.2.0/ -n dell-csm-operator
kubectl get cm module-common-v2.2.0 -n dell-csm-operator
