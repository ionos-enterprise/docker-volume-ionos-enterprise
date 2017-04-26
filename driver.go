package main

import (
	"sync"
	"os"
	"github.com/profitbricks/profitbricks-sdk-go"
	"github.com/docker/go-plugins-helpers/volume"
)

const (
	MetadataDirMode = 0700
	MetadataFileMode = 0600
	MountDirMode = os.ModeDir
)

type Driver struct {
	region       string
	dropletID    int
	metadataPath string
	mountPath    string
	datacenterId string
	serverId     string
	mountUtil    *MountUtil
	m            *sync.Mutex
}

func ProfitBricksDriver(mountutil *MountUtil, args CommandLineArgs) (*Driver, error) {

	profitbricks.SetAuth(*args.profitbricksUsername, *args.profitbricksPassword)

	err := os.MkdirAll(*args.metadataPath, MetadataDirMode)
	if err != nil {
		return nil, err
	}

	err = os.MkdirAll(*args.mountPath, MountDirMode)
	if err != nil {
		return nil, err
	}

	return &Driver{
		datacenterId: *args.datacenterId,
		serverId: *args.serverId,

	}, nil

}

func (d *Driver)Create(r volume.Request) volume.Response {
	return volume.Response{}
}

func (d *Driver)List(r volume.Request) volume.Response {
	return volume.Response{}
}

func (d *Driver)Get(r volume.Request) volume.Response {
	return volume.Response{}
}

func (d *Driver)Remove(r volume.Request) volume.Response {
	return volume.Response{}
}

func (d *Driver)Path(r volume.Request) volume.Response {
	return volume.Response{}
}

func (d *Driver)Mount(r volume.MountRequest) volume.Response {
	return volume.Response{}
}

func (d *Driver)Unmount(r volume.UnmountRequest) volume.Response {
	return volume.Response{}
}

func (d *Driver) Capabilities(r volume.Request) volume.Response {
	return volume.Response{}
}