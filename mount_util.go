package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
)

const mountDevicePrefix = "/dev/disk/by-id/scsi-0DO_Volume_"

type MountUtil struct {
}

func NewMountUtil() *MountUtil {
	return &MountUtil{}
}

func (m MountUtil) MountVolume(volumeName string, mountpoint string) error {
	cmd := exec.Command("mount", mountDevicePrefix + volumeName, mountpoint)
	return cmd.Run()
}

func (m MountUtil) UnmountVolume(volumeName string, mountpoint string) error {
	cmd := exec.Command("umount", mountpoint)
	return cmd.Run()
}

func (m MountUtil) GetDeviceName() (string, error) {
	deviceBaseName := "/dev/%s"

	var stdOut, stdErr bytes.Buffer
	cmd := exec.Command("lsblk", "-o", "MOUNTPOINT,NAME", "-J")
	cmd.Stdout = &stdOut
	cmd.Stderr = &stdErr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("Error: %s", err.Error(), stdErr.String())
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
