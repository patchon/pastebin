# Golang Pastebin
Simple and modern self-hosted pastebin built in go.

# Features
* Syntax highlighting with pygments.
* Clean and simple webinterface
* RESTful API
* Small codebase < 1000 lines.
* Kubernetes and OpenShift native

# Installation
## Kubernetes
### Prerequisites
* A working kubernetes cluster (minikube works)
* A working container image registry
* A working docker / podman client
* A working kubectl client

```bash
# Clone and build container image,
$ > git clone https://github.com/patchon/pastebin
$ > cd kubernetes
$ > podman build . -t pastebin:latest

# If you use minkube and it's registry
$ > eval $(minikube docker-env)

# Upload to your registry
$ > podman tag pastebin localhost:5000/pastebin
$ > podman push localhost:5000/pastebin

# Create namespace and apply resources
$ > kubectl create ns pastebin
$ > kubectl config set-context --current --namespace=pastebin
$ > kubectl apply -f pastebin-k8s.yaml

# If you use minkube
$ > minikube service pastebin

# Check logs
$ > kubectl logs -f --selector=app.kubernetes.io/name=pastebin
```
❗ Notes
* If you are using minkube, you can read about how to use it's docker registry [here](https://minikube.sigs.k8s.io/docs/handbook/pushing/)
* The image-reference in the deployment (pastebin-k8s.yaml) is set to **localhost:5000/pastebin:latest**, you probably want to update it to reflect your image registry. However if you are using minkube and it's registry, this should work.



# Old, will fix, someday
#Golang Pastebin
[![Build Status](https://travis-ci.org/ewhal/Pastebin.svg?branch=master)](https://travis-ci.org/ewhal/Pastebin) [![GoDoc](https://godoc.org/github.com/ewhal/Pastebin?status.svg)](https://godoc.org/github.com/ewhal/Pastebin) [![Go Report Card](https://goreportcard.com/badge/github.com/ewhal/Pastebin)](https://goreportcard.com/report/github.com/ewhal/Pastebin) [![MIT
licensed](https://img.shields.io/badge/license-MIT-blue.svg)](https://raw.githubusercontent.com/ewhal/Pastebin/master/LICENSE.md)

Modern self-hosted pastebin service with a restful API.

## Motivation
Many Pastebin services exist but all are more complicated than they need to be.
That is why I decided to write a pastebin service in golang.

![paste](http://i.imgur.com/7BeCKa3.png)

## Getting started
### Prerequisities
* pygmentize
* go
* mariadb

```
pip install pygmentize
sudo yum install -y go mariadb-server mariadb
```

### Installing
* Please note this assumes you have Mariadb and Go already setup.
* go get github.com/ewhal/Pastebin
* make
* mysql -u root -p
* CREATE USER 'paste'@'localhost' IDENTIFIED BY 'password';
* CREATE database paste;
* GRANT ALL PRIVILEGES ON paste . * TO 'paste'@'localhost';
* FLUSH PRIVILEGES;
* quit;
* mysql -u paste -p paste < database.sql
* cp config.example.json config.json
* nano config.json
* Configure port and database details

## License

This project is licensed under the MIT License - see the [LICENSE.md](LICENSE.md) file for details
