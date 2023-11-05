package main

import (
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
