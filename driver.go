package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/go-plugins-helpers/volume"
	"github.com/profitbricks/profitbricks-sdk-go"
)

//Constances used at driver level
const (
	metadataDirMode  = 0700
	metadataFileMode = 0600
	mountDirMode     = os.ModeDir
	etag             = "docker-volume"
)

//Driver represents main class.
type Driver struct {
	metadataPath string
	mountPath    string
	datacenterID string
	serverID     string
	size         int
	diskType     string
	utilities    *Utilities
	sync.RWMutex
	volumes map[string]*volumeState
	client  *profitbricks.Client
}

//VolumeState represents a volume state in the  metadata.
type volumeState struct {
	VolumeID   string
	MountPoint string
	DeviceName string
}

//ProfitBricksDriver is a constuctor of the driver.
func ProfitBricksDriver(utilities *Utilities, args CommandLineArgs) (*Driver, error) {
	client := profitbricks.NewClient(
		*args.profitbricksUsername,
		*args.profitbricksPassword,
	)

	err := os.MkdirAll(*args.metadataPath, metadataDirMode)
	if err != nil {
		return nil, err
	}

	err = os.MkdirAll(*args.mountPath, mountDirMode)
	if err != nil {
		return nil, err
	}

	serverID, err := utilities.GetServerID()

	if err != nil {
		log.Error(err)
		return nil, err
	}

	log.Info("Server ID:", strings.ToLower(serverID))

	driver := &Driver{
		datacenterID: *args.datacenterID,
		serverID:     strings.ToLower(serverID),
		size:         *args.size,
		diskType:     *args.diskType,
		volumes:      make(map[string]*volumeState),
		metadataPath: *args.metadataPath,
		utilities:    utilities,
		mountPath:    *args.mountPath,
		client:       client,
	}

	ierr := driver.initVolumesFromMetadata()
	if ierr != nil {
		return nil, ierr
	}

	return driver, nil
}

//Create is creating a new instance of a volume.
func (d *Driver) Create(r volume.Request) volume.Response {
	d.Lock()
	defer d.Unlock()
	log.Info("Creating a new volume")

	isNewVolume := true
	shouldDoFormatting := true
	volumeID := ""
	snapshotID := ""
	diskSize := d.size
	diskType := d.diskType
	var err error

	diskSizeParam := r.Options["volume_size"]
	if len(diskSizeParam) > 0 {
		diskSize, err = strconv.Atoi(diskSizeParam)
		if err != nil {
			log.Error(err.Error())
			return volume.Response{Err: err.Error()}
		}
	}

	diskTypeParam := r.Options["volume_type"]
	if len(diskTypeParam) > 0 {
		diskType = diskTypeParam
	}

	vol := profitbricks.Volume{
		Properties: profitbricks.VolumeProperties{
			Size:        diskSize,
			Type:        diskType,
			LicenceType: "OTHER",
			Name:        fmt.Sprintf("%s:%s", r.Name, etag),
		},
	}

	//Tries to discover a volume and make sure it exists
	volumeID, err = d.findVolumeByRequest(r)
	if err != nil {
		log.Error(err.Error())
		return volume.Response{Err: err.Error()}
	}

	volumeID, isNewVolume, shouldDoFormatting, err = d.findVolumeByID(volumeID, &vol, isNewVolume, shouldDoFormatting, r)
	if err != nil {
		log.Error(err.Error())
		return volume.Response{Err: err.Error()}
	}

	//Tries to discover a snapshot and make sure it exists
	snapshotID, err = d.findSnapshotByName(r)
	if err != nil {
		log.Error(err.Error())
		return volume.Response{Err: err.Error()}
	}

	snapshotID, isNewVolume, shouldDoFormatting, err = d.findSnapshotByID(snapshotID, volumeID, &vol, isNewVolume, shouldDoFormatting, r)
	if err != nil {
		log.Error(err.Error())
		return volume.Response{Err: err.Error()}
	}

	if isNewVolume {
		//Check volume name is unique in the datacenter
		volumesresp, err := d.client.ListVolumes(d.datacenterID)
		if err != nil {
			log.Errorf("failed to create a volume '%v'", r.Name)
			return volume.Response{Err: err.Error()}
		}

		log.Info(volumesresp)

		for _, v := range volumesresp.Items {
			if v.Properties.Name == vol.Properties.Name {
				errorAlreadyExists := fmt.Sprintf("failed to create volume '%s', volume with this name already exists in datacenter '%s'", r.Name, d.datacenterID)
				log.Errorf(errorAlreadyExists)
				return volume.Response{Err: errorAlreadyExists}
			}
		}

		//Creates a volume
		createresp, err := d.client.CreateVolume(d.datacenterID, vol)
		log.Info(createresp)
		if err != nil {
			log.Errorf("failed to create a volume '%v'", r.Name)
			return volume.Response{Err: err.Error()}
		}

		volumeID = createresp.ID
		log.Info("Volume provisioned:", vol.Properties.Name)

		err = d.waitTillProvisioned(createresp.Headers.Get("Location"))

		if err != nil {
			log.Error(err.Error())
			return volume.Response{Err: err.Error()}
		}
	}

	//Attach volume
	attachResp, err := d.client.AttachVolume(d.datacenterID, d.serverID, volumeID)
	if err != nil {
		log.Errorf("Arguments: %s %s %s", d.datacenterID, d.serverID, vol.ID)
		log.Errorf("failed to attach a volume '%v', error msg: %q", r.Name, attachResp.Response)
		return volume.Response{Err: err.Error()}
	}

	err = d.waitTillProvisioned(attachResp.Headers.Get("Location"))
	log.Info("Volume attached:", attachResp.Properties.Name)

	if err != nil {
		log.Error(err.Error())
		return volume.Response{Err: err.Error()}
	}

	//Sets a metadata
	log.Info("Getting device name from : ", d.metadataPath)
	volumeName, foundDevice, err := d.utilities.GetDeviceName()
	if err != nil {
		log.Error(err.Error())
		return volume.Response{Err: err.Error()}
	}

	if foundDevice {
		//Sets a partition
		if shouldDoFormatting {
			log.Info("Starting formatting: VolumeName: ", volumeName, " VolumeId: ", volumeID)
			err = d.utilities.FormatVolume(volumeName, volumeID)
			if err != nil {
				log.Error(err.Error())
				return volume.Response{Err: err.Error()}
			}
		} else {
			log.Info("Adjusting volume: VolumeName: ", volumeName, " VolumeId: ", volumeID)
			err = d.utilities.TuneVolume(volumeName, volumeID)
			if err != nil {
				log.Error(err.Error())
				return volume.Response{Err: err.Error()}
			}
		}
	}

	volumePath := filepath.Join(d.mountPath, volumeID)
	log.Info("Make directory for VolumePath: ", volumePath)
	err = os.MkdirAll(volumePath, mountDirMode)
	if err != nil {
		log.Error(err.Error())
		return volume.Response{Err: err.Error()}
	}

	metadataFilePath := filepath.Join(d.metadataPath, r.Name)

	log.Infof("Metadata file path %s", metadataFilePath)

	metadataFile, err := os.Create(metadataFilePath)
	if err != nil {
		log.Errorf("failed to create metadata file '%v' for volume '%v'", metadataFilePath, r.Name)
		return volume.Response{Err: err.Error()}
	}

	err = metadataFile.Chmod(metadataFileMode)
	if err != nil {
		os.Remove(metadataFilePath)
		log.Errorf("failed to change the mode for the metadata file '%v' for volume '%v'", metadataFilePath, r.Name)
		return volume.Response{Err: err.Error()}
	}

	d.volumes[r.Name] = &volumeState{
		VolumeID:   volumeID,
		MountPoint: volumePath,
		DeviceName: volumeName,
	}

	jsn, _ := json.MarshalIndent(d.volumes, "", "\t")
	log.Info("Volumes: ", string(jsn))

	jsn, err = json.Marshal(d.volumes[r.Name])
	if err != nil {
		return volume.Response{Err: err.Error()}
	}

	detachResp, err := d.client.DetachVolume(d.datacenterID, d.serverID, volumeID)
	if err != nil {
		log.Errorf("failed to detach volume '%v' on server '%v'", volumeID, d.serverID)
		return volume.Response{Err: err.Error()}
	}

	err = d.waitTillProvisioned(detachResp.Get("Location"))
	if err != nil {
		return volume.Response{Err: err.Error()}
	}

	return volume.Response{}
}

//Mount is attaching and mounting a volume.
func (d *Driver) Mount(r volume.MountRequest) volume.Response {
	d.Lock()
	defer d.Unlock()
	log.Infof("Mounting Volume: %s", r.Name)

	vol := d.volumes[r.Name]
	log.Info(vol.DeviceName)

	attachResp, err := d.client.AttachVolume(d.datacenterID, d.serverID, vol.VolumeID)
	if err != nil {
		log.Errorf("Arguments: %s %s %s", d.datacenterID, d.serverID, vol.VolumeID)
		log.Errorf("failed to attach a volume '%v', error msg: %q", r.Name, attachResp.Response)
		return volume.Response{Err: err.Error()}
	}

	err = d.waitTillProvisioned(attachResp.Headers.Get("Location"))
	log.Info("Volume attached:", attachResp.Properties.Name)

	volumePath := filepath.Join("/dev", "disk", "by-uuid", vol.VolumeID)
	err = d.utilities.MountVolume(volumePath, vol.MountPoint)
	if err != nil {
		log.Error(err.Error())
		return volume.Response{Err: err.Error()}
	}

	return volume.Response{
		Mountpoint: vol.MountPoint,
	}
}

//getVolumeDevicePath returning device path with uuid.
func (d *Driver) getVolumeDevicePath(volumeID string) string {
	return filepath.Join("/dev", "disk", "by-uuid", volumeID)
}

//Unmount is detacing and unmounting a volume.
func (d *Driver) Unmount(r volume.UnmountRequest) volume.Response {
	d.Lock()
	defer d.Unlock()
	log.Info("Unmounting Volume")

	vol := d.volumes[r.Name]
	volumePath := d.getVolumeDevicePath(vol.VolumeID)
	err := d.utilities.UnmountVolume(volumePath)
	if err != nil {
		log.Error("Error occured while unmounting volume", err.Error())
		return volume.Response{Err: err.Error()}
	}

	detachResp, err := d.client.DetachVolume(d.datacenterID, d.serverID, vol.VolumeID)
	if err != nil {
		log.Errorf("failed to detach volume '%v' on server '%v'", vol.VolumeID, d.serverID)
		return volume.Response{Err: err.Error()}
	}

	err = d.waitTillProvisioned(detachResp.Get("Location"))
	if err != nil {
		return volume.Response{Err: err.Error()}
	}

	return volume.Response{}
}

//List is showing all volumes related for the driver.
func (d *Driver) List(r volume.Request) volume.Response {
	d.Lock()
	defer d.Unlock()
	volumes := []*volume.Volume{}
	log.Info("Getting a Volume")

	for name, state := range d.volumes {
		volumes = append(volumes, &volume.Volume{
			Name:       name,
			Mountpoint: state.MountPoint,
		})
	}
	return volume.Response{Volumes: volumes}
}

//Get is showing a volume meta info.
func (d *Driver) Get(r volume.Request) volume.Response {
	log.Info("Getting a Volume")

	if d.volumes[r.Name] == nil {
		return volume.Response{}
	}
	vol := &volume.Volume{
		Name:       d.volumes[r.Name].DeviceName,
		Mountpoint: d.volumes[r.Name].MountPoint,
	}

	return volume.Response{Volume: vol}
}

//Remove is removing volume from a server and the Profitbricks data center.
func (d *Driver) Remove(r volume.Request) volume.Response {
	d.Lock()
	defer d.Unlock()
	log.Info("Iterating throug map")

	vol := &volumeState{}
	var key string
	for k, v := range d.volumes {
		log.Infof("Key %s", k)
		log.Infof("v.MountPoint == r.Name ", v.MountPoint == r.Name)
		if v.DeviceName == r.Name {
			key = k
			vol = v
			break
		}
	}

	alreadyRemoved := false
	//Try to detach the volume, so it could be deleted.
	resp, err := d.client.DetachVolume(d.datacenterID, d.serverID, vol.VolumeID)
	if err != nil {
		if apiError, ok := err.(profitbricks.ApiError); ok {
			if apiError.HttpStatusCode() == 404 {
				alreadyRemoved = true
			} else {
				log.Errorf("failed to detach volume '%v' on server '%v'", vol.VolumeID, d.serverID)
				return volume.Response{Err: err.Error()}
			}
		} else {
			return volume.Response{Err: fmt.Sprintf("invalid response: %s", err.Error())}
		}
	}

	if !alreadyRemoved {
		err := d.waitTillProvisioned(resp.Get("Location"))
		if err != nil {
			return volume.Response{Err: err.Error()}
		}
	}

	resp, err = d.client.DeleteVolume(d.datacenterID, vol.VolumeID)
	if err != nil {
		log.Errorf("failed to delete volume '%s' from data center '%s'", vol.VolumeID, d.datacenterID)
		return volume.Response{Err: err.Error()}
	}

	err = d.waitTillProvisioned(resp.Get("Location"))
	if err != nil {
		return volume.Response{Err: err.Error()}
	}

	//Remove mount folder
	err = os.Remove(vol.MountPoint)
	if err != nil {
		log.Error(err.Error())
		return volume.Response{Err: err.Error()}
	}

	metadataFilePath := filepath.Join(d.metadataPath, key)
	err = os.Remove(metadataFilePath)
	if err != nil {
		log.Error(err.Error())
		return volume.Response{Err: err.Error()}
	}

	delete(d.volumes, key)

	if err != nil {
		return volume.Response{Err: err.Error()}
	}

	return volume.Response{}
}

//Path is returing path info.
func (d *Driver) Path(r volume.Request) volume.Response {
	d.Lock()
	defer d.Unlock()

	if state, ok := d.volumes[r.Name]; ok {
		return volume.Response{Mountpoint: state.MountPoint}
	}

	return volume.Response{Err: fmt.Sprintf("Volume %q does not exist", r.Name)}
}

//Capabilities is returning info about scope.
func (d *Driver) Capabilities(r volume.Request) volume.Response {
	log.Infof("[Capabilities]: %+v", r)
	return volume.Response{Capabilities: volume.Capability{Scope: "global"}}
}

//findVolumeByName is trying to discover a volume by name.
func (d *Driver) findVolumeByRequest(r volume.Request) (string, error) {
	volumeName := r.Options["volume_name"]
	if len(volumeName) > 0 {
		if r.Name != volumeName {
			return "", fmt.Errorf("Volume name %s and volume_name parameter %s have to be the same", r.Name, volumeName)
		}
		return d.findVolumeByName(volumeName)
	}

	return "", nil
}

//findVolumeByName is trying to discover a volume by name.
func (d *Driver) findVolumeByName(volumeName string) (string, error) {
	if len(volumeName) > 0 {
		volumeID := ""

		log.Info("Using provided volume_name: ", volumeName)
		//Check volume name is unique in the datacenter
		volumesresp, err := d.client.ListVolumes(d.datacenterID)
		if err != nil {
			log.Errorf("failed to list volumes in dc '%v'", d.datacenterID)
			return "", fmt.Errorf(volumesresp.Response)
		}

		for _, v := range volumesresp.Items {
			if v.Properties.Name == volumeName {
				volumeID = v.ID
				log.Infof("Found volume uuid %s with name %s", volumeID, volumeName)
				return volumeID, nil
			}
		}
		//Try to discover volume with etag suffix
		if !strings.HasSuffix(volumeName, etag) {
			volumeSuffixName := fmt.Sprintf("%s:%s", volumeName, etag)
			for _, v := range volumesresp.Items {
				if v.Properties.Name == volumeSuffixName {
					volumeID = v.ID
					log.Infof("Found volume uuid %s with name %s", volumeID, volumeSuffixName)
					return volumeID, nil
				}
			}
		}
		return "", fmt.Errorf("Volume with name %s could not be found", volumeName)
	}

	return "", nil
}

//findVolumeByID is trying to discover a volume by volumeId.
func (d *Driver) findVolumeByID(volumeID string, vol *profitbricks.Volume, isNewVolume bool, shouldDoFormatting bool, r volume.Request) (string, bool, bool, error) {
	log.Debugf("Using volumeID before the options. Value: %s", volumeID)
	if !d.utilities.IsUUID(volumeID) {
		volumeID = r.Options["volume_id"]
		log.Debugf("Using volumeID from the options. Value: %s", volumeID)
	}
	if d.utilities.IsUUID(volumeID) {
		log.Infof("Using provided volume_id: %s", volumeID)

		volResp, err := d.client.GetVolume(d.datacenterID, volumeID)
		if err != nil {
			return "", isNewVolume, shouldDoFormatting, fmt.Errorf("Volume with uuid %s could not be found", volumeID)
		}
		log.Info(volResp)
		//Adding docker suffix tag in case it is not added
		if !(strings.HasSuffix(volResp.Properties.Name, etag)) {
			log.Infof("Update name of the volume %s with the suffix %s", volResp.Properties.Name, etag)
			volProps := profitbricks.VolumeProperties{
				Name: fmt.Sprintf("%s:%s", r.Name, etag),
			}

			volEditResp, err := d.client.UpdateVolume(d.datacenterID, volumeID, volProps)
			if err != nil {
				return "", isNewVolume, shouldDoFormatting, fmt.Errorf("Volume with uuid %s could not be updated", volumeID)
			}

			vol.Properties.Name = volEditResp.Properties.Name

		}

		isNewVolume = false
		shouldDoFormatting = false
	}

	return volumeID, isNewVolume, shouldDoFormatting, nil
}

//findSnapshotByName is trying to discover a snapshot by name.
func (d *Driver) findSnapshotByName(r volume.Request) (string, error) {
	snapshotName := r.Options["snapshot_name"]
	snapshotID := ""
	if len(snapshotName) > 0 {
		log.Info("Using provided snapshot_name: ", snapshotName)
		//Check volume name is unique in the datacenter
		snapshotsresp, err := d.client.ListSnapshots()
		if err != nil {
			log.Errorf("failed to create a volume '%v'", r.Name)
			return "", fmt.Errorf(err.Error())
		}
		log.Info(snapshotsresp)

		for _, v := range snapshotsresp.Items {
			if v.Properties.Name == snapshotName {
				snapshotID = v.ID
			}
		}
		if !d.utilities.IsUUID(snapshotID) {
			return "", fmt.Errorf("Snapshot with name %s could not be found", snapshotName)
		}
	}

	return "", nil
}

//findSnapshotByID is trying to discover a snapshot by snapshotId.
func (d *Driver) findSnapshotByID(snapshotID string, volumeID string, vol *profitbricks.Volume, isNewVolume bool, shouldDoFormatting bool, r volume.Request) (string, bool, bool, error) {
	if !d.utilities.IsUUID(volumeID) && d.utilities.IsUUID(snapshotID) {
		snapshotID = r.Options["snapshot_id"]
	}
	if !d.utilities.IsUUID(volumeID) && d.utilities.IsUUID(snapshotID) {
		log.Info("Using provided shanpshot: ", snapshotID)

		snapshotResp, err := d.client.GetSnapshot(snapshotID)
		if err != nil {
			return "", isNewVolume, shouldDoFormatting, fmt.Errorf("Snapshot with uuid %s could not be found", snapshotID)
		}
		log.Info(snapshotResp)
		vol.Properties.Image = snapshotID
		vol.Properties.LicenceType = ""
		isNewVolume = true
		shouldDoFormatting = false
	}

	return snapshotID, isNewVolume, shouldDoFormatting, nil
}

//initVolumesFromMetadata init volumes from the meta data.
func (d *Driver) initVolumesFromMetadata() error {
	metadataFiles, ferr := ioutil.ReadDir(d.metadataPath)
	if ferr != nil {
		return ferr
	}

	for _, metadataFile := range metadataFiles {
		volumeName := metadataFile.Name()
		metadataFilePath := filepath.Join(d.metadataPath, volumeName)

		log.Infof("Initializing volume '%v' from metadata file '%v'", volumeName, metadataFilePath)

		volumeState, ierr := d.initVolume(volumeName)
		if ierr != nil {
			return ierr
		}

		d.volumes[volumeName] = volumeState
	}

	return nil
}

//initVolume init volume from the API.
func (d *Driver) initVolume(name string) (*volumeState, error) {
	volumeID, _ := d.findVolumeByName(name)
	if volumeID == "" {
		log.Errorf("Volume '%v' not found", name)
		return nil, fmt.Errorf("Volume '%v' not found", name)
	}

	volumePath := filepath.Join(d.mountPath, volumeID)

	merr := os.MkdirAll(volumePath, mountDirMode)
	if merr != nil {
		log.Errorf("failed to create the volume mount path '%v'", volumePath)
		return nil, fmt.Errorf("failed to create the volume mount path '%v'", volumePath)
	}

	d.utilities.UnmountVolume(d.getVolumeDevicePath(volumeID))

	volumeState := &volumeState{
		VolumeID:   volumeID,
		MountPoint: volumePath,
	}

	return volumeState, nil
}

//waitTillProvisioned is wating till a Profitbricks long executing request is done.
func (d *Driver) waitTillProvisioned(path string) error {
	for {
		request, err := d.client.GetRequestStatus(path)
		if err != nil {
			return fmt.Errorf("failed to get request status for %s", path)
		}
		log.Debugf("Request status: %s", request.Metadata.Status)
		log.Debugf("Request status path: %s", path)

		if request.Metadata.Status == "DONE" {
			return nil
		}
		if request.Metadata.Status == "FAILED" {

			return fmt.Errorf("Request failed with following error: %s", request.Metadata.Message)
		}
		time.Sleep(10 * time.Second)
	}
}
