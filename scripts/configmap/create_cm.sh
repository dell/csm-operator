k delete cm powerscale-driver-v2.2.0,driver-sidecars,authorization-v2.2.0,authorization-common -n dell-csm-operator
kubectl create cm powerscale-driver-v2.2.0 --from-file=powerscale/v2.2.0/ -n dell-csm-operator
kubectl get cm powerscale-driver-v2.2.0 -n dell-csm-operator

kubectl create cm driver-sidecars --from-file=sidecars/ -n dell-csm-operator
k get cm driver-sidecars -n dell-csm-operator

kubectl create cm authorization-v1.2.0 --from-file=authorization/v1.2.0 -n dell-csm-operator
k get cm authorization-v2.2.0 -n dell-csm-operator

kubectl create cm authorization-common --from-file=auth-common/ -n dell-csm-operator
k get cm authorization-common -n dell-csm-operator
