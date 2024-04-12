package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	google_oauth "golang.org/x/oauth2/google"
	"google.golang.org/api/dns/v1"
	"google.golang.org/api/option"
)

var (
	dnsChangesProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "dns_changes_processed_total",
		Help: "The total number of DNS changes processed",
	})
)

type CloudDNSSpec struct {
	svc         *dns.Service
	project     *string
	zone        *string
	domain      *string
	default_ttl *int
	dry_run     *bool
}

func getMyIP() (string, error) {
	res, err := http.Get("http://whatismyip.akamai.com")
	if err != nil {
		log.Fatal("HTTP Error getting our IP: ", err)
	}

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		log.Fatal("Error reading response body: ", err)
	}

	ip := net.ParseIP(string(resBody))
	if ip == nil {
		log.Fatalf("Non-IP returned from request: %s", ip)
	}

	return string(resBody), nil
}

func main() {
	var jsonKeyfile = flag.String("json-keyfile", "", "json credentials file for Cloud DNS")
	var cloudProject = flag.String("cloud-project", "", "Google Cloud Project")
	var cloudZone = flag.String("cloud-dns-zone", "", "Cloud DNS zone to operate on")
	var defaultCloudTtl = flag.Int("cloud-dns-default-ttl", 300, "Default TTL for Cloud DNS records")
	var dryRun = flag.Bool("dry-run", false, "Do not update Cloud DNS, print what would be done")
	var pruneMissing = flag.Bool("prune-missing", false, "on putzonefile, prune cloud dns entries not in zone file")

	// For [get|put]zonefile
	var zoneFilename = flag.String("zonefilename", "", "Local zone file to operate on")

	// for nomad_sync
	var nomadServerURI = flag.String("nomad-server-uri", "http://localhost:4646", "URI for a nomad server to talk to.")
	var nomadTokenFile = flag.String("nomad-token-file", "", "file to read ou rnomad token from")
	var nomadSyncInterval = flag.Int("nomad-sync-interval-secs", 300, "seconds between nomad updates. set to -1 to sync once only.")
	var httpPort = flag.Int("http-port", 8080, "Port to listen on for /metrics")

	// for dynrecord
	var cloudDnsDynRecordName = flag.String("cloud-dns-dyn-record-name", "", "Cloud DNS record to update with our IP")

	flag.Parse()

	// Verb and flag verification
	if len(flag.Args()) != 1 {
		log.Fatal("No verb specified")
	}

	verb := flag.Args()[0]

	// Required in all cases
	if *cloudZone == "" {
		log.Fatal("--cloud-dns-zone is required")
	}

	if verb == "putzonefile" {
		if *zoneFilename == "" {
			log.Fatal("--zonefilename is required for putzonefile")
		}
	}

	if verb == "dynrecord" {
		if *cloudDnsDynRecordName == "" {
			log.Fatal("--cloud-dns-dyn-record-name is required for dynrecord")
		}
	}

	ctx := context.Background()
	creds := &google_oauth.Credentials{}

	if *jsonKeyfile != "" {
		jsonData, ioerror := os.ReadFile(*jsonKeyfile)
		if ioerror != nil {
			log.Fatal(*jsonKeyfile, ioerror)
		}
		creds, _ = google_oauth.CredentialsFromJSON(ctx, jsonData, "https://www.googleapis.com/auth/cloud-platform")
	} else {
		creds, _ = google_oauth.FindDefaultCredentials(ctx)
	}

	// Get project from json keyfile if present.
	if creds.ProjectID != "" {
		*cloudProject = creds.ProjectID
	}

	if *cloudProject == "" {
		log.Fatal("--cloud-project is required if not defined in json credentials")
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

	switch verb {
	case "getzonefile":
		dumpZonefile(dns_spec)
	case "putzonefile":
		uploadZonefile(dns_spec, zoneFilename, dryRun, pruneMissing)
	case "dynrecord":
		my_ip, err := getMyIP()
		if err != nil {
			log.Fatalf("Error getting our IP: %s", err)
		}

		log.Printf("Detected IP: %s", my_ip)

		current_dns, err := net.LookupIP(*cloudDnsDynRecordName)

		current_ip := ""

		if err != nil {
			log.Print("Error in DNS resolution: ", err)
			log.Print("Continuing...")
		} else {
			if len(current_dns) > 1 {
				log.Fatalf("%s resolves to multiple IPs. Weird.", *cloudDnsDynRecordName)
			}

			current_ip = current_dns[0].To4().String()

			if my_ip == current_ip {
				log.Print("My IP matches DNS. Nothing to do.")
				return
			}
		}

		err = updateOneARecord(dns_spec, *cloudDnsDynRecordName, current_ip, my_ip)

		if err != nil {
			log.Fatalf("Error Updating GCloud: %s", err)
		}
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

		http.Handle("/metrics", promhttp.Handler())

		go periodicallySyncNomad(dns_spec, nomadSpec, *nomadSyncInterval, pruneMissing)

		log.Fatal(http.ListenAndServe(":"+fmt.Sprintf("%v", *httpPort), nil))

	default:
		log.Fatal("Unknown verb: ", verb)
	}
}
