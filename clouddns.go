package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	zonefile "github.com/bwesterb/go-zonefile"
	"google.golang.org/api/dns/v1"
)

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
