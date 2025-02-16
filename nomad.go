package main

import (
	"log"
	"time"

	nomad "github.com/hashicorp/nomad/api"
)

type TaskInfo struct {
	jobid string
	ip    string
}

type NodeInfo map[string]string

type NomadSpec struct {
	uri   string
	token string
}

func periodicallySyncNomad(dns_spec *CloudDNSSpec, nomadSpec *NomadSpec, interval int, pruneMissing *bool) {
	syncNomad(dns_spec, nomadSpec, pruneMissing)

	if interval >= 0 {
		for {
			log.Printf("Waiting %d seconds.", interval)
			time.Sleep(time.Duration(interval) * time.Second)
			syncNomad(dns_spec, nomadSpec, pruneMissing)
		}
	}
}

func syncNomad(dnsSpec *CloudDNSSpec, nomadSpec *NomadSpec, pruneMissing *bool) {
	//c := make(<-chan *dns.Change)
	jobLocs := getNomadTaskLocations(nomadSpec)

	log.Printf("Found %d nomad jobs", len(jobLocs))

	change, err := buildNomadDnsChange(dnsSpec, jobLocs, *pruneMissing)
	if err != nil {
		log.Fatal("Building DNS change from nomad info:", err)
	}

	err = processCloudDnsChange(dnsSpec, change)

	if err != nil {
		log.Fatal("Updating Cloud DNS from nomad:", err)
	}
	dnsTotalRecordCount.Set(float64(len(jobLocs)))
}

func getNomadNodesList(nomadSpec *NomadSpec) NodeInfo {
	conf := &nomad.Config{
		Address: nomadSpec.uri,
	}

	c, err := nomad.NewClient(conf)
	if err != nil {
		log.Fatal("Talking to Nomad: ", err)
	}

	nodes, _, err := c.Nodes().List(nil)
	if err != nil {
		log.Fatal("Getting Allocs from nomad: ", err)
	}

	ret := NodeInfo{}
	for _, n := range nodes {
		if n.Address == "" {
			log.Fatalf("Found nomad node %s with unknown IP", n.Name)
		}
		ret[n.Name] = n.Address
	}

	return ret
}

func getNomadAllocsList(nomadSpec *NomadSpec) []*nomad.AllocationListStub {
	conf := &nomad.Config{
		Address: nomadSpec.uri,
	}

	c, err := nomad.NewClient(conf)
	if err != nil {
		log.Fatal("Talking to Nomad: ", err)
	}

	allocs, _, err := c.Allocations().List(nil)
	if err != nil {
		log.Fatal("Getting Allocs from nomad: ", err)
	}

	return allocs
}

func getNomadTaskLocations(nomadSpec *NomadSpec) []TaskInfo {
	ret := []TaskInfo{}

	allocs := getNomadAllocsList(nomadSpec)
	nodes := getNomadNodesList(nomadSpec)

	for _, a := range allocs {

		if a.ClientStatus != "running" {
			continue
		}
		ip, ok := nodes[a.NodeName]
		if !ok {
			log.Printf("Unknown node %s for running alloc %s", a.NodeName, a.ID)
			continue
		}
		ret = append(ret, TaskInfo{
			jobid: a.JobID,
			ip:    ip,
		})

	}

	return ret
}
