package main

import (
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

func rrsetsDiffer(x *dns.ResourceRecordSet, y *dns.ResourceRecordSet) bool {
	if x.Type != y.Type ||
		x.Name != y.Name {
		return true
	}

	if len(x.Rrdatas) != len(y.Rrdatas) {
		return true
	}

	for _, xv := range x.Rrdatas {
		found := false
		for _, yv := range y.Rrdatas {
			if xv == yv {
				found = true
			}
		}
		if !found {
			fmt.Printf("%s not found in %s", xv, y.Name)
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

func mergeZoneEntryIntoRrsets(dnsSpec *CloudDNSSpec, rrs []*dns.ResourceRecordSet, e zonefile.Entry) []*dns.ResourceRecordSet {
	// Ignore control entries
	if e.Command() != nil {
		return rrs
	}

	// fully qualify the name, as this is what cloud dns does.
	e_fqdn := addDomainForZone(string(e.Domain()), *dnsSpec.domain)
	if string(e.Domain()) == "@" {
		// Technically it's whatever $ORIGIN is set to but whatevsies it's only DNS.
		e_fqdn = *dnsSpec.domain
	}

	// Create a new rrset if this is the first sighting of this name,
	// otherwise add the new rr to the existing one.
	found := false
	this_rrset := &dns.ResourceRecordSet{}
	for _, rr := range rrs {
		if rr.Name == e_fqdn && rr.Type != "SOA" {
			found = true
			this_rrset = rr
		}
	}
	if !found {
		rrs = append(rrs, this_rrset)
	}

	// Now, populate the rrset, either adding or appending the new answer.
	for _, rd := range e.Values() {
		if string(e.Type()) == "CNAME" {
			// for 'naked' cnames, add the domain to the rrdata
			this_rrset.Rrdatas = append(this_rrset.Rrdatas, addDomainForZone(string(rd), *dnsSpec.domain))
		} else {
			this_rrset.Rrdatas = append(this_rrset.Rrdatas, string(rd))
		}
	}
	this_rrset.Kind = string(e.Class())
	this_rrset.Name = e_fqdn
	// Fuck's sake.
	if e.TTL() != nil {
		this_rrset.Ttl = int64(*e.TTL())
	}
	this_rrset.Type = string(e.Type())

	return rrs
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

	// The format go-zonefile uses to represent RRs gives me hives.
	// Convert the go-zonefile format to a list of *dns.ResourceRecordSet
	zone_rrs := []*dns.ResourceRecordSet{}

	for _, e := range zf.Entries() {
		zone_rrs = mergeZoneEntryIntoRrsets(dnsSpec, zone_rrs, e)
	}

	log.Printf("Processing %d zonefile entries rendered %d rrsets", len(zf.Entries()), len(zone_rrs))

	change := dns.Change{}

	cloud_rrs, err := getResourceRecordSetsForZone(dnsSpec)
	if err != nil {
		log.Fatal("Getting RRs for zone:", dnsSpec.zone)
	}

	for _, z := range zone_rrs {
		// Ignore SOA, gcloud looks after this.
		if z.Type == "SOA" || z.Type == "NS" {
			log.Printf("Ignoring SOA/NS record")
			continue
		}

		found := false
		for _, c := range cloud_rrs {
			if z.Type == c.Type && z.Name == c.Name {
				found = true
				if rrsetsDiffer(z, c) {
					// Modify means a delete of the exact old record plus
					// addition of the new one.
					change.Additions = append(change.Additions, z)
					change.Deletions = append(change.Deletions, c)
					break
				}
			}
		}
		if !found {
			// Not found in Cloud DNS, set for addition
			change.Additions = append(change.Additions, z)
		}

		// TODO: Implement --prune-missing
	}
	if *pruneMissing {
		for _, c := range cloud_rrs {
			found := false
			if c.Type == "SOA" || c.Type == "NS" {
				continue
			}
			for _, z := range zone_rrs {
				if c.Name == z.Name && c.Type == z.Type {
					found = true
				}
			}
			if !found {
				// Missing from zone file, prune from cloud.
				change.Deletions = append(change.Deletions, c)
			}
		}

	}
	log.Printf("Adding %d entries to Cloud DNS", len(change.Additions))
	for _, a := range change.Additions {
		log.Printf(" + %s (%s) %s", a.Name, a.Type, strings.Join(a.Rrdatas, " "))
	}
	log.Printf("Removing %d entries from Cloud DNS", len(change.Deletions))
	for _, a := range change.Deletions {
		log.Printf(" - %s (%s) %s", a.Name, a.Type, strings.Join(a.Rrdatas, " "))
	}
	if len(change.Additions) == 0 && len(change.Deletions) == 0 {
		log.Printf("No Changes to do")
		return nil
	}

	return processCloudDnsChange(dnsSpec, &change)
}
