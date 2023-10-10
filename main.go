package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	google_oauth "golang.org/x/oauth2/google"
	"google.golang.org/api/dns/v1"
	"google.golang.org/api/option"
)

func main() {
	var jsonKeyfile = flag.String("json-keyfile", "key.json", "json credentials file for Cloud DNS")
	var cloudProject = flag.String("cloud-project", "myproject", "Google Cloud Project")
	var cloudZone = flag.String("cloud-dns-zone", "myzone", "Cloud DNS zone to operate on")
	flag.Parse()

	if len(flag.Args()) != 1 {
		log.Fatal("No verb specified")
	}

	jsonData, ioerror := os.ReadFile(*jsonKeyfile)
	if ioerror != nil {
		log.Fatal(*jsonKeyfile, ioerror)
	}

	ctx := context.Background()

	creds, err := google_oauth.CredentialsFromJSON(ctx, jsonData, "https://www.googleapis.com/auth/cloud-platform")

	if err != nil {
		log.Fatal("Cloud DNS Error: ", err)
	}

	dnsservice, err := dns.NewService(ctx, option.WithCredentials(creds))

	if err != nil {
		log.Fatal("Cloud DNS Error: ", err)
	}

	zones, err := getCloudManagedZones(dnsservice, cloudZone)

	if err != nil {
		log.Fatal("Cloud DNS Error: ", err.Error())
	}

	if len(zones) != 1 {
		log.Fatal("Zone not found: ", cloudZone)
	}

	zoneName := &zones[0].Name
	//zoneDomain := zones[0].DnsName

	rrs, err := getResourceRecordSetsForZone(dnsservice, cloudProject, zoneName)

	if err != nil {
		log.Fatal("Cloud DNS Error: ", err.Error())
	}

	for _, rr := range rrs {
		fmt.Println(ZoneFileFragment(rr))
	}
}
