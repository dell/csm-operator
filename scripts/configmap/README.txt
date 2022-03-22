goal : remove ymls from csm-operator container and src repo


move csm-operator/operatorconfig ymls to new repo csm-operator-config

	that is csm-operator-config repo contains ymls and scripts folder for different drivers/modules versions

	samples folder contains usage for new versions of driver/modules


new version deploy process 

 csm-operator-config  scripts/configmaps/samples folder is updated by engg team for new versions of any driver/module

	neeed code to automate conversion of driver/modules yamls to operator-format yamls

	detect changes during driver release cycle and update operator yamls

customer clones this config repo and runs either

	script create_cm.sh to create new configmap for new versions
	or 
	runs runs script upload_cm.sh upload tgz file to local repo

	todo:
	during deploy  see cm.go code to download and create configmaps -we add this to reconcile 

	run:
	edit cm.go to enter user / pswd to repo
	go run cm.go

test : 
	install version x
	upgrade to version y
	dont support downgrade if desired is less than installed version

	instead delete y and install x is ok --we cannot prevent this
