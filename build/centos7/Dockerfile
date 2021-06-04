From centos:7
ENV container docker
RUN yum install git wget -y
RUN git clone https://github.com/OSU-SOC/nagini.git
RUN wget https://dl.google.com/go/go1.13.5.linux-amd64.tar.gz
RUN tar -C /usr/local -xzf go1.13.5.linux-amd64.tar.gz
RUN cd /nagini && /usr/local/go/bin/go install .