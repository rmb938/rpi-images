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
	"github.com/diskfs/go-diskfs/disk"
	"github.com/diskfs/go-diskfs/filesystem"
	"github.com/diskfs/go-diskfs/partition/mbr"
	"github.com/google/uuid"
	"gopkg.in/yaml.v2"
)

type MetaData struct {
	Image      string            `yaml:"image"`
	Hostname   string            `yaml:"hostname"`
	PublicKeys map[string]string `yaml:"publicKeys"`
	Network    struct {
		MAC         string   `yaml:"mac"`
		IPAddress   string   `yaml:"ipAddress"`
		Netmask     string   `yaml:"netmask"`
		Gateway     string   `yaml:"gateway"`
		Nameservers []string `yaml:"nameservers"`
		Search      []string `yaml:"search"`
	} `yaml:"network"`
	UserData string `yaml:"userData"`
}

func main() {
	var configPath string
	var diskPath string

	flag.StringVar(&configPath, "config", "", "The path to the configuration")
	flag.StringVar(&diskPath, "disk", "", "The path to the disk to write the image to")

	flag.Parse()

	if len(configPath) == 0 {
		log.Fatalf("config flag must be given")
	}

	log.Printf("Opening configuration %s", configPath)
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
	cloudInitPartitionNumber := -1

	if rawTable.Type() != "mbr" {
		log.Fatalf("GPT partition tables are not supported")
	}

	table := rawTable.(*mbr.Table)
	cloudInitSize := 64 * 1024 * 1024 // 64 MB
	cloudInitSectors := uint32(cloudInitSize / table.LogicalSectorSize)
	// we want to create it at the end of the disk
	// so find the disk sector count and minus the cloudinit sectors
	cloudInitStart := uint32(int(destDisk.Size)/table.LogicalSectorSize) - cloudInitSectors

	partitions := make([]*mbr.Partition, 0)
	for _, part := range table.Partitions {
		if part.Type == mbr.Empty {
			continue
		}
		partitions = append(partitions, part)
	}

	if len(partitions) >= 4 {
		log.Fatalf("partition table already has 4 partitions, there is no room for cloud-init on disk %s", diskPath)
	}

	// add cloud-init partition
	table.Partitions = append(partitions, &mbr.Partition{
		Bootable: false,
		Type:     mbr.Linux,
		Start:    cloudInitStart,
		Size:     cloudInitSectors,
	})
	cloudInitPartitionNumber = len(table.Partitions)

	// write partition table to disk
	log.Printf("Writing partition table to disk")
	err = destDisk.Partition(table)
	if err != nil {
		log.Fatalf("error writing partition table to disk %s: %v", diskPath, err)
	}

	log.Printf("Creating cloud init filesystem")
	cloudInitFS, err := destDisk.CreateFilesystem(disk.FilesystemSpec{
		Partition:   cloudInitPartitionNumber,
		FSType:      filesystem.TypeFat32,
		VolumeLabel: "config-2",
	})
	if err != nil {
		log.Fatalf("error creating cloud-init filesystem on disk %s: %v", diskPath, err)
	}

	cloudInitPrefix := path.Join("/", "openstack", "latest")
	log.Printf("Creating cloud init directory structure")
	err = cloudInitFS.Mkdir(cloudInitPrefix)
	if err != nil {
		log.Fatalf("error creating cloud-init directory structure %v", err)
	}

	metadataPath := path.Join(cloudInitPrefix, "meta_data.json")
	log.Printf("Opening %s", metadataPath)
	metadataFile, err := cloudInitFS.OpenFile(metadataPath, os.O_CREATE|os.O_RDWR)
	if err != nil {
		log.Fatalf("Error opening meta data: %v", err)
	}
	uid, err := uuid.NewUUID()
	if err != nil {
		log.Fatalf("Error generating metadata uuid %v", err)
	}
	metadataContents := map[string]interface{}{
		"uuid":        uid.String(),
		"public_keys": metadataConfig.PublicKeys,
		"hostname":    metadataConfig.Hostname,
	}
	data, err := json.MarshalIndent(&metadataContents, "", "\t")
	log.Printf("Writing metadata contents: \n%v", string(data))
	_, err = metadataFile.Write(data)
	if err != nil {
		log.Fatalf("Error writting meta data: %v", err)
	}

	networkdataPath := path.Join(cloudInitPrefix, "network_data.json")
	log.Printf("Opening %s", networkdataPath)
	networkdataFile, err := cloudInitFS.OpenFile(networkdataPath, os.O_CREATE|os.O_RDWR)
	if err != nil {
		log.Fatalf("Error opening network data: %v", err)
	}
	networkdataContents := map[string]interface{}{
		"links": []map[string]string{
			{
				"id":                   "eth0",
				"ethernet_mac_address": metadataConfig.Network.MAC,
				"type":                 "phy",
			},
		},
		"networks": []map[string]interface{}{
			{
				"link":            "eth0",
				"type":            "ipv4",
				"ip_address":      metadataConfig.Network.IPAddress,
				"netmask":         metadataConfig.Network.Netmask,
				"gateway":         metadataConfig.Network.Gateway,
				"dns_nameservers": metadataConfig.Network.Nameservers,
				"dns_search":      metadataConfig.Network.Search,
			},
		},
	}
	data, err = json.MarshalIndent(&networkdataContents, "", "\t")
	log.Printf("Writing networkdata contents: \n%v", string(data))
	_, err = networkdataFile.Write(data)
	if err != nil {
		log.Fatalf("Error writting network data: %v", err)
	}

	userdataPath := path.Join(cloudInitPrefix, "user_data")
	log.Printf("Opening %s", userdataPath)
	userdataFile, err := cloudInitFS.OpenFile(userdataPath, os.O_CREATE|os.O_RDWR)
	if err != nil {
		log.Fatalf("Error opening user data: %v", err)
	}
	log.Printf("Writing userdata contents: \n%v", metadataConfig.UserData)
	_, err = userdataFile.Write([]byte(metadataConfig.UserData))
	if err != nil {
		log.Fatalf("Error writting user data: %v", err)
	}

	err = destDisk.File.Close()
	if err != nil {
		log.Fatalf("Error closing disk %s: %v", diskPath, err)
	}
}
