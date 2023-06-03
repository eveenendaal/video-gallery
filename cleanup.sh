#!/bin/bash

PROJECT="veenendaal-base"
REGION="europe-west4"

for LINE in $(gcloud run revisions list --region="$REGION" --filter="status.conditions[1].status != True AND metadata.ownerReferences.name = 'videogallery'" --format="value(metadata.name)"); do
  gcloud run revisions delete --region=$REGION --quiet --async "$LINE" || true
done

for LINE in $(gcloud artifacts docker images list "$REGION-docker.pkg.dev/$PROJECT/docker/videogallery" --include-tags --filter="NOT tags ~ 'latest' AND createTime<-P1W" --format="value[separator='@']( package,version)"); do
  gcloud artifacts docker images delete --quiet --async "$LINE" || true
done
