## Set up developer environment (updated on 2021-07-22)

Note: Tested on AWS EC2 Ubuntu 16.04/18.04/20.04 x86 image.

### Clone repo
```
$ mkdir -p go/src/
$ cd go/src/
$ git clone https://github.com/centaurusinfra/arktos
```

### Install needed packages (docker, make, gcc, jq and golang)
```
$ ./hack/setup-dev-node.sh
```

### Update your account's profile
Add the following lines into the profile
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
