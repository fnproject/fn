# ssh into the box on staging that the prototype is running on

set -ex

ssh -i gateway.pem rancher@ec2-52-205-252-82.compute-1.amazonaws.com
