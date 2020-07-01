FROM centos:7

RUN yum install -y git

ENTRYPOINT ["jx-admin"]

COPY ./build/linux/jx-admin /usr/bin/jx-admin