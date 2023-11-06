package main

import (
	"reflect"
	"testing"

	"google.golang.org/api/dns/v1"
)

func Test_rrsetsEqual(t *testing.T) {
	type args struct {
		x *dns.ResourceRecordSet
		y *dns.ResourceRecordSet
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "EmptySetsAreEqual",
			args: args{
				x: &dns.ResourceRecordSet{},
				y: &dns.ResourceRecordSet{},
			},
			want: true,
		},
		{
			name: "SimpleEqual",
			args: args{
				x: &dns.ResourceRecordSet{
					Type: "A",
					Name: "hostname.example.com",
				},
				y: &dns.ResourceRecordSet{
					Type: "A",
					Name: "hostname.example.com",
				},
			},
			want: true,
		},
		{
			name: "DifferingType",
			args: args{
				x: &dns.ResourceRecordSet{
					Type: "A",
					Name: "hostname.example.com",
				},
				y: &dns.ResourceRecordSet{
					Type: "CNAME",
					Name: "hostname.example.com",
				},
			},
			want: false,
		},
		{
			name: "DifferingTtl",
			args: args{
				x: &dns.ResourceRecordSet{
					Type: "A",
					Name: "hostname.example.com",
					Ttl:  60,
				},
				y: &dns.ResourceRecordSet{
					Type: "A",
					Name: "hostname.example.com",
				},
			},
			want: false,
		},
		{
			name: "DifferingRRdatas_len",
			args: args{
				x: &dns.ResourceRecordSet{
					Type:    "A",
					Name:    "hostname.example.com",
					Rrdatas: []string{"1.2.3.4"},
				},
				y: &dns.ResourceRecordSet{
					Type:    "A",
					Name:    "hostname.example.com",
					Rrdatas: []string{"1.2.3.4", "5.6.7.8"},
				},
			},
			want: false,
		},
		{
			name: "DifferingRRdatas_data",
			args: args{
				x: &dns.ResourceRecordSet{
					Type:    "A",
					Name:    "hostname.example.com",
					Rrdatas: []string{"1.2.3.4"},
				},
				y: &dns.ResourceRecordSet{
					Type:    "A",
					Name:    "hostname.example.com",
					Rrdatas: []string{"1.2.3.5"},
				},
			},
			want: false,
		},
		{
			name: "SameRRdatas",
			args: args{
				x: &dns.ResourceRecordSet{
					Type:    "A",
					Name:    "hostname.example.com",
					Rrdatas: []string{"1.2.3.4", "5.6.7.8"},
				},
				y: &dns.ResourceRecordSet{
					Type:    "A",
					Name:    "hostname.example.com",
					Rrdatas: []string{"1.2.3.4", "5.6.7.8"},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rrsetsEqual(tt.args.x, tt.args.y); got != tt.want {
				t.Errorf("rrsetsEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestZoneFileFragment(t *testing.T) {
	type args struct {
		rr *dns.ResourceRecordSet
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Empty",
			args: args{rr: &dns.ResourceRecordSet{}},
			want: "",
		},
		{
			name: "SimpleA",
			args: args{rr: &dns.ResourceRecordSet{
				Type:    "A",
				Name:    "hostname.example.com.",
				Rrdatas: []string{"1.2.3.4"},
			}},
			want: "hostname.example.com. IN A 1.2.3.4",
		},
		{
			name: "MultipleA",
			args: args{rr: &dns.ResourceRecordSet{
				Type:    "A",
				Name:    "hostname.example.com.",
				Rrdatas: []string{"1.2.3.4", "5.6.7.8"},
			}},
			want: "hostname.example.com. IN A 1.2.3.4\nhostname.example.com. IN A 5.6.7.8",
		},
		{
			name: "SimpleCNAME",
			args: args{rr: &dns.ResourceRecordSet{
				Type:    "CNAME",
				Name:    "hostname.example.com.",
				Rrdatas: []string{"otherhostname.example.com."},
			}},
			want: "hostname.example.com. IN CNAME otherhostname.example.com.",
		},
		{
			name: "SimpleCNAMEWithTTL",
			args: args{rr: &dns.ResourceRecordSet{
				Type:    "CNAME",
				Name:    "hostname.example.com.",
				Ttl:     60,
				Rrdatas: []string{"otherhostname.example.com."},
			}},
			want: "hostname.example.com. 60 IN CNAME otherhostname.example.com.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ZoneFileFragment(tt.args.rr); got != tt.want {
				t.Errorf("ZoneFileFragment() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_mergeAnswerToRrsets(t *testing.T) {
	type args struct {
		rrsets []*dns.ResourceRecordSet
		name   string
		ip     string
	}
	tests := []struct {
		name string
		args args
		want []*dns.ResourceRecordSet
	}{
		// TODO: Add test cases.
		{
			name: "MergeSimpleRecordtoNothing",
			args: args{
				rrsets: []*dns.ResourceRecordSet{},
				name:   "doot",
				ip:     "1.2.3.4",
			},
			want: []*dns.ResourceRecordSet{
				{
					Name:    "doot",
					Type:    "A",
					Rrdatas: []string{"1.2.3.4"},
				},
			},
		},
		{
			name: "MergeSimpleRecordtoOther",
			args: args{
				rrsets: []*dns.ResourceRecordSet{
					{
						Name:    "blarg",
						Type:    "A",
						Rrdatas: []string{"1.2.3.4"},
					},
				},
				name: "doot",
				ip:   "5.6.7.8",
			},
			want: []*dns.ResourceRecordSet{
				{
					Name:    "blarg",
					Type:    "A",
					Rrdatas: []string{"1.2.3.4"},
				},
				{
					Name:    "doot",
					Type:    "A",
					Rrdatas: []string{"5.6.7.8"},
				},
			},
		},
		{
			name: "MergeSimpleRecordtoSameName",
			args: args{
				rrsets: []*dns.ResourceRecordSet{
					{
						Name:    "doot",
						Type:    "A",
						Rrdatas: []string{"1.2.3.4"},
					},
				},
				name: "doot",
				ip:   "5.6.7.8",
			},
			want: []*dns.ResourceRecordSet{
				{
					Name:    "doot",
					Type:    "A",
					Rrdatas: []string{"1.2.3.4", "5.6.7.8"},
				},
			},
		},
		{
			name: "MergeSimpleRecordtoSameNameSameIP",
			args: args{
				rrsets: []*dns.ResourceRecordSet{
					{
						Name:    "doot",
						Type:    "A",
						Rrdatas: []string{"1.2.3.4"},
					},
				},
				name: "doot",
				ip:   "1.2.3.4",
			},
			want: []*dns.ResourceRecordSet{
				{
					Name:    "doot",
					Type:    "A",
					Rrdatas: []string{"1.2.3.4"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mergeAnswerToRrsets(tt.args.rrsets, tt.args.name, tt.args.ip); !reflect.DeepEqual(got, tt.want) {
				for _, rr := range got {
					t.Logf("Got: %s (%s), %d answers.", rr.Name, rr.Type, len(rr.Rrdatas))
					for _, rrd := range rr.Rrdatas {
						t.Logf("Got: - %s", rrd)
					}
				}
				t.Errorf("mergeAnswerToRrsets() = %v records, want %v", len(got), len(tt.want))
			}
		})
	}
}