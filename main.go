package main

import (
	"fmt"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/go-plugins-helpers/volume"
	flag "github.com/ogier/pflag"
)

//CommandLineArgs represent the parameters application could accept.
type CommandLineArgs struct {
	profitbricksEndpoint *string
	profitbricksUsername *string
	profitbricksPassword *string
	metadataPath         *string
	mountPath            *string
	unixSocketGroup      *string
	version              *bool
	datacenterID         *string
	size                 *int
	diskType             *string
	credentialFilePath   *string
	logLevel             *string
}

//Constances used at application level.
const (
	driverName              = "profitbricks"
	defaultBaseMetadataPath = "/etc/docker/plugins/profitbricks/volumes"
	defaultBaseMountPath    = "/var/run/docker/volumedriver/profitbricks"
	defaultUnixSocketGroup  = "docker"
	driverVersion           = "1.0.0"
)

func main() {

	mountUtil := NewUtilities()
	args := parseCommandLineArgs(mountUtil)

	logLevel, err := log.ParseLevel(*args.logLevel)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	log.SetLevel(logLevel)

	log.Infof("initialization parameters: profitbricks-endpoint=%s profitbricks-username=%s credential-file-path=%s profitbricks-datacenter-id=%s profitbricks-volume-size=%d profitbricks-disk-type=%s metadata-path=%s mount-path=%s unix-socket-group=%s version=%s log-level=%s",
		*args.profitbricksEndpoint, *args.profitbricksUsername,
		*args.credentialFilePath, *args.datacenterID, *args.size,
		*args.diskType, *args.metadataPath, *args.mountPath,
		*args.unixSocketGroup, *args.version, *args.logLevel)

	driver, err := ProfitBricksDriver(mountUtil, *args)
	if err != nil {
		log.Fatalf("failed to create the driver: %v", err)
		os.Exit(1)
	}
	handler := volume.NewHandler(driver)

	//Start listening in a unix socket
	log.Info("Listening on", *args.unixSocketGroup)
	err = handler.ServeUnix(*args.unixSocketGroup, driverName)
	if err != nil {
		log.Fatalf("failed to bind to the Unix socket: %v", err)
		os.Exit(1)
	}
}

func parseCommandLineArgs(mountUtil *Utilities) *CommandLineArgs {
	args := &CommandLineArgs{}
	var err error

	//Credentials
	args.profitbricksEndpoint = flag.StringP("profitbricks-endpoint", "e", "", "ProfitBricks endpoint")
	args.profitbricksUsername = flag.StringP("profitbricks-username", "u", "", "ProfitBricks username")
	args.profitbricksPassword = flag.StringP("profitbricks-password", "p", "", "ProfitBricks password")
	args.credentialFilePath = flag.String("credential-file-path", "", "the path to the credential file")

	//ProfitBricks VDC, server and location parameters
	args.datacenterID = flag.StringP("profitbricks-datacenter-id", "d", os.Getenv("PROFITBRICKS_DATACENTER_ID"), "ProfitBricks Virtual Data Center ID")
	args.size = flag.IntP("profitbricks-volume-size", "s", 50, "ProfitBricks Volume size")
	args.diskType = flag.StringP("profitbricks-disk-type", "t", "HDD", "ProfitBricks Volume type")

	//Mount parameters
	args.metadataPath = flag.String("metadata-path", defaultBaseMetadataPath, "the path under which to store volume metadata")
	args.mountPath = flag.StringP("mount-path", "m", defaultBaseMountPath, "the path under which to create the volume mount folders")
	args.unixSocketGroup = flag.StringP("unix-socket-group", "g", defaultUnixSocketGroup, "the group to assign to the Unix socket file")

	//Other parameters
	args.version = flag.BoolP("version", "v", false, "outputs the driver version and exits")
	args.logLevel = flag.StringP("log-level", "l", "error", "log level")
	flag.Parse()

	if *args.version {
		fmt.Printf("%v\n", driverVersion)
		os.Exit(0)
	}

	//Try to get values from the environment variables
	if os.Getenv("PROFITBRICKS_ENDPOINT") != "" {
		*args.profitbricksEndpoint = os.Getenv("PROFITBRICKS_ENDPOINT")
	}
	if os.Getenv("PROFITBRICKS_USERNAME") != "" {
		*args.profitbricksUsername = os.Getenv("PROFITBRICKS_USERNAME")
	}
	if os.Getenv("PROFITBRICKS_PASSWORD") != "" {
		*args.profitbricksPassword = os.Getenv("PROFITBRICKS_PASSWORD")
	}

	//Try to get value from the config file
	if *args.profitbricksUsername == "" && *args.credentialFilePath != "" {
		*args.profitbricksUsername, err = mountUtil.GetConfValS(*args.credentialFilePath, "username")
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
	}

	if *args.profitbricksPassword == "" && *args.credentialFilePath != "" {
		*args.profitbricksPassword, err = mountUtil.GetConfValS(*args.credentialFilePath, "password")
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
	}

	//Final validation
	if *args.profitbricksUsername == "" {
		fmt.Println(fmt.Errorf("Username should be provided either using %q or using the environment variable %q", "--profitbricks-username", "PROFITBRICKS_USERNAME"))
		os.Exit(1)
	}

	if *args.profitbricksPassword == "" {
		fmt.Println(fmt.Errorf("Password should be provided either using %q or using the environment variables %q", "--profitbricks-password", "PROFITBRICKS_PASSWORD"))
		os.Exit(1)
	}

	if *args.datacenterID == "" {
		fmt.Println(fmt.Errorf("Please provide a Virtual Data Center ID %q or using the environment variable %q", "--profitbricks-datacenter-id [UUID]", "PROFITBRICKS_DATACENTER_ID"))
		os.Exit(1)
	}

	return args
}
