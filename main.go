package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/go-plugins-helpers/volume"
	flag "github.com/ogier/pflag"
	"os"
	"syscall"
)

type CommandLineArgs struct {
	profitbricksUsername *string
	profitbricksPassword *string
	metadataPath         *string
	mountPath            *string
	unixSocketGroup      *string
	version              *bool
	datacenterId         *string
	size                 *int
	diskType             *string
}

const (
	DefaultBaseMetadataPath = "/etc/docker/plugins/profitbricks-volume"
	DefaultBaseMountPath    = "/var/lib/docker-volume-profitbricks"
	DefaultUnixSocketGroup  = "docker"
	DriverVersion           = "1.0.0"
)

func main() {

	args := parseCommandLineArgs()
	fmt.Println(*args.profitbricksUsername)
	fmt.Println(*args.profitbricksPassword)
	fmt.Println(*args.metadataPath)
	fmt.Println(*args.mountPath)
	fmt.Println(*args.unixSocketGroup)
	fmt.Println(*args.version)
	mountUtil := NewUtilities()

	driver, err := ProfitBricksDriver(mountUtil, *args)
	if err != nil {
		log.Fatalf("failed to create the driver: %v", err)
		os.Exit(1)
	}

	handler := volume.NewHandler(driver)

	//Start listening in a unix socket
	err = handler.ServeUnix(*args.unixSocketGroup, syscall.Getegid())
	if err != nil {
		log.Fatalf("failed to bind to the Unix socket: %v", err)
		os.Exit(1)
	}

}

func parseCommandLineArgs() *CommandLineArgs {
	args := &CommandLineArgs{}

	log.Info("USERNAME", os.Getenv("PROFITBRICKS_USERNAME"))
	log.Info("PASSWORD", os.Getenv("PROFITBRICKS_PASSWORD"))
	//Credentials
	args.profitbricksUsername = flag.StringP("profitbricks-username", "u", "", "ProfitBricks user name")
	args.profitbricksPassword = flag.StringP("profitbricks-password", "p", "", "ProfitBricks user name")

	//ProfitBricks VDC, server and location parameters
	args.datacenterId = flag.StringP("profitbricks-datacenter", "d", os.Getenv("PROFITBRICKS_DATACENTER"), "ProfitBricks Virtual Data Center ID")
	args.size = flag.IntP("profitbricks-volume-size", "s", 50, "ProfitBricks Volume size")
	args.diskType = flag.StringP("profitbricks-disk-type", "t", "HDD", "ProfitBricks Volume type")

	//Mount parameters
	args.metadataPath = flag.String("metadata-path", DefaultBaseMetadataPath, "the path under which to store volume metadata")
	args.mountPath = flag.StringP("mount-path", "m", DefaultBaseMountPath, "the path under which to create the volume mount folders")
	args.unixSocketGroup = flag.StringP("unix-socket-group", "g", DefaultUnixSocketGroup, "the group to assign to the Unix socket file")
	args.version = flag.BoolP("version", "v", false, "outputs the driver version and exits")
	flag.Parse()

	if *args.version {
		fmt.Printf("%v\n", DriverVersion)
		os.Exit(0)
	}

	*args.profitbricksUsername = os.Getenv("PROFITBRICKS_USERNAME")
	*args.profitbricksPassword = os.Getenv("PROFITBRICKS_PASSWORD")
	if *args.profitbricksUsername == "" || *args.profitbricksPassword == "" {
		fmt.Println(fmt.Errorf("Credentials should be provided either using %q, %q or using environment variables %q, %q", "--profitbricks-username", "--profitbricks-password", "PROFITBRICKS_USERNAME", "PROFITBRICKS_PASSWORD"))
		os.Exit(1)
	}

	if *args.datacenterId == "" {
		fmt.Println(fmt.Errorf("Please provide Virtual Data Center Id %q or using environment variable %q", "--profitbricks-datacenter [UUID]", "PROFITBRICKS_DATACENTER"))
		os.Exit(1)
	}

	return args
}
