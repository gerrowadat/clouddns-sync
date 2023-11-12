FROM golang:1.21
RUN go install github.com/gerrowadat/clouddns-sync@0.0.6

COPY entrypoint.sh /
RUN chmod +x /entrypoint.sh

USER root

# Interval between runs.
ENV GCLOUD_DNS_INTERVAL_SECS=86400

# gcloud specifiers
ENV GCLOUD_VERB "dynrecord"
ENV GCLOUD_DNS_ZONE ""
ENV GCLOUD_DYN_RECORD_NAME ""

# nomad specifiers
ENV NOMAD_SERVER_URI ""
ENV NOMAD_TOKEN_FILE ""

# json credentials file location
ENV JSON_KEYFILE ""

# zonefile location
ENV ZONEFILENAME ""

ENTRYPOINT ["/entrypoint.sh"]
CMD ["/entrypoint.sh"]

