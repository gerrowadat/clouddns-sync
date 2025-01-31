#!/usr/bin/bash

set -e

while true
do
  echo "Doing $GCLOUD_VERB for zone $GCLOUD_DNS_ZONE"

  case $GCLOUD_VERB in
    nomad_sync)
      clouddns-sync \
        --cloud-dns-zone=$GCLOUD_DNS_ZONE \
        --json-keyfile=$JSON_KEYFILE \
        --nomad-server-uri=$NOMAD_SERVER_URI \
        --nomad-token-file=$NOMAD_TOKEN_FILE \
        --http-port=$HTTP_PORT \
        $GCLOUD_VERB
              ;;
    getzonefile | putzonefile)
      clouddns-sync \
        --cloud-dns-zone=$GCLOUD_DNS_ZONE \
        --json-keyfile=$JSON_KEYFILE \
        -zonefilename=$ZONEFILENAME \
        $GCLOUD_VERB
              ;;
    dynrecord)
      clouddns-sync \
        --cloud-dns-zone=$GCLOUD_DNS_ZONE \
        --cloud-dns-dyn-record-name=$GCLOUD_DYN_RECORD_NAME \
        --json-keyfile=$JSON_KEYFILE \
        $GCLOUD_VERB
              ;;
  esac

  echo "Sleeping for $GCLOUD_DNS_INTERVAL_SECS seconds..."
  sleep $GCLOUD_DNS_INTERVAL_SECS
done
