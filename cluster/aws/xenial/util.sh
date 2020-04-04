#!/bin/bash

# Copyright 2020 Authors of Arktos.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

SSH_USER=ubuntu

# Detects the AMI to use for ubuntu (considering the region)
#
# Vars set:
#   AWS_IMAGE
function detect-xenial-image () {
  # This is the ubuntu 16.04 image for <region>, amd64, hvm:ebs-ssd
  # See here: http://cloud-images.ubuntu.com/locator/ec2/ for other images
  # This will need to be updated from time to time as amis are deprecated
  if [[ -z "${AWS_IMAGE-}" ]]; then
    case "${AWS_REGION}" in
      ap-south-1)
        AWS_IMAGE=ami-54d2a63b
        ;;

      ap-northeast-1)
        AWS_IMAGE=ami-003c6ed5c5176db19
        ;;

      ap-northeast-2)
        AWS_IMAGE=ami-79815217
        ;;

      ap-southeast-1)
        AWS_IMAGE=ami-0b21b3ea2cb8660a5
        ;;

      eu-central-1)
        AWS_IMAGE=ami-0257508f40836e6cf
        ;;

      eu-west-1)
        AWS_IMAGE=ami-01793b684af7a3e2c
        ;;

      us-east-1)
        AWS_IMAGE=ami-04ac550b78324f651
        ;;

      us-east-2)
        AWS_IMAGE=ami-0009e532719fe9bff
        ;;

      us-west-1)
        AWS_IMAGE=ami-0798ac7e2b0fb9e75
        ;;

      cn-north-1)
        AWS_IMAGE=ami-00f390a7c449fa29e
        ;;

      us-west-2)
        AWS_IMAGE=ami-02e30ba14d8ffa6e6
        ;;

      *)
        echo "Please specify AWS_IMAGE directly (region ${AWS_REGION} not recognized)"
        exit 1
    esac
  fi
}

