package main

import (
	"os"
	flag "github.com/ogier/pflag"
	"fmt"
	"github.com/docker/go-plugins-helpers/volume"
	"github.com/Sirupsen/logrus"
)

type CommandLineArgs struct {
	profitbricksUsername *string
	profitbricksPassword *string
	metadataPath         *string
	mountPath            *string
	unixSocketGroup      *string
	version              *bool
	datacenterId         *string
	serverId             *string
	location             *string
}

const (
	DriverName = "ProfitBricks"
	DefaultBaseMetadataPath = "/etc/docker/plugins/pbvolume/volumes"
	DefaultBaseMountPath = "/mnt/pbvolume"
	DefaultUnixSocketGroup = "docker"
	DriverVersion = "1.0.0"
)

func main() {

	args := parseCommandLineArgs()
	fmt.Println(*args.profitbricksUsername)
	fmt.Println(*args.profitbricksPassword)
	fmt.Println(*args.metadataPath)
	fmt.Println(*args.mountPath)
	fmt.Println(*args.unixSocketGroup)
	fmt.Println(*args.version)
	mountUtil := NewMountUtil()

	driver, err := ProfitBricksDriver(mountUtil, *args)
	if err != nil {
		logrus.Fatalf("failed to create the driver: %v", err)
		os.Exit(1)
	}

	handler := volume.NewHandler(driver)

	//Start listening in a unix socket
	err = handler.ServeUnix(*args.unixSocketGroup, 0)
	if err != nil {
		logrus.Fatalf("failed to bind to the Unix socket: %v", err)
		os.Exit(1)
	}

}

func parseCommandLineArgs() *CommandLineArgs {
	args := &CommandLineArgs{}

	//Credentials
	args.profitbricksUsername = flag.StringP("profitbricks-username", "u", os.Getenv("PROFITBRICKS_USERNAME"), "ProfitBricks user name")
	args.profitbricksPassword = flag.StringP("profitbricks-password", "p", os.Getenv("PROFITBRICKS_PASSWORD"), "ProfitBricks user name")

	//ProfitBricks VDC, server and location parameters
	args.datacenterId = flag.StringP("profitbricks-datacenter", "d", os.Getenv("PROFIT1BRICKS_DATACENTER"), "ProfitBricks Virtual Data Center ID")
	args.serverId = flag.StringP("profitbricks-server", "s", os.Getenv("PROFIT1BRICKS_SERVER"), "ProfitBricks Virtual Data Center ID")
	args.location = flag.StringP("profitbricks-location", "l", "us/las", "ProfitBricks Location")

	//Mount parameters
	args.metadataPath = flag.String("metadata-path", DefaultBaseMetadataPath, "the path under which to store volume metadata")
	args.mountPath = flag.StringP("mount-path", "m", DefaultBaseMountPath, "the path under which to create the volume mount folders")
	args.unixSocketGroup = flag.StringP("unix-socket-group", "g", DefaultUnixSocketGroup, "the group to assign to the Unix socket file")
	args.version = flag.Bool("version", false, "outputs the driver version and exits")
	flag.Parse()

	if *args.version {
		fmt.Printf("%v\n", DriverVersion)
		os.Exit(0)
	}

	if *args.profitbricksUsername == "" || *args.profitbricksPassword == "" {
		flag.Usage()
		os.Exit(1)
	}

	if *args.datacenterId == "" {
		fmt.Errorf("Please provide Virtual Data Center Id  '--profitbricks-datacenter [UUID]'")
		os.Exit(1)
	}

	if *args.serverId == "" {
		fmt.Errorf("Please provide Server Id  '--profitbricks-server [UUID]'")
		os.Exit(1)
	}

	return args
}

