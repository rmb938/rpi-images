package main

import (
	"encoding/json"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"

	"github.com/diskfs/go-diskfs"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type MetaData struct {
	Image         string                 `yaml:"image"`
	Hostname      string                 `yaml:"hostname"`
	NetworkConfig map[string]interface{} `yaml:"network_config"`
}

func main() {
	var hostDirectory string
	var diskPath string

	flag.StringVar(&hostDirectory, "host", "", "The host directory")
	flag.StringVar(&diskPath, "disk", "", "The path to the disk to write the image to")

	flag.Parse()

	logrus.SetLevel(logrus.DebugLevel)

	if len(hostDirectory) == 0 {
		log.Fatalf("host flag must be given")
	}

	hostDirectoryInfo, err := os.Stat(hostDirectory)
	if err != nil {
		log.Fatalf("Error checking host directory %s: %v", hostDirectory, err)
	}
	if hostDirectoryInfo.Mode().IsDir() == false {
		log.Fatalf("Given host %s is not a directory", hostDirectory)
	}

	configPath := path.Join(hostDirectory, "metadata.yaml")

	log.Printf("Opening metadata %s", configPath)
	configFile, err := os.Open(configPath)
	if err != nil {
		log.Fatalf("Error opening configuration %s: %v", configPath, err)
	}
	defer configFile.Close()

	configBytes, err := ioutil.ReadAll(configFile)
	if err != nil {
		log.Fatalf("Error reading configuration %s: %v", configPath, err)
	}

	metadataConfig := &MetaData{}
	err = yaml.Unmarshal(configBytes, metadataConfig)
	if err != nil {
		log.Fatalf("Error parsing configuration %s: %v", configPath, err)
	}

	if len(metadataConfig.Image) == 0 {
		log.Fatalf("image must be set in configuration")
	}

	if len(diskPath) == 0 {
		log.Fatalf("disk flag must be given")
	}

	userDataPath := path.Join(hostDirectory, "user_data")
	log.Printf("Opening user data")
	userDataFile, err := os.Open(userDataPath)
	if err != nil {
		log.Fatalf("Error opening user data %s: %v", userDataPath, err)
	}
	defer userDataFile.Close()
	userDataBytes, err := ioutil.ReadAll(userDataFile)
	if err != nil {
		log.Fatalf("Error reading user data %s: %v", userDataPath, err)
	}

	log.Printf("Opening image %s", metadataConfig.Image)
	imageFile, err := os.Open(metadataConfig.Image)
	if err != nil {
		log.Fatalf("Error opening image %s: %v", metadataConfig.Image, err)
	}

	log.Printf("Opening disk %s", diskPath)
	diskFile, err := os.OpenFile(diskPath, os.O_RDWR|os.O_EXCL, 0600)
	if err != nil {
		log.Fatalf("Error opening disk %s: %v", diskPath, err)
	}

	log.Print("Writing image to disk")
	_, err = io.Copy(diskFile, imageFile)
	if err != nil {
		log.Fatalf("Error writing image to disk %s: %v", diskPath, err)
	}
	err = imageFile.Close()
	if err != nil {
		log.Fatalf("Error closing image %s: %v", metadataConfig.Image, err)
	}
	err = diskFile.Close()
	if err != nil {
		log.Fatalf("Error closing disk %s: %v", diskPath, err)
	}

	log.Printf("Reading disk partitions %s", diskPath)
	destDisk, err := diskfs.Open(diskPath)
	if err != nil {
		log.Fatalf("Error opening disk %s: %v", diskPath, err)
	}

	rawTable, err := destDisk.GetPartitionTable()
	if err != nil {
		log.Fatalf("Error getting partition table for disk %s: %v", diskPath, err)
	}

	log.Printf("Found partition table %s", rawTable.Type())

	if rawTable.Type() != "mbr" {
		log.Fatalf("GPT partition tables are not supported")
	}

	for _, part := range rawTable.GetPartitions() {
		log.Printf("%#vn", part)
	}

	// system-boot filesystem is always the first one
	systemBootFs, err := destDisk.GetFilesystem(1)
	if err != nil {
		log.Fatalf("Error getting filesystem for partition %d: %v", 0, err)
	}

	metadataPath := path.Join("/", "meta-data")
	log.Printf("Opening %s", metadataPath)
	metadataFile, err := systemBootFs.OpenFile(metadataPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC)
	if err != nil {
		log.Fatalf("Error opening meta data: %v", err)
	}
	uid, err := uuid.NewUUID()
	if err != nil {
		log.Fatalf("Error generating metadata uuid %v", err)
	}
	metadataContents := map[string]interface{}{
		"instance_id":    uid.String(),
		"local-hostname": metadataConfig.Hostname,
		"hostname":       metadataConfig.Hostname,
	}
	data, err := json.MarshalIndent(&metadataContents, "", "\t")
	log.Printf("Writing metadata contents: \n%v", string(data))
	_, err = metadataFile.Write(data)
	if err != nil {
		log.Fatalf("Error writting meta data: %v", err)
	}

	networkConfigPath := path.Join("/", "network-config")
	log.Printf("Opening %s", networkConfigPath)
	networkConfigFile, err := systemBootFs.OpenFile(networkConfigPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC)
	if err != nil {
		log.Fatalf("Error opening network config: %v", err)
	}

	networkConfigContents, err := yaml.Marshal(metadataConfig.NetworkConfig)
	if err != nil {
		log.Fatalf("Error marshalling network config: %v", err)
	}

	log.Printf("Writing network config contents: \n%v", string(networkConfigContents))
	_, err = networkConfigFile.Write(networkConfigContents)
	if err != nil {
		log.Fatalf("Error writting network config: %v", err)
	}

	userdataPath := path.Join("/", "user-data")
	log.Printf("Opening %s", userdataPath)
	userdataFile, err := systemBootFs.OpenFile(userdataPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC)
	if err != nil {
		log.Fatalf("Error opening user data: %v", err)
	}
	log.Printf("Writing userdata contents: \n%v", string(userDataBytes))
	_, err = userdataFile.Write(userDataBytes)
	if err != nil {
		log.Fatalf("Error writting user data: %v", err)
	}

	err = destDisk.File.Close()
	if err != nil {
		log.Fatalf("Error closing disk %s: %v", diskPath, err)
	}
}
