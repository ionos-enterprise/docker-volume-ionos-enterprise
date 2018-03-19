FROM alpine

RUN apk update && apk add sshfs

RUN mkdir -p /run/docker/plugins /mnt/state /mnt/volumes

COPY docker-volume-profitbricks docker-volume-profitbricks

CMD ["docker-volume-profitbricks"]
