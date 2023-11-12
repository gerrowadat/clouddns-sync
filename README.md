# clouddns-sync

Utlity for syncing local DNS records to gcloud (and back).

# Getting Started

Assuming yu have the `gcloud` command set up with access to Google Cloud console, the following should get you going:

(Figuring out the clicky web console equivalent is left as an exercise to the reader etc. etc.)

```
# Create a nice project just for dns
gcloud projects create mydnsproject

# Switch to the new project
gcloud config set project mydnsproject

# Enable Cloud DNS for this project
gcloud services enable dns.googleapis.com

# Set up your first zone. Note trailing period.
gcloud dns managed-zones create myzone \
    --dns-name=myzone.mydomain.tld. --description="My Zone"

# Set up a service account to admininster just DNS
gcloud iam service-accounts create cloud-dns-sync

# Add your service account to the DNS Administrator role for this project
gcloud projects add-iam-policy-binding myproject \
    --member=serviceAcount:cloud-dns-sync@mydnsproject.iam.gserviceaccount.com \
    --role=roles/dns.admin

# Generate a key for the service account. There's a more secure way of doing this with identity federation doodly do but who can be arsed.
gcloud iam service-accounts keys create cloud-dns-sync.key.json \
    --key-file-type=json \
    --iam-account=cloud-dns-sync@mydnsproject.iam.gserviceaccount.com

```
Now, delegate dns for the (sub)domain you want to whatever is in the output of `gcloud dns managed-zones describe myzone` 

Congrats! you now have a useless DNS zone with no records! You can add them with the `gcloud` command if you like, refer to the docs.


# Tool Usage

if you're using a JSON keyfile as above, you don't need to specify ```--cloud-project``` if the project is named there.

If you don't specify ```--json-keyfile``` then we'l try to use default credentials (i.e. the ones that the ```gcloud``` CLI uses). The examples below do this for clarity.

## ```getzonefile``` and ```putzonefile``` - Zonefile Nonsense

If you want to spit out a mostly valid zonefile from your gcloud-dns zone, this will do it:

`clouddns-sync --cloud-project=mydnsproject --cloud-dns-zone=myzone getzonefile`

If you have a zonefile, slurp it into gcloud DNS by doing this: 

`clouddns-sync --cloud-project=mydnsproject --cloud-dns-zone=myzone --zonefile=myzonefile putzonefile`

You can add `--dry-run` to putzonefile to see what we'd do. You can also add `--prune-missing` to remove RRs that aren't in your zonefile but are in gcloud.

My own use case is to do this once and then do future updates from a data source more reliable than your grandad's text file.

## ```nomad_sync``` Update from Nomad cluster 

```clouddns-sync --cloud-project=mydnsproject --cloud-dns-zone=myzone --nomad-server-uri=http://anynomadserver:4646/ nomad_sync```

Right now we build a list of A records by inspecting all allocs and pointing *jobname*.domain to all nodes that hold an alloc in that job. That might not be what you want, but the important thing is that it's what I want. Patches welcome!

## ```dynrecord``` dyndns-style single record updating

This is if you have a DNS name you want to do 'dyndns' style updating for (i.e. we find out what our public IP is and set the specificed A record to that.)

```clouddns-sync --cloud-project=mydnsproject --cloud-dns-zone=myzone --cloud-dns-dyn-record-name=myhomeip.domain.tld. dynrecord```