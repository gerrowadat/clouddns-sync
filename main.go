package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

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

func zoneDiffersFromCloud(e *zonefile.Entry, rr *dns.ResourceRecordSet) bool {
	// I'm not usually a fanboy of reading the rfc and naming
	// your data types accordingly, but jaysis.
	if string(e.Type()) != rr.Type ||
		string(e.Domain()) != rr.Name ||
		string(e.Class()) != rr.Kind {
		return true
	}
	// Compare e.Values to rr.Rrdatas
	// Also it's rrdatums, you clod.
	if len(e.Values()) != len(rr.Rrdatas) {
		return true
	}
	for _, ev := range e.Values() {
		found := false
		for _, rv := range rr.Rrdatas {
			if rv == string(ev) {
				found = true
			}
		}
		if !found {
			return true
		}
	}
	return false
}

func rrFromZoneEntry(e *zonefile.Entry) *dns.ResourceRecordSet {
	rrdatas := []string{}
	for _, e := range e.Values() {
		rrdatas = append(rrdatas, string(e))
	}

	ret := &dns.ResourceRecordSet{}
	ret.Kind = string(e.Class())
	ret.Name = string(e.Domain())
	// Fuck's sake.
	if e.TTL() != nil {
		ret.Ttl = int64(*e.TTL())
	}
	ret.Type = string(e.Type())
	ret.Rrdatas = rrdatas

	return ret
}

func uploadZonefile(dnsSpec *CloudDNSSpec, zoneFilename *string, dryRun *bool, pruneMissing *bool) error {
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

	to_add := []*dns.ResourceRecordSet{}
	to_modify := []*dns.ResourceRecordSet{}
	to_delete := []*dns.ResourceRecordSet{}

	rrs, err := getResourceRecordSetsForZone(dnsSpec)
	if err != nil {
		log.Fatal("Getting RRs for zone:", dnsSpec.zone)
	}

	for _, e := range zf.Entries() {
		// Ignore SOA, gcloud looks after this.
		if bytes.Equal(e.Type(), []byte("SOA")) ||
			bytes.Equal(e.Type(), []byte("NS")) {
			log.Printf("Ignoring SOA/NS")
			continue
		}
		// Also ignore control entries (for now I guess?)
		if e.Command() != nil {
			log.Printf("Ignoring control entry: %s", e.Command())
			continue
		}
		for _, rr := range rrs {
			if string(e.Type()) == rr.Type && string(e.Domain()) == rr.Name {
				if zoneDiffersFromCloud(&e, rr) {
					to_modify = append(to_modify, rrFromZoneEntry(&e))
					continue
				}
			}

			if *pruneMissing {
				found := false
				for _, ee := range zf.Entries() {
					if string(ee.Type()) == rr.Type && string(ee.Domain()) == rr.Name {
						found = true
					}
				}
				if found {
					to_delete = append(to_delete, rr)
					continue
				}
			}
		}
		// Not found in Cloud DNS, set for addition
		to_add = append(to_add, rrFromZoneEntry(&e))
	}
	log.Printf("Adding %d entries to Cloud DNS", len(to_add))
	for _, a := range to_add {
		log.Printf(" - %s (%s) %s", a.Name, a.Type, strings.Join(a.Rrdatas, " "))
	}
	log.Printf("Modifying %d entries in Cloud DNS", len(to_modify))
	log.Printf("Removing %d entries from Cloud DNS", len(to_delete))
	return nil
}

func main() {
	var jsonKeyfile = flag.String("json-keyfile", "key.json", "json credentials file for Cloud DNS")
	var cloudProject = flag.String("cloud-project", "", "Google Cloud Project")
	var cloudZone = flag.String("cloud-dns-zone", "", "Cloud DNS zone to operate on")
	var zoneFilename = flag.String("zonefilename", "", "Local zone file to operate on")
	var dryRun = flag.Bool("dry-run", false, "Do not update Cloud DNS, print what would be done")
	var pruneMissing = flag.Bool("prune-mising", false, "on putzonefile, prune cloud dns entries not in zone file")
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
		svc:     dnsservice,
		project: cloudProject,
		zone:    cloudZone,
	}

	log.Print("Found zone in Cloud DNS")

	switch verb {
	case "getzonefile":
		dumpZonefile(dns_spec)
	case "putzonefile":
		uploadZonefile(dns_spec, zoneFilename, dryRun, pruneMissing)
	default:
		log.Fatal("Uknown verb: ", verb)
	}
}
