## One stop set up developer environment

To help easily building up arktos dev environment, we build a script which will install typical arktos dev needed components in one stop.

The script was test on empty vm AWS EC2 and GCP, for Ubuntu 16.04 x86 image.

The script will run about 15 minutes. After it restarts the vm, arktos dev environment is ready.  

```bash
$ wget https://raw.githubusercontent.com/futurewei-cloud/arktos/master/hack/arktos-dev-env-setup.sh && chmod 755 arktos-dev-env-setup.sh && ./arktos-dev-env-setup.sh
```

## Set up developer environment

Alternatively, following steps are to install necessary arktos dev needed components one by one. 

Note: tested on Ubuntu 16.04 x86 image 

### Clone repo
```
$ mkdir -p go/src/
$ cd go/src/
$ git clone https://github.com/futurewei-cloud/arktos
```

### Install Golang
```
$ sudo apt-get update
# $ sudo apt-get -y upgrade // optional
$ cd /tmp
$ wget https://dl.google.com/go/go1.12.9.linux-amd64.tar.gz
$ tar -xvf go1.12.9.linux-amd64.tar.gz
$ sudo mv go /usr/local
$ rm go1.12.9.linux-amd64.tar.gz
```
Add the following lines to ~/.profile
```
GOROOT=/usr/local/go
GOPATH=$HOME/go
PATH=$GOPATH/bin:$GOROOT/bin:$PATH
```
Update the current shell session
```
$ source ~/.profile
```

### Install gcc and make. There might be an issue to build images. It can be fixed by running "git tag -a v2.7.4"
```
$ sudo apt install build-essential
```

### Install Docker 
```
# sudo apt-get update -y -q

# sudo apt-get install \
    apt-transport-https \
    ca-certificates \
    curl \
    gnupg-agent \
    software-properties-common -y -q

$ curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
# sudo apt-key fingerprint 0EBFCD88

# sudo add-apt-repository \
   "deb [arch=amd64] https://download.docker.com/linux/ubuntu \
   $(lsb_release -cs) \
   stable"

# sudo apt-get update -y -q
# sudo apt-get install docker-ce docker-ce-cli containerd.io -y -q
# sudo gpasswd -a $USER docker
```

### Install crictl 
```
$ cd /tmp
$ wget https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.17.0/crictl-v1.17.0-linux-amd64.tar.gz
# sudo tar zxvf crictl-v1.17.0-linux-amd64.tar.gz -C /usr/local/bin 
$ rm -f crictl-v1.17.0-linux-amd64.tar.gz 

$ touch /tmp/crictl.yaml
$ echo runtime-endpoint: unix:///run/containerd/containerd.sock >> /tmp/crictl.yaml
$ echo image-endpoint: unix:///run/containerd/containerd.sock >> /tmp/crictl.yaml
$ echo timeout: 10 >> /tmp/crictl.yaml
$ echo debug: true >> /tmp/crictl.yaml
# sudo mv /tmp/crictl.yaml /etc/crictl.yaml

# mkdir -p /etc/containerd
# sudo rm -rf /etc/containerd/config.toml
# sudo containerd config default > /tmp/config.toml
# sudo mv /tmp/config.toml /etc/containerd/config.toml
# sudo systemctl restart containerd
```
