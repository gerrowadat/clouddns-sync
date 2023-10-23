package main

import (
	"log"

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

func syncNomad(dnsSpec *CloudDNSSpec, nomadSpec *NomadSpec, dryRun *bool, pruneMissing *bool) {
	//c := make(<-chan *dns.Change)
	jobLocs := getNomadTaskLocations(nomadSpec)

	change, err := buildNomadDnsChange(dnsSpec, jobLocs, *pruneMissing)
	if err != nil {
		log.Fatal("Building DNS change from nomad info:", err)
	}

	err = processCloudDnsChange(dnsSpec, change)

	if err != nil {
		log.Fatal("Updating Cloud DNS from nomad:", err)
	}
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
