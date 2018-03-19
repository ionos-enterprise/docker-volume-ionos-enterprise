package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/creamdog/gonfig"
)

const (
	productUUIDPath = "/sys/devices/virtual/dmi/id/product_uuid"
)

//Utilities is main stucture.
type Utilities struct {
}

//NewUtilities is a constructor.
func NewUtilities() *Utilities {
	return &Utilities{}
}

//GetConfValS is trying to load a string value from a config file.
func (m Utilities) GetConfValS(path string, value string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	config, err := gonfig.FromJson(f)
	if err != nil {
		return "", err
	}

	configValue, err := config.GetString(value, nil)
	if err != nil {
		return "", err
	}

	return configValue, nil
}

//MountVolume is trying to mount a volume.
func (m Utilities) MountVolume(volumeName string, mountPoint string) error {
	log.Infof("Mounting volume %s at %s", volumeName, mountPoint)

	var stdOut, stdErr bytes.Buffer
	cmd := exec.Command("mount", volumeName, mountPoint)
	cmd.Stdout = &stdOut
	cmd.Stderr = &stdErr
	err := cmd.Run()
	log.Infof("Mount stdout: %s", stdOut.String())

	if err != nil {
		return fmt.Errorf("Error occurred while mounting %s: %s", volumeName, stdErr.String())
	}
	return err
}

//UnmountVolume is trying to unmount a volume.
func (m Utilities) UnmountVolume(mountPoint string) error {
	log.Infof("Unmounting volume %s ", mountPoint)
	var stdOut, stdErr bytes.Buffer
	cmd := exec.Command("umount", mountPoint)
	cmd.Stdout = &stdOut
	cmd.Stderr = &stdErr
	err := cmd.Run()
	log.Infof("Umount stdout: %s", stdOut.String())

	if err != nil {
		return fmt.Errorf("Error occurred while unmounting %s: %s", mountPoint, stdErr.String())
	}
	return err
}

//FormatVolume is formating a volume.
func (m Utilities) FormatVolume(volumeName string, volumeID string) error {
	log.Infof("Formating volume %s with uuid %s", volumeName, volumeID)
	var stdOut, stdErr bytes.Buffer
	cmd := exec.Command("mkfs.ext4", volumeName, "-U", volumeID)
	cmd.Stdout = &stdOut
	cmd.Stderr = &stdErr
	return cmd.Run()
}

//TuneVolume is setting a volume uuid to match profitbricks volume id.
func (m Utilities) TuneVolume(volumeName string, volumeID string) error {
	log.Infof("Tuning volume %s with uuid %s", volumeName, volumeID)
	var stdOut, stdErr bytes.Buffer
	cmd := exec.Command("tune2fs", volumeName, "-U", volumeID)
	cmd.Stdout = &stdOut
	cmd.Stderr = &stdErr
	return cmd.Run()
}

//GetServerID is loading server id from a config file.
func (m Utilities) GetServerID() (string, error) {
	output, err := ioutil.ReadFile(productUUIDPath)
	toReturn := string(output)
	return strings.TrimSpace(toReturn), err
}

//WriteLsblk is writing to a metadata file.
func (m Utilities) WriteLsblk(metadataPath string, result Result) error {
	jsn, err := json.MarshalIndent(result, "\t", "\t")
	if err != nil {
		return err
	}
	ioutil.WriteFile(metadataPath, jsn, 0644)

	return err
}

//getNewLsbkl is getting a lsbkl value.
func (m Utilities) getNewLsblk() (Result, error) {
	cmd := exec.Command("lsblk", "-P", "-o", "NAME,MOUNTPOINT,TYPE,UUID")

	data, err := cmd.CombinedOutput()
	if err != nil {
		return Result{}, fmt.Errorf("Error: %s", err.Error())
	}
	result := []*Device{}
	devices := strings.Split(string(data), "\n")
	for _, device := range devices {
		parsed := parseDevice(device)
		if parsed != nil {
			result = append(result, parsed)
		}
	}

	return Result{Devices: result}, err
}

//parseDevice is trying to parse a device name.
func parseDevice(device string) *Device {
	raw := strings.Split(device, " ")
	if len(raw) == 4 {
		name := strings.Split(raw[0], "=")[1]
		mountpoint := strings.Split(raw[1], "=")[1]
		_type := strings.Split(raw[2], "=")[1]
		UUID := strings.Split(raw[3], "=")[1]

		d := &Device{
			Name:       strings.Trim(name, `"`),
			Mountpoint: strings.Trim(mountpoint, `"`),
			Type:       strings.Trim(_type, `"`),
			UUID:       strings.Trim(UUID, `"`),
		}
		return d
	}
	return nil
}

//RemoveMetaDataFile is removing a metadata from a file.
func (m Utilities) RemoveMetaDataFile(metadataFilePath string) error {
	return os.Remove(metadataFilePath)
}

//GetDeviceName is getting a device name.
func (m Utilities) GetDeviceName() (string, bool, error) {
	deviceBaseName := "/dev/%s"
	deviceName := ""
	deviceCounter := 0
	newList, err := m.getNewLsblk()

	for _, device := range newList.Devices {
		// Condition explainations:
		// Every attached volume is type of a disk
		// vda is reserved for OS based disk
		// By default UUID is not assign to a attached volume, and we will make sure to do that for each of them
		if device.Type == "disk" && device.Name != "vda" && len(device.UUID) == 0 {
			deviceName = device.Name
			deviceCounter++
		}
	}
	if deviceCounter > 1 {
		return "", false, fmt.Errorf("There is more than %d new devices", deviceCounter)
	}

	foundDevice := "" != deviceName
	return fmt.Sprintf(deviceBaseName, deviceName), foundDevice, err
}

//IsUUID validates if a provided value is a uuid
func (m Utilities) IsUUID(value string) bool {
	var validUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	return validUUID.MatchString(value)
}

//Result represents an array of devices.
type Result struct {
	Devices []*Device `json:"blockdevices"`
}

//Device represents a device meta data.
type Device struct {
	Name       string
	Type       string
	Mountpoint string
	UUID       string
}
