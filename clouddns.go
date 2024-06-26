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

	// Print in order of SOA, NS, A, MX CNAME, Other
	rtypes := []string{"SOA", "A", "MX", "CNAME"}
	for _, rt := range rtypes {
		for _, rr := range rrs {
			if rr.Type == rt {
				fmt.Println(ZoneFileFragment(rr))
			}
		}
	}
	// Other.
	for _, rr := range rrs {
		othertype := true
		for _, rt := range rtypes {
			if rr.Type == rt {
				othertype = false
			}
		}

		if othertype {
			fmt.Println(ZoneFileFragment(rr))
		}
	}

}

func rrsetsEqual(x *dns.ResourceRecordSet, y *dns.ResourceRecordSet) bool {
	if x.Type != y.Type ||
		x.Name != y.Name ||
		x.Ttl != y.Ttl {
		return false
	}

	if len(x.Rrdatas) != len(y.Rrdatas) {
		return false
	}

	for _, xv := range x.Rrdatas {
		found := false
		for _, yv := range y.Rrdatas {
			if xv == yv {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func addDomainForZone(name string, domain string) string {
	if !strings.HasSuffix(name, ".") {
		return name + "." + domain
	}
	return name
}

func processCloudDnsChange(dnsSpec *CloudDNSSpec, dnsChange *dns.Change) error {
	if dnsChange == nil || (len(dnsChange.Additions) == 0 && len(dnsChange.Deletions) == 0) {
		log.Printf("No DNS changes for Cloud")
		return nil
	}

	log.Printf("Adding %d entries to Cloud DNS", len(dnsChange.Additions))
	for _, a := range dnsChange.Additions {
		log.Printf(" + %s (%s) %s", a.Name, a.Type, strings.Join(a.Rrdatas, " "))
	}
	log.Printf("Removing %d entries from Cloud DNS", len(dnsChange.Deletions))
	for _, a := range dnsChange.Deletions {
		log.Printf(" - %s (%s) %s", a.Name, a.Type, strings.Join(a.Rrdatas, " "))
	}

	if *dnsSpec.dry_run {
		log.Print("Running in dry run mode. Not actually updating Cloud DNS.")
		return nil
	}

	call := dnsSpec.svc.Changes.Create(*dnsSpec.project, *dnsSpec.zone, dnsChange)
	out, err := call.Do()
	if err != nil {
		log.Printf("Error updating Cloud DNS: %s", err)
	} else {
		log.Printf("Added [%d] and deleted [%d] records.",
			len(out.Additions), len(out.Deletions))
		dnsChangesProcessed.Inc()
	}
	return err
}

func buildNomadDnsChange(dnsSpec *CloudDNSSpec, tasks []TaskInfo, pruneMissing bool) (*dns.Change, error) {
	// Build a new TaskInfo with fully qualified dns names.
	fq_taskinfo := []TaskInfo{}
	for _, t := range tasks {
		fq_taskinfo = append(fq_taskinfo, TaskInfo{
			jobid: addDomainForZone(t.jobid, *dnsSpec.domain),
			ip:    t.ip,
		})
	}

	nomad_rrs, err := buildTaskInfoToRrsets(fq_taskinfo, dnsSpec.default_ttl)
	if err != nil {
		log.Print("Converting Nomad RRs for zone:", dnsSpec.zone)
		return nil, err
	}

	cloud_rrs, err := getResourceRecordSetsForZone(dnsSpec)
	if err != nil {
		log.Print("Getting Cloud DNS RRs for zone:", dnsSpec.zone)
		return nil, err
	}

	ret := buildDnsChange(cloud_rrs, nomad_rrs, pruneMissing)

	return ret, nil
}

func buildTaskInfoToRrsets(tasks []TaskInfo, default_ttl *int) ([]*dns.ResourceRecordSet, error) {
	// Take a set of TaskInfo (essentially name to IP) and return a slice of ResourceRecordSet
	// use default_ttl as the ttl of all records (nomad has no opinion on ttl).
	ret := []*dns.ResourceRecordSet{}

	for _, t := range tasks {
		ret = mergeAnswerToRrsets(ret, t.jobid, t.ip, *default_ttl)
	}
	return ret, nil
}

func mergeAnswerToRrsets(rrsets []*dns.ResourceRecordSet, name string, ip string, default_ttl int) []*dns.ResourceRecordSet {
	// merges an answer that point name to ip into these rrsets.
	// Only handles simple A records.
	for _, rr := range rrsets {
		if rr.Name == name {
			// Name already exists, append the additional IP (if it's not there already)
			ip_found := false
			for _, rrd := range rr.Rrdatas {
				if rrd == ip {
					ip_found = true
				}
			}
			if !ip_found {
				rr.Rrdatas = append(rr.Rrdatas, ip)
			}
			return rrsets
		}
	}
	// Not found, add new rrdata
	new_rr := &dns.ResourceRecordSet{
		Name: name,
		Type: "A",
		Ttl:  int64(default_ttl),
	}
	new_rr.Rrdatas = []string{ip}
	rrsets = append(rrsets, new_rr)
	return rrsets
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

	// Also ignore SOA/NS records, since these are managed by gcloud.
	if string(e.Type()) == "SOA" || string(e.Type()) == "NS" {
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
		if rr.Name == e_fqdn && rr.Type != "SOA" && rr.Type != "NS" {
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
	} else {
		this_rrset.Ttl = int64(*dnsSpec.default_ttl)
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

	cloud_rrs, err := getResourceRecordSetsForZone(dnsSpec)
	if err != nil {
		log.Fatal("Getting RRs for zone:", dnsSpec.zone)
	}

	change := buildDnsChange(cloud_rrs, zone_rrs, *pruneMissing)

	if *dryRun {
		log.Print("Running in dry run mode. Not actually updating Cloud DNS.")
	}

	return processCloudDnsChange(dnsSpec, change)
}

func buildDnsChange(cloud_rrs, zone_rrs []*dns.ResourceRecordSet, prune_missing bool) *dns.Change {

	ret := dns.Change{}

	for _, z := range zone_rrs {
		found := false
		for _, c := range cloud_rrs {
			if z.Type == c.Type && z.Name == c.Name {
				found = true
				if !rrsetsEqual(z, c) {
					// Modify means a delete of the exact old record plus
					// addition of the new one.
					ret.Additions = append(ret.Additions, z)
					ret.Deletions = append(ret.Deletions, c)
					break
				}
			}
		}
		if !found {
			// Not found in Cloud DNS, set for addition
			ret.Additions = append(ret.Additions, z)
		}
	}
	if prune_missing {
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
				ret.Deletions = append(ret.Deletions, c)
			}
		}

	}
	return &ret

}

func updateOneARecord(dns_spec *CloudDNSSpec, record_name string, old_ip, new_ip string) error {

	log.Printf("Updating Cloud DNS: %s : %s -> %s", record_name, old_ip, new_ip)

	change := &dns.Change{
		Additions: []*dns.ResourceRecordSet{
			{
				Name:    record_name,
				Type:    "A",
				Rrdatas: []string{new_ip},
				Ttl:     int64(*dns_spec.default_ttl),
			},
		},
	}

	// Gcloud DNS shits the bed if you try to delete a record that's not there.
	if old_ip != "" {
		new_rr := dns.ResourceRecordSet{
			Name:    record_name,
			Type:    "A",
			Rrdatas: []string{old_ip},
			Ttl:     int64(*dns_spec.default_ttl),
		}
		change.Deletions = append(change.Deletions, &new_rr)
	}

	return processCloudDnsChange(dns_spec, change)
}
