package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	zonefile "github.com/bwesterb/go-zonefile"
	google_oauth "golang.org/x/oauth2/google"
	"google.golang.org/api/dns/v1"
	"google.golang.org/api/option"
)

type CloudDNSSpec struct {
	svc     *dns.Service
	project *string
	zone    *string
}

func dumpZonefile(dnsSpec *CloudDNSSpec) {

	rrs, err := getResourceRecordSetsForZone(dnsSpec)
	if err != nil {
		log.Fatal("Getting RRs for zone:", dnsSpec.zone)
	}
	for _, rr := range rrs {
		fmt.Println(ZoneFileFragment(rr))
	}
}

func uploadZonefile(dnsSpec *CloudDNSSpec, zoneFilename *string, dryRun *bool) error {
	data, err := os.ReadFile(*zoneFilename)
	if err != nil {
		log.Print("Error opening zonefile: ", zoneFilename)
		return err
	}

	zf, err := zonefile.Load(data)
	if err != nil {
		log.Print("Error parsing zonefile: ", err)
		return err
	}

	for _, e := range zf.Entries() {
		// Ignore SOA, gcloud looks after this.
		if !bytes.Equal(e.Type(), []byte("SOA")) {
			continue
		}

	}
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

	dns_spec := &CloudDNSSpec{
		svc:     dnsservice,
		project: cloudProject,
		zone:    cloudZone,
	}

	log.Print("Found zone in Cloud DNS")

	switch verb {
	case "getzonefile":
		dumpZonefile(dns_spec)
	case "putzonefile":
		uploadZonefile(dns_spec, zoneFilename, dryRun)
	default:
		log.Fatal("Uknown verb: ", verb)
	}
}
