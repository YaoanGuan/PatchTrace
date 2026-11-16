FROM docker.m.daocloud.io/library/ubuntu:22.04
ARG DEBIAN_FRONTEND=non-interactive
ARG TARGETARCH
ARG CFR_VERSION=0.152
ENV TZ=Asia/ShangHai
COPY data /data

RUN sed -i "s/ports.ubuntu.com/mirrors.ustc.edu.cn/g" /etc/apt/sources.list && \
apt update &&\
apt install -y php gcc python3 dirsearch nmap sqlmap python3-pip netcat openjdk-8-jdk golang unzip zip vim curl wget git mysql-client ssh ftp sshpass nodejs proxychains4 &&\
apt install -y libx11-dev libxtst-dev libpng-dev python3-tk
RUN pip3 install \
flask \
requests \
pyyaml \
impacket \
ddddocr \
cryptography \
-i https://mirrors.ustc.edu.cn/pypi/simple

WORKDIR /tmp
RUN ln -s /data/tools/phpgcc/phpgcc /bin/phpgcc && \
git clone https://github.com/eeeeeeeeee-code/POC.git /data/POCS && chmod +x /data/tools/* && \
curl https://raw.githubusercontent.com/rapid7/metasploit-omnibus/master/config/templates/metasploit-framework-wrappers/msfupdate.erb | bash && \
mv /data/captcha /bin/captcha_get && chmod +x /bin/captcha_get
RUN mkdir -p /opt/aixvuln-tools && \
    curl -fsSL "https://www.benf.org/other/cfr/cfr-${CFR_VERSION}.jar" -o /opt/aixvuln-tools/cfr.jar
CMD ["tail", "-f", "/dev/null"]
