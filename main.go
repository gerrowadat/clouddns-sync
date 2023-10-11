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

func dumpZonefile(rrs []*dns.ResourceRecordSet) {
	for _, rr := range rrs {
		fmt.Println(ZoneFileFragment(rr))
	}
}

func uploadZonefile(fn *string, dry_run *bool) error {
	return nil
}

func main() {
	var jsonKeyfile = flag.String("json-keyfile", "key.json", "json credentials file for Cloud DNS")
	var cloudProject = flag.String("cloud-project", "", "Google Cloud Project")
	var cloudZone = flag.String("cloud-dns-zone", "", "Cloud DNS zone to operate on")
	var zoneFilename = flag.String("zonefilename", "", "Local zone file to operate on")
	var dryRun = flag.Bool("dry-run", false, "Do not update Cloud DNs, print what would be done")
	flag.Parse()

	// Verb and flag verification
	if len(flag.Args()) != 1 {
		log.Fatal("No verb specified")
	}

	verb := flag.Args()[0]

	// These are required in all cases
	if *cloudProject == "" {
		log.Fatal("--cloud-project is required")
	}
	if *cloudZone == "" {
		log.Fatal("--cloud-dns-zone is required")
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

	rrs, err := getResourceRecordSetsForZone(dnsservice, cloudProject, cloudZone)

	if err != nil {
		log.Fatal("Cloud DNS Error: ", err.Error())
	}

	log.Print("Found zone in Cloud DNS")

	switch verb {
	case "getzonefile":
		dumpZonefile(rrs)
	case "putzonefile":
		uploadZonefile(zoneFilename, dryRun)
	default:
		log.Fatal("Uknown verb: ", verb)
	}

}
