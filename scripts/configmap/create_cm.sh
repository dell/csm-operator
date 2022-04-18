#!/bin/bash

CONFIGMAPPREFIX="pscale-v220"
NS="dell-csm-operator"

echo "************ Downloading configmap yamls ************"

# Download config tgz -- you will be required to put your personal access token in here as long as the csm-operator-config repo is private
read -s -p 'Please enter GitHub token: ' github_token
wget --header "Authorization: token ${github_token}" https://raw.githubusercontent.com/dell/csm-operator-config/fix-module-common/powerscale/powerscale-v2.2.0/downloads/pscale-v220-cfg.tgz
if [ "$?" != "0" ]; then
        echo "wget of config files failed, exiting"
	exit 1
fi

echo "************ Untar config files ************"
# untar the config files, this will produce a folder called csmconfig
tar -xzvf pscale-v220-cfg.tgz

# remove the unnecessary tar file
rm -f pscale-v220-cfg.tgz

echo "************ Create config maps ************"
cd csmconfig
printf '%s\n' */ | while read cfgmap
do
	echo "************** Creating configmap $CONFIGMAPPREFIX-${cfgmap%/} *****************"

	# check for existing configmap
	kubectl get configmap -n $NS | grep $CONFIGMAPPREFIX-${cfgmap%/}

	# delete if user wants to replace; otherwise, skip this iteration
	if [ "$?" == "0" ]; then
		echo "configmap $CONFIGMAPPREFIX-${cfgmap%/} already exists. Would you like to replace it? yes/no:"
		read response < /dev/tty
		if [ "$response" == "yes" ]; then
			kubectl delete configmap $CONFIGMAPPREFIX-${cfgmap%/} -n $NS
		else
			echo "Not replacing pre-existing configmap $CONFIGMAPPREFIX-${cfgmap%/}"
			continue
		fi
	fi

	# create new configmap
	kubectl create cm $CONFIGMAPPREFIX-${cfgmap%/} --from-file=$cfgmap -n $NS

	# check the configmap exists
	kubectl get configmap -n $NS | grep $CONFIGMAPPREFIX-${cfgmap%/}
	if [ "$?" == "0" ]; then
		echo "configmap $CONFIGMAPPREFIX-${cfgmap%/} successfully created"
	else
		echo "configmap $CONFIGMAPPREFIX-${cfgmap%/} not successfully created"
	fi
done
cd ..

