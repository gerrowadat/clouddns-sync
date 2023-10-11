package main

import (
	"fmt"
	"strconv"
	"strings"

	"google.golang.org/api/dns/v1"
)

func getCloudManagedZones(dnsSpec *CloudDNSSpec) ([]*dns.ManagedZone, error) {
	nextPageToken := ""
	ret := []*dns.ManagedZone{}
	for {
		out, err := dnsSpec.svc.ManagedZones.List(*dnsSpec.project).PageToken(nextPageToken).Do()
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

func getResourceRecordSetsForZone(dnsSpec *CloudDNSSpec) ([]*dns.ResourceRecordSet, error) {
	nextPageToken := ""
	ret := []*dns.ResourceRecordSet{}

	for {
		call := dnsSpec.svc.ResourceRecordSets.List(*dnsSpec.project, *dnsSpec.zone)

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
	if rr.Type == "SOA" {
		// SOA
		ret = append(ret, fmt.Sprintf("%s IN %s %s", rr.Name, rr.Type, string(rr.Rrdatas[0])))
	} else {
		soa_str := string("")
		if int(rr.Ttl) != 0 {
			soa_str = fmt.Sprintf("%s ", strconv.Itoa(int(rr.Ttl)))
		}
		for i := range rr.Rrdatas {
			ret = append(ret, fmt.Sprintf("%s %sIN %s %s", rr.Name, soa_str, rr.Type, string(rr.Rrdatas[i])))
		}
	}
	return strings.Join(ret, "\n")
}