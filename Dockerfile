FROM centos:7

COPY ./test /usr/local/bin/test

ENTRYPOINT [ "/usr/local/bin/test"]