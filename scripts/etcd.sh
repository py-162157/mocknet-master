#!/usr/bin/env bash

# this script download etcd pre-built file in /tmp directory,
# with disk temporary occupation of about 100M, make sure enough space.

ETCD_VER=v3.5.0

# choose either URL
GOOGLE_URL=https://storage.googleapis.com/etcd
GITHUB_URL=https://github.com/etcd-io/etcd/releases/download
DOWNLOAD_URL=${GITHUB_URL}

if [ ! -f "/usr/local/bin/etcd" ]; then
    rm -f /tmp/etcd-${ETCD_VER}-linux-amd64.tar.gz
    rm -rf /tmp/etcd-download-test && mkdir -p /tmp/etcd-download-test

    curl -L ${DOWNLOAD_URL}/${ETCD_VER}/etcd-${ETCD_VER}-linux-amd64.tar.gz -o /tmp/etcd-${ETCD_VER}-linux-amd64.tar.gz
    tar xzvf /tmp/etcd-${ETCD_VER}-linux-amd64.tar.gz -C /tmp/etcd-download-test --strip-components=1
    rm -f /tmp/etcd-${ETCD_VER}-linux-amd64.tar.gz

    mv /tmp/etcd-download-test/etcd /usr/local/bin/
    mv /tmp/etcd-download-test/etcdctl /usr/local/bin/
    mv /tmp/etcd-download-test/etcdutl /usr/local/bin/

    etcd --version
    etcdctl version
    etcdutl version

    echo "successfully setup etcd binary!"
else 
    echo "the etcd binary already exist, no need to setup"
fi

etcd --name=contiv-etcd --data-dir=/var/etcd/mocknet-data --advertise-client-urls=http://0.0.0.0:22379 --listen-client-urls=http://0.0.0.0:22379 --listen-peer-urls=http://0.0.0.0:22380

NODE_IP=`ifconfig -a|grep inet|grep -v 127.0.0.1|grep -v inet6|grep 192|awk '{print $2}'|tr -d "addr:"​`

if [ ! -d "/opt/etcd" ]; then
    mkdir /opt/etcd
    cd /opt/etcd
    touch etcd.conf
    chmod 777 etcd.conf
	echo "insecure-transport: true" >> etcd.conf
	echo "dial-timeout: 10000000000" >> etcd.conf
	echo "allow-delayed-start: true" >> etcd.conf
	echo "endpoints:" >> etcd.conf
	echo "  - "${NODE_IP}:22380"" >> etcd.conf
fi