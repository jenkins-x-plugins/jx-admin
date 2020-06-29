FROM centos:7

RUN yum install -y git

ENTRYPOINT ["jx-alpha-remote"]

COPY ./build/linux/jx-alpha-remote /usr/bin/jx-alpha-remote