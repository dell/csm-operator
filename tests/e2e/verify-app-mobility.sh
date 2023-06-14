BACKUP_NAME_EXT=$(date +%s)
BACKUP_NAME=b$BACKUP_NAME_EXT
RESTORE_NAME=r$BACKUP_NAME_EXT
VOL_NS=helmtest-vxflexos
RES_NS=res-vxflexos


# no need to check for pod success since e2e already does that


# attempt backup, check if successful
./dellctl backup create $BACKUP_NAME --include-namespaces $VOL_NS


# check return code from backup command
RET=$?
if [ "${RET}" != "0" ]; then
  echo "backup failed with return code $RET"
  exit $RET
fi


# give the backup resource 5 minutes to succeed
BACKUP_WAIT_TIME=$((SECONDS+30))
sleep 5
while [ $SECONDS -lt $BACKUP_WAIT_TIME ]; do
  NUM_GOOD_BACKUPS=$(./dellctl backup get $BACKUP_NAME | grep "Completed" | wc -l)
  if [ "${NUM_GOOD_BACKUPS}" == "1" ]; then
    echo "backup successful"
    break
  fi
  echo "waiting for backup to complete"
  sleep 5
done


if [ "${NUM_GOOD_BACKUPS}" != "1" ]; then
  echo -e "backup not completed -- backup current status:"
  ./dellctl backup get $BACKUP_NAME
  exit 1
fi


# attempt restore, check if successful
./dellctl restore create $RESTORE_NAME --from-backup $BACKUP_NAME --namespace-mappings $VOL_NS:$RES_NS
RET=$?
if [ "${RET}" != "0" ]; then
  echo "restore failed with return code $RET"
  exit $RET
fi


# give the backup resource 5 minutes to succeed
RESTORE_WAIT_TIME=$((SECONDS+30))
sleep 5
while [ $SECONDS -lt $RESTORE_WAIT_TIME ]; do
  NUM_GOOD_RESTORE=$(./dellctl restore get $RESTORE_NAME | grep "Completed" | wc -l)
  if [ "${NUM_GOOD_RESTORE}" == "1" ]; then
    echo "restore successful"
    break
  fi
  echo "waiting for restore to complete"
  sleep 5
done


if [ "${NUM_GOOD_RESTORE}" != "1" ]; then
  echo -e "backup not completed -- backup current status:"
  ./dellctl backup get $RESTORE_NAME
  exit 1
fi


# success -- delete test backup
kubectl delete backups.mobility.storage.dell.com/$BACKUP_NAME
kubectl patch backups.mobility.storage.dell.com/$BACKUP_NAME \
    --type json \
    --patch='[ { "op": "remove", "path": "/metadata/finalizers" } ]'


exit 0
