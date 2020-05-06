#!/bin/bash

####################

echo Setup: Enable password login

sudo sed -i 's/PasswordAuthentication no/PasswordAuthentication yes/g' /etc/ssh/sshd_config
### Set password: sudo passwd ubuntu
sudo service sshd restart

####################

echo Setup: Install remote desktop

sudo apt update
sudo apt install -y ubuntu-desktop xrdp

sudo service xrdp restart
sudo apt install -y xfce4 xfce4-goodies
echo xfce4-session >~/.xsession

####################

echo Setup: Install go \(currently limited to version 1.12.12\)

sudo apt-get update -y -q

cd /tmp
wget https://dl.google.com/go/go1.12.12.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.12.12.linux-amd64.tar.gz
rm -rf go1.12.12.linux-amd64.tar.gz

####################

echo Setup: Install bazel

sudo apt install g++ unzip zip
sudo apt-get install openjdk-8-jdk -y -q
cd /tmp
wget https://github.com/bazelbuild/bazel/releases/download/0.26.1/bazel-0.26.1-installer-linux-x86_64.sh
chmod +x bazel-0.26.1-installer-linux-x86_64.sh
./bazel-0.26.1-installer-linux-x86_64.sh --user

####################

echo Setup: Install goland

cd /tmp
wget https://download.jetbrains.com/go/goland-2019.3.4.tar.gz
tar -xzf goland-2019.3.4.tar.gz
mv GoLand-2019.3.4 ~/GoLand-2019.3.4

echo fs.inotify.max_user_watches=524288 > ./max_user_watches.conf
sudo mv ./max_user_watches.conf /etc/sysctl.d/
sudo sysctl -p --system

####################

echo Setup: Enlist arktos

cd ~
git clone https://github.com/futurewei-cloud/arktos.git ~/go/src/k8s.io/arktos
cd ~/go/src/k8s.io
ln -s ./arktos kubernetes

####################

echo Setup: Install etcd

cd ~/go/src/k8s.io/arktos/
git tag v1.15.0
./hack/install-etcd.sh

####################

echo Setup: Install Docker

sudo apt-get update -y -q

sudo apt-get install \
    apt-transport-https \
    ca-certificates \
    curl \
    gnupg-agent \
    software-properties-common -y -q

curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
sudo apt-key fingerprint 0EBFCD88

sudo add-apt-repository \
   "deb [arch=amd64] https://download.docker.com/linux/ubuntu \
   $(lsb_release -cs) \
   stable"

sudo apt-get update -y -q
sudo apt-get install docker-ce docker-ce-cli containerd.io -y -q
sudo gpasswd -a $USER docker


####################

echo Setup: Install crictl

cd /tmp
wget https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.17.0/crictl-v1.17.0-linux-amd64.tar.gz
sudo tar zxvf crictl-v1.17.0-linux-amd64.tar.gz -C /usr/local/bin
rm -f crictl-v1.17.0-linux-amd64.tar.gz

touch /tmp/crictl.yaml
echo runtime-endpoint: unix:///run/containerd/containerd.sock >> /tmp/crictl.yaml
echo image-endpoint: unix:///run/containerd/containerd.sock >> /tmp/crictl.yaml
echo timeout: 10 >> /tmp/crictl.yaml
echo debug: true >> /tmp/crictl.yaml
sudo mv /tmp/crictl.yaml /etc/crictl.yaml

mkdir -p /etc/containerd
sudo rm -rf /etc/containerd/config.toml
sudo containerd config default > /tmp/config.toml
sudo mv /tmp/config.toml /etc/containerd/config.toml
sudo systemctl restart containerd

####################

echo Setup: Install miscellaneous

sudo apt install awscli -y -q
sudo apt install python-pip -y -q
sudo apt install jq -y -q

####################

echo Setup: Setup profile

echo PATH=\"\$HOME/go/src/k8s.io/arktos/third_party/etcd:/usr/local/go/bin:\$HOME/go/bin:\$HOME/go/src/k8s.io/arktos/_output/bin:\$HOME/go/src/k8s.io/arktos/_output/dockerized/bin/linux/amd64:\$PATH\" >> ~/.profile
echo GOPATH=\"\$HOME/go\" >> ~/.profile
echo GOROOT=\"/usr/local/go\" >> ~/.profile
echo >> ~/.profile
echo alias arktos=\"cd \$HOME/go/src/k8s.io/arktos\" >> ~/.profile
echo alias up=\"\$HOME/go/src/k8s.io/arktos/hack/arktos-up.sh\" >> ~/.profile
echo alias status=\"git status\" >> ~/.profile
echo cd \$HOME/go/src/k8s.io/arktos >> ~/.profile

source "$HOME/.profile"

####################

echo Setup: Install kubetest

cd ~/go/src/k8s.io
git clone https://github.com/kubernetes/test-infra.git
cd ~/go/src/k8s.io/test-infra/
GO111MODULE=on go install ./kubetest
GO111MODULE=on go mod vendor

####################

echo Setup: Install Kind

cd ~/go/src/
GO111MODULE="on" go get sigs.k8s.io/kind@v0.7.0

####################

echo Setup: Machine setup completed!

sudo reboot