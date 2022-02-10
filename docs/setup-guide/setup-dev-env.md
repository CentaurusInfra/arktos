## Set up developer environment (updated on 2021-11-16)

Note: Tested on AWS EC2 Ubuntu 16.04/18.04/20.04 x86 image.

### Clone repo
```
$ mkdir -p go/src/
$ cd go/src/
$ git clone https://github.com/centaurusinfra/arktos
```

### Install needed packages (docker, make, gcc, jq and golang)
```
$ cd arktos
$ ./hack/setup-dev-node.sh
```

### Update your account's profile
Add the following lines into the profile ~/.profile
```
GOROOT=/usr/local/go
GOPATH=$HOME/go
PATH=$GOPATH/bin:$GOROOT/bin:$PATH
```
Update the current shell session
```
$ source ~/.profile
$ echo $PATH
```

### Ensure the permisson of others for file /var/run/docker.sock should be readable and writable
Normally if the machine is rebooted, the permission of this file is changed to default permission.
Please run the command to add the permission for 'others' using sudo
```bash
sudo chmod o+rw /var/run/docker.sock; sudo ls -al /var/run/docker.sock
```
