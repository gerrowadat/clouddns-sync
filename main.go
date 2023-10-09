package main

import (
				"os"
				"context"
				"google.golang.org/api/option"
				"fmt"
				"io/ioutil"
				"flag"
				google_oauth "golang.org/x/oauth2/google"
				"google.golang.org/api/dns/v1"
			)


func getCloudManagedZones(dnsservice *dns.Service, project string) ([]*dns.ManagedZone, error) {
	nextPageToken := ""
	ret := []*dns.ManagedZone{}
	for {
		out, err := dnsservice.ManagedZones.List(project).PageToken(nextPageToken).Do()
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


func main() {
	var jsonKeyfile = flag.String("json-keyfile", "key.json", "json credentials file for Cloud DNS")
	var cloudProject = flag.String("cloud-project", "myproject", "Google Cloud Project")
	var cloudZone = flag.String("cloud-dns-zone", "myzone", "Cloud DNS zone to operate on")
	flag.Parse()

	jsonData, ioerror := ioutil.ReadFile(*jsonKeyfile)
	if ioerror != nil {
		fmt.Println(*jsonKeyfile, ioerror)
		os.Exit(1)
	}

	ctx := context.Background()

	creds, err := google_oauth.CredentialsFromJSON(ctx, jsonData, "https://www.googleapis.com/auth/cloud-platform")

	if err != nil {
		fmt.Println("Cloud DNS Error: ", err)
		os.Exit(1)
	}

	dnsservice, err := dns.NewService(ctx, option.WithCredentials(creds))

	if err != nil {
		fmt.Println("Cloud DNS Error: ", err)
		os.Exit(1)
	}

	zones, err := getCloudManagedZones(dnsservice, "awaylab")

	if err != nil {
		fmt.Println("Cloud DNS Error: ", err.Error())
	}

	for _, z := range zones {
		fmt.Println(z.Name, ": ", z.DnsName)
	}

	rrs, err := getResourceRecordSetsForZone(dnsservice, cloudProject, cloudZone)

	if err != nil {
		fmt.Println("Cloud DNS Error: ", err.Error())
	}

	for _, z := range rrs {
		fmt.Println(z.Name, ": ", z.Type)
	}

}

