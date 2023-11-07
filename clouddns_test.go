package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	zonefile "github.com/bwesterb/go-zonefile"
	"google.golang.org/api/dns/v1"
)

// Return a single-line description of an rrset
func describeRrset(rr *dns.ResourceRecordSet) string {
	return fmt.Sprintf("'%s' (%s) TTL %d : %s", rr.Name, rr.Type, rr.Ttl, strings.Join(rr.Rrdatas, ", "))
}

// Compare the fields we care about in a slice of *ResourceRecordSet to see
// If they're 'equal' for our purposes.
func rrsetListEquals(a, b []*dns.ResourceRecordSet) bool {
	if len(a) != len(b) {
		return false
	}
	for _, arr := range a {
		found := false
		for _, brr := range b {
			if rrsetsEqual(arr, brr) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

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
			name: "SimpleSOA",
			args: args{rr: &dns.ResourceRecordSet{
				Type:    "SOA",
				Rrdatas: []string{"doot. root.doot. 0 0 0"},
			}},
			want: " IN SOA doot. root.doot. 0 0 0",
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
				t.Errorf("ZoneFileFragment() = '%v', want '%v'", got, tt.want)
			}
		})
	}
}

func Test_mergeAnswerToRrsets(t *testing.T) {
	type args struct {
		rrsets      []*dns.ResourceRecordSet
		name        string
		ip          string
		default_ttl int
	}
	tests := []struct {
		name string
		args args
		want []*dns.ResourceRecordSet
	}{
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
			if got := mergeAnswerToRrsets(tt.args.rrsets, tt.args.name, tt.args.ip, tt.args.default_ttl); !reflect.DeepEqual(got, tt.want) {
				for _, rr := range got {
					t.Logf("Got: %s (%s) TTL %d, %d answers.", rr.Name, rr.Type, rr.Ttl, len(rr.Rrdatas))
					for _, rrd := range rr.Rrdatas {
						t.Logf("Got: - %s", rrd)
					}
				}
				t.Errorf("mergeAnswerToRrsets() = %v records, want %v", len(got), len(tt.want))
			}
		})
	}
}

func sloppyParseEntry(entry string) zonefile.Entry {
	ret, _ := zonefile.ParseEntry([]byte(entry))
	return ret
}

func Test_mergeZoneEntryIntoRrsets(t *testing.T) {
	test_domain := "mydomain.test."

	type args struct {
		dnsSpec *CloudDNSSpec
		rrs     []*dns.ResourceRecordSet
		e       zonefile.Entry
	}
	tests := []struct {
		name string
		args args
		want []*dns.ResourceRecordSet
	}{
		// TODO: Add test cases.
		{
			// ns and SOA records get ignored.
			name: "mergeNS",
			args: args{
				dnsSpec: nil,
				rrs:     []*dns.ResourceRecordSet{},
				e:       sloppyParseEntry(" IN NS ns1.example.com."),
			},
			want: []*dns.ResourceRecordSet{},
		},
		{
			// ns and SOA records get ignored.
			name: "mergeSOA",
			args: args{
				dnsSpec: nil,
				rrs:     []*dns.ResourceRecordSet{},
				e:       sloppyParseEntry(" IN SOA doot. root.doot. 0 0 0 0 0 0"),
			},
			want: []*dns.ResourceRecordSet{},
		},
		{
			// set a bare name and see if we get it qualified
			name: "qualifyBareName",
			args: args{
				dnsSpec: &CloudDNSSpec{
					domain: &test_domain,
				},
				rrs: []*dns.ResourceRecordSet{},
				e:   sloppyParseEntry("barename IN A 1.2.3.4"),
			},
			want: []*dns.ResourceRecordSet{
				{
					Name:    string("barename." + test_domain),
					Type:    "A",
					Rrdatas: []string{"1.2.3.4"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mergeZoneEntryIntoRrsets(tt.args.dnsSpec, tt.args.rrs, tt.args.e); !rrsetListEquals(got, tt.want) {
				for _, rr := range tt.want {
					t.Logf("Want: %s", describeRrset(rr))
				}
				for _, rr := range got {
					t.Logf("Got : %s", describeRrset(rr))
				}
				t.Errorf("mergeZoneEntryIntoRrsets() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_buildTaskInfoToRrsets(t *testing.T) {
	test_default_ttl := 60
	type args struct {
		tasks       []TaskInfo
		default_ttl *int
	}
	tests := []struct {
		name    string
		args    args
		want    []*dns.ResourceRecordSet
		wantErr bool
	}{
		{
			name: "EmptyTaskInfo",
			args: args{
				tasks: []TaskInfo{},
			},
			want:    []*dns.ResourceRecordSet{},
			wantErr: false,
		},
		{
			name: "SimpleTaskInfo",
			args: args{
				tasks: []TaskInfo{
					{
						jobid: "doot",
						ip:    "1.2.3.4",
					},
				},
				default_ttl: &test_default_ttl,
			},
			want: []*dns.ResourceRecordSet{
				{
					Name:    "doot",
					Type:    "A",
					Ttl:     int64(test_default_ttl),
					Rrdatas: []string{"1.2.3.4"},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildTaskInfoToRrsets(tt.args.tasks, tt.args.default_ttl)
			if (err != nil) != tt.wantErr {
				t.Errorf("buildTaskInfoToRrsets() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !rrsetListEquals(got, tt.want) {
				for _, rr := range tt.want {
					t.Logf("Want: %s", describeRrset(rr))
				}
				for _, rr := range got {
					t.Logf("Got : %s", describeRrset(rr))
				}
				t.Errorf("buildTaskInfoToRrsets() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_buildDnsChange(t *testing.T) {
	simpleARecord := &dns.ResourceRecordSet{
		Name:    "doot.doot.",
		Type:    "A",
		Rrdatas: []string{"1.2.3.4"},
	}
	otherARecord := &dns.ResourceRecordSet{
		Name:    "doot.doot.",
		Type:    "A",
		Rrdatas: []string{"5.6.7.8"},
	}
	simpleCnameRecord := &dns.ResourceRecordSet{
		Name:    "otherdoot.doot.",
		Type:    "CNAME",
		Rrdatas: []string{"doot.doot."},
	}

	type args struct {
		cloud_rrs     []*dns.ResourceRecordSet
		zone_rrs      []*dns.ResourceRecordSet
		prune_missing bool
	}
	tests := []struct {
		name string
		args args
		want *dns.Change
	}{
		{
			name: "BothEmpty",
			args: args{
				cloud_rrs:     []*dns.ResourceRecordSet{},
				zone_rrs:      []*dns.ResourceRecordSet{},
				prune_missing: false,
			},
			want: &dns.Change{},
		},
		{
			name: "SingleRecordIntoEmptyCloud",
			args: args{
				cloud_rrs:     []*dns.ResourceRecordSet{},
				zone_rrs:      []*dns.ResourceRecordSet{simpleARecord},
				prune_missing: false,
			},
			want: &dns.Change{
				Additions: []*dns.ResourceRecordSet{simpleARecord},
			},
		},
		{
			name: "AdditionalRecordIntoNonEmptyCloud",
			args: args{
				cloud_rrs:     []*dns.ResourceRecordSet{simpleCnameRecord},
				zone_rrs:      []*dns.ResourceRecordSet{simpleCnameRecord, simpleARecord},
				prune_missing: false,
			},
			want: &dns.Change{
				Additions: []*dns.ResourceRecordSet{simpleARecord},
			},
		},
		{
			name: "PruneMissing",
			args: args{
				cloud_rrs:     []*dns.ResourceRecordSet{simpleCnameRecord, simpleARecord},
				zone_rrs:      []*dns.ResourceRecordSet{simpleCnameRecord},
				prune_missing: true,
			},
			want: &dns.Change{
				Deletions: []*dns.ResourceRecordSet{simpleARecord},
			},
		},
		{
			name: "ReplaceExistingRecord",
			args: args{
				cloud_rrs:     []*dns.ResourceRecordSet{simpleARecord},
				zone_rrs:      []*dns.ResourceRecordSet{otherARecord},
				prune_missing: true,
			},
			want: &dns.Change{
				Deletions: []*dns.ResourceRecordSet{simpleARecord},
				Additions: []*dns.ResourceRecordSet{otherARecord},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildDnsChange(tt.args.cloud_rrs, tt.args.zone_rrs, tt.args.prune_missing); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildDnsChange() = %v, want %v", got, tt.want)
			}
		})
	}
}
