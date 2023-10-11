BACKUP_NAME_EXT=$(date +%s)
BACKUP_NAME=b$BACKUP_NAME_EXT
RESTORE_NAME=r$BACKUP_NAME_EXT

# no need to check for pod success since e2e already does that


# make sure dellctl is installed
./dellctl 
RET=$?
if [ "${RET}" == "127" ]; then
  echo "dellctl is not installed, attempting install"
  wget https://github.com/dell/csm/releases/download/v1.7.1/dellctl  
  chmod +x dellctl 
fi

# make sure env variables are present
ExitCode=0
if [ "${VOL_NS}" == "" ]; then
   echo "env variable VOL_NS is not set"
   ExitCode=1
fi
if [ "${RES_NS}" == "" ]; then
   echo "env variable RES_NS is not set"
   ExitCode=1
fi
if [ "${AM_NS}" == "" ]; then
   echo "env variable AM_NS is not set"
   ExitCode=1
fi

if [ "${ExitCode}" == "1" ]; then
  echo "Some env variables are missing. Set in env-e2e-test.sh and run source env-e2e-test.sh"
  exit 1
fi


# attempt backup, check if successful
./dellctl backup create $BACKUP_NAME --include-namespaces $VOL_NS -n $AM_NS


# check return code from backup command
RET=$?
if [ "${RET}" != "0" ]; then
  echo "backup failed with return code $RET"
  exit $RET
fi


# give the backup resource 5 minutes to succeed
BACKUP_WAIT_TIME=$((SECONDS+300))
sleep 5
while [ $SECONDS -lt $BACKUP_WAIT_TIME ]; do
  NUM_GOOD_BACKUPS=$(./dellctl backup get $BACKUP_NAME -n $AM_NS | grep "Completed" | wc -l)
  if [ "${NUM_GOOD_BACKUPS}" == "1" ]; then
    echo "backup successful"
    break
  fi
  echo "waiting for backup to complete"
  sleep 60
done


if [ "${NUM_GOOD_BACKUPS}" != "1" ]; then
  echo -e "backup not completed -- backup current status:"
  ./dellctl backup get $BACKUP_NAME -n $AM_NS
  exit 1
fi


kubectl delete ns $RES_NS

# attempt restore, check if successful
./dellctl restore create $RESTORE_NAME --from-backup $BACKUP_NAME -n $AM_NS --namespace-mappings $VOL_NS:$RES_NS
RET=$?
if [ "${RET}" != "0" ]; then
  echo "restore failed with return code $RET"
  exit $RET
fi


# give the backup resource 5 minutes to succeed
RESTORE_WAIT_TIME=$((SECONDS+300))
sleep 5
while [ $SECONDS -lt $RESTORE_WAIT_TIME ]; do
  NUM_GOOD_RESTORE=$(./dellctl restore get $RESTORE_NAME -n $AM_NS | grep "Completed" | wc -l)
  if [ "${NUM_GOOD_RESTORE}" == "1" ]; then
    echo "restore successful"
    kubectl get all -n $RES_NS
    break
  fi
  echo "waiting for restore to complete"
  sleep 60
done


if [ "${NUM_GOOD_RESTORE}" != "1" ]; then
  echo -e "restore not completed -- restore current status:"
  ./dellctl restore get $RESTORE_NAME -n $AM_NS
  exit 1
fi



# success -- delete test restore and backup

./dellctl restore delete $RESTORE_NAME -n $AM_NS --confirm 

./dellctl backup delete $BACKUP_NAME -n $AM_NS --confirm


exit 0
