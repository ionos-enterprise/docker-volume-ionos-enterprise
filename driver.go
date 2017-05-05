package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/go-plugins-helpers/volume"
	"github.com/profitbricks/profitbricks-sdk-go"
	"io/ioutil"
	"os"
	"runtime"
	"sync"
	"time"
	"path/filepath"
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
	size         int
	diskType     string
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

	serverId, err := getServerId()

	if err != nil {
		log.Error(err)
		return nil, err
	}
	return &Driver{
		datacenterId: *args.datacenterId,
		serverId:     serverId,
		size:         *args.size,
		diskType:     *args.diskType,
	}, nil

}

func (d *Driver) Create(r volume.Request) volume.Response {
	d.m.Lock()
	defer d.m.Unlock()

	vol := profitbricks.Volume{
		Properties: profitbricks.VolumeProperties{
			Size:        d.size,
			Type:        d.diskType,
			LicenceType: "OTHER",
			Name:        fmt.Sprintf("docker-volume-profitbricks:%s", r.Name),
		},
	}
	vol = profitbricks.CreateVolume(d.datacenterId, vol)

	err := d.waitTillProvisioned(vol.Headers.Get("Location"))

	if err != nil {
		log.Error(err.Error())
		return volume.Response{Err: err.Error()}
	}

	vol = profitbricks.AttachVolume(d.datacenterId, d.serverId, vol.Id)
	err = d.waitTillProvisioned(vol.Headers.Get("Location"))

	if err != nil {
		log.Error(err.Error())
		return volume.Response{Err: err.Error()}
	}

	volumeName, err := d.mountUtil.GetDeviceName()
	if err!=nil{
		log.Error(err.Error())
		return volume.Response{Err: err.Error()}
	}

	volumePath := filepath.Join(d.mountPath, volumeName)

	err = os.MkdirAll(volumePath, MountDirMode)
	if err != nil {
		log.Error(err.Error())
		return volume.Response{Err: err.Error()}
	}

	err = d.mountUtil.MountVolume(volumeName, volumePath)
	if err != nil {
		log.Error(err.Error())
		return volume.Response{Err: err.Error()}
	}

	return volume.Response{}
}

func (d *Driver) List(r volume.Request) volume.Response {
	return volume.Response{}
}

func (d *Driver) Get(r volume.Request) volume.Response {
	return volume.Response{}
}

func (d *Driver) Remove(r volume.Request) volume.Response {
	return volume.Response{}
}

func (d *Driver) Path(r volume.Request) volume.Response {
	return volume.Response{}
}

func (d *Driver) Mount(r volume.MountRequest) volume.Response {
	return volume.Response{}
}

func (d *Driver) Unmount(r volume.UnmountRequest) volume.Response {
	return volume.Response{}
}

func (d *Driver) Capabilities(r volume.Request) volume.Response {
	return volume.Response{}
}

func getServerId() (string, error) {
	output, err := ioutil.ReadFile("/sys/devices/virtual/dmi/id/product_uuid")

	return string(output), err
}

func (d *Driver) waitTillProvisioned(path string) error {

	waitCount := 50

	for i := 0; i < waitCount; i++ {
		request := profitbricks.GetRequestStatus(path)
		pc, _, _, ok := runtime.Caller(1)
		details := runtime.FuncForPC(pc)
		if ok && details != nil {
			log.Printf("[DEBUG] Called from %s", details.Name())
		}
		log.Info("Request status: %s", request.Metadata.Status)
		log.Info("Request status path: %s", path)

		if request.Metadata.Status == "DONE" {
			return nil
		}
		if request.Metadata.Status == "FAILED" {

			return fmt.Errorf("Request failed with following error: %s", request.Metadata.Message)
		}
		time.Sleep(10 * time.Second)
		i++
	}
	return fmt.Errorf("Timeout has expired")
}

func getDeviceName(deviceNumber int64) string {
	alphabet := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z"}

	name := fmt.Sprintf("vd%s", alphabet[deviceNumber - 1])

	return name
}
