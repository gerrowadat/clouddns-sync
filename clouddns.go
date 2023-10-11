package main

import (
	"bytes"
	"errors"
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

func zoneDiffersFromCloud(e *zonefile.Entry, rr *dns.ResourceRecordSet, dnsDomain *string) bool {
	// I'm not usually a fanboy of reading the rfc and naming
	// your data types accordingly, but jaysis.

	if string(e.Type()) != rr.Type ||
		string(e.Domain()) != rr.Name {
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
			fmt.Printf("%s not found in %s", string(ev), rr.Name)
			return true
		}
	}
	return false
}

func addDomainForZone(name string, domain string) string {
	if !strings.HasSuffix(name, ".") {
		return name + "." + domain
	}
	return name
}

func rrFromZoneEntry(dnsSpec *CloudDNSSpec, e *zonefile.Entry) *dns.ResourceRecordSet {
	rrdatas := []string{}
	for _, rd := range e.Values() {
		if string(e.Type()) == "CNAME" {
			// for 'naked' cnames, add the domain to the rrdata
			rrdatas = append(rrdatas, addDomainForZone(string(rd), *dnsSpec.domain))
		} else {
			rrdatas = append(rrdatas, string(rd))
		}
	}

	ret := &dns.ResourceRecordSet{}
	ret.Kind = string(e.Class())
	ret.Name = addDomainForZone(string(e.Domain()), *dnsSpec.domain)
	// Fuck's sake.
	if e.TTL() != nil {
		ret.Ttl = int64(*e.TTL())
	}
	ret.Type = string(e.Type())
	ret.Rrdatas = rrdatas

	return ret
}

func processCloudDnsChange(dnsSpec *CloudDNSSpec, dnsChange *dns.Change) error {
	call := dnsSpec.svc.Changes.Create(*dnsSpec.project, *dnsSpec.zone, dnsChange)
	out, err := call.Do()
	if err != nil {
		log.Printf("Error updating Cloud DNS: %s", err)
	} else {
		log.Printf("Added [%d] and deleted [%d] records.",
			len(out.Additions), len(out.Deletions))
	}
	return err
}

func populateDnsSpec(dnsSpec *CloudDNSSpec) error {
	// This just populates 'domain' at the moment.
	if dnsSpec.domain != nil {
		return nil
	}
	call := dnsSpec.svc.ManagedZones.List(*dnsSpec.project)
	out, err := call.Do()
	if err != nil {
		log.Printf("Error Getting zones for project %s: %s", *dnsSpec.project, err)
		return err
	} else {
		for _, m := range out.ManagedZones {
			if m.Name == *dnsSpec.zone {
				dnsSpec.domain = &m.DnsName
				return nil
			}
		}
	}
	errmsg := fmt.Sprintf("Managed zone not found in project %s: %s", *dnsSpec.project, *dnsSpec.zone)
	return errors.New(errmsg)
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

	change := dns.Change{}

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

		found := false
		for _, rr := range rrs {
			if string(e.Type()) == rr.Type && string(e.Domain()) == rr.Name {
				found = true
				if zoneDiffersFromCloud(&e, rr, dnsSpec.domain) {
					// Modify means a delete of the exact old record plus
					// addition of the new one.
					change.Additions = append(change.Additions, rrFromZoneEntry(dnsSpec, &e))
					change.Deletions = append(change.Deletions, rr)
					break
				}
			}
		}
		if !found {
			// Not found in Cloud DNS, set for addition
			change.Additions = append(change.Additions, rrFromZoneEntry(dnsSpec, &e))
		}

		// TODO: Implement --prune-missing
	}
	log.Printf("Adding %d entries to Cloud DNS", len(change.Additions))
	for _, a := range change.Additions {
		log.Printf(" - %s (%s) %s", a.Name, a.Type, strings.Join(a.Rrdatas, " "))
	}
	log.Printf("Removing %d entries from Cloud DNS", len(change.Deletions))

	if len(change.Additions) == 0 && len(change.Deletions) == 0 {
		log.Printf("No Changes to do")
		return nil
	}

	return processCloudDnsChange(dnsSpec, &change)
}
