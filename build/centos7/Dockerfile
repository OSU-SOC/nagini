From centos:7
ENV container docker
RUN yum install git wget -y
RUN wget https://golang.org/dl/go1.16.5.linux-amd64.tar.gz
RUN tar -C /usr/local -xzf go1.16.5.linux-amd64.tar.gz
RUN /usr/local/go/bin/go get github.com/OSU-SOC/nagini