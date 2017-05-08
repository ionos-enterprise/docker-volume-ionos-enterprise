package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os/exec"
)

type Utilities struct {
}

func NewUtilities() *Utilities {
	return &Utilities{}
}

func (m Utilities) MountVolume(volumeName string, mountpoint string) error {
	cmd := exec.Command("mount", volumeName, mountpoint)
	return cmd.Run()
}

func (m Utilities) UnmountVolume(mountPoint string) error {
	cmd := exec.Command("umount", mountPoint)
	return cmd.Run()
}

func (m Utilities) FormatVolume(volumeName string) error {
	cmd := exec.Command("mkfs.ext4", volumeName)
	return cmd.Run()
}

func (m Utilities) GetServerId() (string, error) {
	output, err := ioutil.ReadFile("/sys/devices/virtual/dmi/id/product_uuid")

	return string(output), err
}

func (m Utilities) GetDeviceName() (string, error) {
	deviceBaseName := "/dev/%s"

	var stdOut, stdErr bytes.Buffer
	cmd := exec.Command("lsblk", "-o", "MOUNTPOINT,NAME", "-J")
	cmd.Stdout = &stdOut
	cmd.Stderr = &stdErr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("Error: %s, %s", err.Error(), stdErr.String())
	}

	resultObj := &Result{}

	json.Unmarshal(stdOut.Bytes(), resultObj)

	for _, b := range resultObj.Blockdevices {
		if b.Mountpoint == "" && len(b.Children) == 0 {
			return fmt.Sprintf(deviceBaseName, b.Name), nil
		}
	}
	return "", err
}

type Result struct {
	Blockdevices []struct {
		Mountpoint string `json:"mountpoint"`
		Name       string `json:"name"`
		Children   []struct {
			Mountpoint string `json:"mountpoint"`
			Name       string `json:"name"`
		} `json:"children,omitempty"`
	} `json:"blockdevices"`
}
