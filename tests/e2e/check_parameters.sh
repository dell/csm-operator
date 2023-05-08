# !/bin/bash

# get controller describe
controllerpod=`kubectl get pods -n $2 | grep -m 1 controller | awk '{print $1}'`
kubectl describe pod $controllerpod -n $2 > controller-describe

# get node describe
nodepod=`kubectl get pods -n $2 | grep -m 1 node | awk '{print $1}'`
kubectl describe pod $nodepod -n $2 > node-describe

# get csm describe
kubectl describe csm $3 -n $2 > csm-describe

{
  read
  while IFS=, read -r paramName grepOptions paramValue k8sResource
  do
    cat $k8sResource-describe | grep "$paramName" | grep -q $grepOptions "$paramValue"
    RET=$?
    if [ "$RET" == "0" ]; then
      echo "$paramName with value $paramValue found in $k8sResource"
    else
      echo "$paramName with value $paramValue NOT found in $k8sResource"
    fi
  done 
}< $1
