FROM gcr.io/jenkinsxio/jx-cli-base:0.0.10

ENTRYPOINT ["jx-admin"]

COPY ./build/linux/jx-admin /usr/bin/jx-admin