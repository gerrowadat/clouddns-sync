package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	google_oauth "golang.org/x/oauth2/google"
	"google.golang.org/api/dns/v1"
	"google.golang.org/api/option"
)

func getCloudManagedZones(dnsservice *dns.Service, project *string) ([]*dns.ManagedZone, error) {
	nextPageToken := ""
	ret := []*dns.ManagedZone{}
	for {
		out, err := dnsservice.ManagedZones.List(*project).PageToken(nextPageToken).Do()
		if err != nil {
			return ret, err
		}
		ret = append(ret, out.ManagedZones...)
		if out.NextPageToken == "" {
			break
		}
		nextPageToken = out.NextPageToken
	}
	return ret, nil
}

func getResourceRecordSetsForZone(dnsservice *dns.Service, project *string, zone *string) ([]*dns.ResourceRecordSet, error) {
	nextPageToken := ""
	ret := []*dns.ResourceRecordSet{}

	for {
		call := dnsservice.ResourceRecordSets.List(*project, *zone)

		if nextPageToken != "" {
			call = call.PageToken(nextPageToken)
		}

		out, err := call.Do()

		if err != nil {
			return ret, err
		}

		ret = append(ret, out.Rrsets...)

		if out.NextPageToken == "" {
			break
		}

		nextPageToken = out.NextPageToken
	}

	return ret, nil
}

func ZoneFileFragment(rr *dns.ResourceRecordSet) string {
	ret := []string{}
	if rr.Type != "SOA" {
		soa_str := string("")
		if int(rr.Ttl) != 0 {
			soa_str = fmt.Sprintf("%s ", strconv.Itoa(int(rr.Ttl)))
		}
		for i, _ := range rr.Rrdatas {
			ret = append(ret, fmt.Sprintf("%s %sIN %s %s", rr.Name, soa_str, rr.Type, string(rr.Rrdatas[i])))
		}
	} else {
		// SOA
		ret = append(ret, fmt.Sprintf("%s IN %s %s", rr.Name, rr.Type, string(rr.Rrdatas[0])))
	}
	return strings.Join(ret, "\n")
}

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
