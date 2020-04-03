## Set up developer environment

Note: tested on AWS EC2 Ubuntu 16.04 x86 image 

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

### Install Containerd 
```
$ wget https://storage.googleapis.com/cri-containerd-release/cri-containerd-1.1.8.linux-amd64.tar.gz
$ sudo tar --no-overwrite-dir -C / -xzf cri-containerd-1.1.8.linux-amd64.tar.gz
$ rm cri-containerd-1.1.8.linux-amd64.tar.gz
$ sudo systemctl start containerd
```
