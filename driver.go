package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/go-plugins-helpers/volume"
	"github.com/profitbricks/profitbricks-sdk-go"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	MetadataDirMode  = 0700
	MetadataFileMode = 0600
	MountDirMode     = os.ModeDir
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
	utilities    *Utilities
	m            *sync.Mutex
	volumes      map[string]*VolumeState
}

type VolumeState struct {
	volumeId   string
	mountPoint string
	deviceName string
}

func ProfitBricksDriver(utilities *Utilities, args CommandLineArgs) (*Driver, error) {

	profitbricks.SetAuth(*args.profitbricksUsername, *args.profitbricksPassword)

	err := os.MkdirAll(*args.metadataPath, MetadataDirMode)
	if err != nil {
		return nil, err
	}

	err = os.MkdirAll(*args.mountPath, MountDirMode)
	if err != nil {
		return nil, err
	}

	serverId, err := utilities.GetServerId()

	if err != nil {
		log.Error(err)
		return nil, err
	}
	return &Driver{
		datacenterId: *args.datacenterId,
		serverId:     serverId,
		size:         *args.size,
		diskType:     *args.diskType,
		volumes:      make(map[string]*VolumeState),
		metadataPath: *args.metadataPath,
		utilities:    utilities,
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

	volumeName, err := d.utilities.GetDeviceName()
	if err != nil {
		log.Error(err.Error())
		return volume.Response{Err: err.Error()}
	}

	volumePath := filepath.Join(d.mountPath, volumeName)

	err = os.MkdirAll(volumePath, MountDirMode)
	if err != nil {
		log.Error(err.Error())
		return volume.Response{Err: err.Error()}
	}

	metadataFilePath := filepath.Join(d.metadataPath, r.Name)

	metadataFile, err := os.Create(metadataFilePath)
	if err != nil {
		log.Errorf("failed to create metadata file '%v' for volume '%v'", metadataFilePath, r.Name)
		return volume.Response{Err: err.Error()}
	}

	err = metadataFile.Chmod(MetadataFileMode)
	if err != nil {
		os.Remove(metadataFilePath)
		log.Errorf("failed to change the mode for the metadata file '%v' for volume '%v'", metadataFilePath, r.Name)
		return volume.Response{Err: err.Error()}
	}

	d.volumes[r.Name] = &VolumeState{
		volumeId:   vol.Id,
		mountPoint: volumePath,
		deviceName: volumeName,
	}

	return volume.Response{}
}

func (d *Driver) Mount(r volume.MountRequest) volume.Response {
	d.m.Lock()
	defer d.m.Unlock()
	err := d.utilities.MountVolume(r.Name, d.volumes[r.Name].mountPoint)
	if err != nil {
		log.Error(err.Error())
		return volume.Response{Err: err.Error()}
	}

	return volume.Response{}
}

func (d *Driver) Unmount(r volume.UnmountRequest) volume.Response {
	d.m.Lock()
	defer d.m.Unlock()

	err := d.utilities.UnmountVolume(d.volumes[r.Name].mountPoint)
	if err != nil {
		log.Error("Error occured while unmounting volume", err.Error())
		return volume.Response{Err: err.Error()}
	}
	return volume.Response{}
}

func (d *Driver) List(r volume.Request) volume.Response {
	d.m.Lock()
	defer d.m.Unlock()
	return volume.Response{}
}

func (d *Driver) Get(r volume.Request) volume.Response {
	volumes := []*volume.Volume{}

	for name, state := range d.volumes {
		volumes = append(volumes, &volume.Volume{
			Name:       name,
			Mountpoint: state.mountPoint,
		})
	}
	return volume.Response{Volumes: volumes}
}

func (d *Driver) Remove(r volume.Request) volume.Response {
	d.m.Lock()
	defer d.m.Unlock()

	resp := profitbricks.DetachVolume(d.datacenterId, d.serverId, d.volumes[r.Name].volumeId)
	if resp.StatusCode > 299 {
		log.Errorf("failed to create metadata file '%v' for volume '%v'", d.metadataPath, r.Name)
		return volume.Response{Err: string(resp.Body)}
	}

	err := d.waitTillProvisioned(resp.Headers.Get("Location"))
	if err != nil {
		return volume.Response{Err: err.Error()}
	}

	resp = profitbricks.DeleteVolume(d.datacenterId, d.volumes[r.Name].volumeId)
	if resp.StatusCode > 299 {
		log.Errorf("failed to create metadata file '%v' for volume '%v'", d.metadataPath, r.Name)
		return volume.Response{Err: string(resp.Body)}
	}

	err = d.waitTillProvisioned(resp.Headers.Get("Location"))
	if err != nil {
		return volume.Response{Err: err.Error()}
	}

	return volume.Response{}
}

func (d *Driver) Path(r volume.Request) volume.Response {
	d.m.Lock()
	defer d.m.Unlock()

	if state, ok := d.volumes[r.Name]; ok {
		return volume.Response{Mountpoint: state.mountPoint}
	}

	return volume.Response{Err: fmt.Sprintf("Volume %q does not exist", r.Name)}
}

func (d *Driver) Capabilities(r volume.Request) volume.Response {
	return volume.Response{Capabilities: volume.Capability{Scope: "local"}}
}

func (d *Driver) waitTillProvisioned(path string) error {

	waitCount := 50

	for i := 0; i < waitCount; i++ {
		request := profitbricks.GetRequestStatus(path)
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
	return fmt.Errorf("Timeout has expired %s", "")
}

//
//func getDeviceName(deviceNumber int64) string {
//	alphabet := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z"}
//
//	name := fmt.Sprintf("vd%s", alphabet[deviceNumber - 1])
//
//	return name
//}
