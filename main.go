package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	google_oauth "golang.org/x/oauth2/google"
	"google.golang.org/api/dns/v1"
	"google.golang.org/api/option"
)

type CloudDNSSpec struct {
	svc         *dns.Service
	project     *string
	zone        *string
	domain      *string
	default_ttl *int
	dry_run     *bool
}

func main() {
	var jsonKeyfile = flag.String("json-keyfile", "key.json", "json credentials file for Cloud DNS")
	var cloudProject = flag.String("cloud-project", "", "Google Cloud Project")
	var cloudZone = flag.String("cloud-dns-zone", "", "Cloud DNS zone to operate on")
	var defaultCloudTtl = flag.Int("cloud-dns-default-ttl", 300, "Default TTL for Cloud DNS records")

	var zoneFilename = flag.String("zonefilename", "", "Local zone file to operate on")
	var dryRun = flag.Bool("dry-run", false, "Do not update Cloud DNS, print what would be done")
	var pruneMissing = flag.Bool("prune-missing", false, "on putzonefile, prune cloud dns entries not in zone file")

	var nomadServerURI = flag.String("nomad-server-uri", "http://localhost:4646", "URI for a nomad server to talk to.")
	var nomadTokenFile = flag.String("nomad-token-file", "", "file to read ou rnomad token from")
	var nomadSyncInterval = flag.Int("nomad-sync-interval-secs", 300, "seconds between nomad updates. set to -1 to sync once only.")

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

	if verb == "putzonefile" {
		if *zoneFilename == "" {
			log.Fatal("--zonefilename is required for putzonefile")
		}
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
		svc:         dnsservice,
		project:     cloudProject,
		zone:        cloudZone,
		default_ttl: defaultCloudTtl,
		dry_run:     dryRun,
	}

	err = populateDnsSpec(dns_spec)
	if err != nil {
		log.Fatal(err)
	}

	log.Print("Found zone in Cloud DNS")

	switch verb {
	case "getzonefile":
		dumpZonefile(dns_spec)
	case "putzonefile":
		uploadZonefile(dns_spec, zoneFilename, dryRun, pruneMissing)
	case "nomad_sync":
		nomadSpec := &NomadSpec{
			uri: *nomadServerURI,
		}
		if *nomadTokenFile != "" {
			nomadToken, err := os.ReadFile(*nomadTokenFile)
			if err != nil {
				log.Fatal("Reading Nomad Token: ", err)
			}
			nomadSpec.token = string(nomadToken)
		} else {
			nomadSpec.token = ""
		}

		syncNomad(dns_spec, nomadSpec, dryRun, pruneMissing)

		if *nomadSyncInterval >= 0 {
			for {
				log.Printf("Waiting %d seconds.", *nomadSyncInterval)
				time.Sleep(time.Duration(*nomadSyncInterval) * time.Second)
				syncNomad(dns_spec, nomadSpec, dryRun, pruneMissing)
			}
		}
	default:
		log.Fatal("Unknown verb: ", verb)
	}
}
