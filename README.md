# Docker Volume Plugin

Version: **docker-volume-profitbricks v1.0.0**

## Table of Contents

* [Description](#description)
* [Getting Started](#getting-started)
* [Installation](#installation)
    * [Download](#download)
    * [Build](#build)
    * [Application usage](#application-usage)
    * [Install](#install)
    * [System integration](#system-integration)
* [Usage](#usage)
* [Contributing](#contributing)

## Description

This is a Docker volume plugin which assists in the creation and use of ProfitBricks storage volumes by Docker containers.

## Getting Started

The ProfitBricks volume plugin for Docker has a few requirements:

* Your ProfitBricks account username and password.
* The UUID of an existing ProfitBricks virtual data center (VDC).
* A running ProfitBricks server.
* [Docker](https://www.docker.com/) (CE or EE) >= 17.12
* [Go](https://golang.org/) is required if you want to build the plugin.

Before you begin you will need to have [signed-up](https://www.profitbricks.com/signup) for a ProfitBricks account. The credentials you establish during sign-up will be used by the Docker volume plugin to authenticate against the ProfitBricks Cloud API.

A ProfitBricks virtual data center and server have to be created, configured, and the server must be running. Please review the official [ProfitBricks documentation](https://www.profitbricks.com/help/The_First_Virtual_Data_Center) for more information.

The Docker offical [documentation](https://docs.docker.com/engine/installation/) provides details on installing Docker. For best results, we suggest using a recent release. The ProfitBricks volume plugin has been tested successfully with Docker 17.12.1-ce on both Ubuntu 16.04 and CentOS 7.

The Go programming language [documentation](https://golang.org/doc/install) provides more information on installing and configuring a Go programming environment. Setting up a Go environment is only necessary if you are building the plugin manually, rather than using the release binaries.

## Installation

#### Download

The driver is written in Go and it consists of a single static binary which can be downloaded from the [releases page](https://github.com/profitbricks/docker-volume-profitbricks/releases). Appropriate binaries are made available for different Linux platforms and architectures.

#### Build

You can build the driver yourself if you prefer. [The requirements](#getting-started) have to be fulfilled before running a build.

```
$ go get github.com/profitbricks/docker-volume-profitbricks
$ cd $GOPATH/src/github.com/profitbricks/docker-volume-profitbricks
$ go build
```

#### Application Usage

The ProfitBricks volume plugin can be run manually for testing, or it can be setup as a service. See the [System Integration](#system-integration) section below. Running `docker-volume-profitbricks -h` returns some basic `help` information:

```
$ ./docker-volume-profitbricks  -h
Usage of ./docker-volume-profitbricks:
  --credential-file-path string
    	the path to the credential file
  -l, --log-level string
    	log level (default "error")
  --metadata-path string
    	the path under which to store volume metadata (default "/etc/docker/plugins/profitbricks/volumes")
  -m, --mount-path string
    	the path under which to create the volume mount folders (default "/var/run/docker/volumedriver/profitbricks")
  -d, --profitbricks-datacenter-id string
    	ProfitBricks Virtual Data Center ID (default "863d743f-1730-4ffa-86a4-ee66a3357963")
  -t, --profitbricks-disk-type string
    	ProfitBricks Volume type (default "HDD")
  -e, --profitbricks-endpoint string
      ProfitBricks endpoint
  -p, --profitbricks-password string
    	ProfitBricks password
  -u, --profitbricks-username string
    	ProfitBricks username
  -s, --profitbricks-volume-size int
    	ProfitBricks Volume size (default 50)
  -g, --unix-socket-group string
    	the group to assign to the Unix socket file (default "docker")
  -v, --version
    	outputs the driver version and exits

```

**Please note**: The `--credential-file-path` can be used to provide your ProfitBricks username and password from a JSON formatted text file located anywhere on the servers file system. As an example, your JSON file could look like this:

```
{ "username":"username@domain.tld",
  "password":"5eCUre_RAnD0M-P@S5w0rD"}
```

## Install

To install, copy the release binary into your path and make it executable:

```
$ sudo cp docker-volume-profitbricks /usr/local/bin/
$ sudo chmod +x /usr/local/bin/docker-volume-profitbricks
```

## System Integration

If you wish to have the ProfitBricks volume plugin running under `systemd`:

Edit or create [systemd/docker-volume-profitbricks.default](systemd/docker-volume-profitbricks.default):

```bash
# ProfitBricks credentials and data center
PROFITBRICKS_USERNAME="username"
PROFITBRICKS_PASSWORD="password"
PROFITBRICKS_DATACENTER_ID="000000000-0000-0000-0000-000000000000"
```

Edit or create [systemd/docker-volume-profitbricks.service](systemd/docker-volume-profitbricks.service):

```bash
[Unit]
Description=Docker Volume Driver for ProfitBricks
Before=docker.service
After=network.target
Requires=docker.service

[Service]
EnvironmentFile=/etc/profitbricks/docker-volume-profitbricks.default
ExecStart=/usr/local/bin/docker-volume-profitbricks

[Install]
WantedBy=multi-user.target
```

and copy:

```bash
# copy environment configuration
sudo mkdir -p /etc/profitbricks/ && cp docker-volume-profitbricks.default /etc/profitbricks/
# copy service configuration
sudo cp docker-volume-profitbricks.service /etc/systemd/system/
```

Start the service:

```bash
# execute the driver directly
sudo systemctl start docker-volume-profitbricks

# enable automated startup on reboot
sudo systemctl enable docker-volume-profitbricks
```

## Usage

Create a Docker volume with the `profitbricks` driver:

```bash
docker volume create --driver profitbricks --name test02
# Mount the volume and start an interactive shell to access contents of your ProfitBricks volume from within a container
docker run -ti --rm --volume test02:/mydata busybox sh
```

Once inside the container:

```bash
echo "hello world" > /mydata/hello.txt
cat /mydata/hello.txt
 hello world
```

The current status of the Docker volume can be inspected using the following command:

```bash
docker volume inspect test02
```

```json
[
    {
        "Driver": "profitbricks",
        "Labels": {},
        "Mountpoint": "/var/lib/docker/volumes/test02/_data",
        "Name": "test02",
        "Options": {},
        "Scope": "local"
    }
]
```

If you want to override the default settings for either *volume_size* or *volume_type*, you can do so by passing the options in with `--opt`:

```bash
docker volume create --driver profitbricks --name test02 --opt volume_size=40 --opt volume_type=SSD
```

A Docker volume can be created from existing volume:

```bash
docker volume create --driver profitbricks --name test03 --opt volume_id=[UUID]
# Mount the volume and start an interactive shell to access contents of your ProfitBricks volume from within a container
docker run -ti --rm --volume test03:/mydata busybox sh
```

OR

```bash
docker volume create --driver profitbricks --name test03 --opt volume_name=test03
# Mount the volume and start an interactive shell to access contents of your ProfitBricks volume from within a container
docker run -ti --rm --volume test03:/mydata busybox sh
```

In addition a Docker volume can be created from a snapshot:

```bash
docker volume create --driver profitbricks --name test04 --opt snapshot_id=[UUID]
# Mount the volume and start an interactive shell to access contents of your ProfitBricks volume from within a container
docker run -ti --rm --volume test04:/mydata busybox sh
```

OR

```bash
docker volume create --driver profitbricks --name test04 --opt snapshot_name=test04_snapshot
# Mount the volume and start an interactive shell to access contents of your ProfitBricks volume from within a container
docker run -ti --rm --volume test04:/mydata busybox sh
```

## Support

You are welcome to contact us with questions or comments using the **Community** section of the [ProfitBricks DevOps Central](https://devops.profitbricks.com/). Please report any feature requests or issues using GitHub issue tracker.

* [ProfitBricks Cloud API](https://devops.profitbricks.com/api/cloud/) documentation.
* Ask a question or discuss at [ProfitBricks DevOps Central](https://devops.profitbricks.com/community/).
* Report an [issue here](https://github.com/profitbricks/docker-volume-profitbricks/issues).

## Contributing

1. Fork the repository ([https://github.com/profitbricks/docker-volume-profitbricks/fork](https://github.com/profitbricks/docker-volume-profitbricks/fork))
2. Create your feature branch (`git checkout -b my-new-feature`)
3. Commit your changes (`git commit -am 'Add some feature'`)
4. Push to the branch (`git push origin my-new-feature`)
5. Create a new Pull Request
