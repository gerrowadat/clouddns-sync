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

# Set up your first zone.
gcloud dns managed-zones create myzone --dns-name=myzone.mydomain.tld --description="My Zone"

# Set up a service account to admininster just DNS
gcloud iam service-accounts create gcloud-dns-sync

# Add your service account to the DNS Administrator role for this project
gcloud projects add-iam-policy-binding myproject  --member=serviceAcount:cloud-dns-sync@mydnsproject.iam.gserviceaccount.com --role=roles/dns.admin

# Generate a key for the service account. There's a more secure way of doing this with identity federation doodly do but who can be arsed.
gcloud iam service-accounts keys create cloud-dns-sync.key.json --key-file-type=json --iam-account=cloud-dns-sync@mydnsproject.iam.gserviceaccount.com
