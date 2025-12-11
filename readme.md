# libdns-websupport: Websupport DNS Provider (libdns)

This project implements a [libdns](https://github.com/libdns/libdns) provider for [Websupport](https://rest.websupport.sk/v2/docs) DNS.  
Implements the [libdns](https://github.com/libdns/libdns) interfaces for Websupport's DNS API so you can manage TXT records for ACME DNS-01 and other use cases.

Important: This repo includes a Windows-friendly test app (`main.go`) that can:
- Create/delete TXT records via Websupport API
- Generate a self‑signed certificate for `libdns.example.com` for local testing
- Simulate an ACME DNS‑01 workflow (create/verify/cleanup)

---

## Import path note

You may notice imports like `github.com/libdns/websupport/websupport` (double `websupport`).
This is because the provider implementation lives in the `websupport/` subdirectory of the repository.

- The module path is `github.com/libdns/websupport` (the repository root).
- The package that implements the provider is in the `websupport` subfolder, so the full import path becomes `github.com/libdns/websupport/websupport`.

If you prefer a single-segment import (for example `github.com/libdns/websupport`), we can reorganize the repository so the provider package is at the repository root and move the test application into `cmd/` (recommended). Let me know if you want me to do that.


## Features

- **ACME DNS-01 Support**: Solve DNS challenges for Let's Encrypt and other ACME providers
- **Full libdns Interface**: Implements `RecordAppender`, `RecordDeleter`, and `RecordGetter` interfaces
- **TXT Record Management**: Create, retrieve, and delete DNS TXT records
- **Basic Authentication**: Secure API communication using Websupport API credentials
- **Context Support**: Full context cancellation support for timeouts and cancellations

---

## Installation

To use this provider in your project, add it as a dependency:

```bash
go get github.com/libdns/websupport
```

Alternatively, clone the repository:

```bash
git clone https://github.com/libdns/websupport.git
cd websupport
go mod download
```

---

## Usage

### Basic Setup

```go
package main

import (
    "context"
    "github.com/libdns/libdns"
    "github.com/libdns/websupport/websupport"
    "time"
)

func main() {
    // Create a provider with your Websupport credentials
    // SECURITY: Do NOT hardcode real API keys. Prefer environment variables.
    provider := &websupport.Provider{
        APIKey:    os.Getenv("WEBSUPPORT_API_KEY"),
        APISecret: os.Getenv("WEBSUPPORT_API_SECRET"),
        APIBase:   "https://rest.websupport.sk/v2",
    }

    ctx := context.Background()
    zone := "example.com"

    // Create a TXT record for ACME challenge
    records := []libdns.Record{
        &libdns.TXT{
            Name: "_acme-challenge",
            Text: "challenge-value",
            TTL:  120 * time.Second,
        },
    }

    // Append records
    created, err := provider.AppendRecords(ctx, zone, records)
    if err != nil {
        panic(err)
    }
    
    // ... use created records ...

    // Delete records when done
    deleted, err := provider.DeleteRecords(ctx, zone, created)
    if err != nil {
        panic(err)
    }
}
```

---

## Configuration

### Environment Variables

You can configure the provider using environment variables:

```bash
export WEBSUPPORT_API_KEY="your-api-key"
export WEBSUPPORT_API_SECRET="your-api-secret"
export WEBSUPPORT_SERVICE_ID="your-service-id"  # Required: Numeric ID for your domain
export WEBSUPPORT_TEST_ZONE="example.com"       # Your domain name (not subdomain)
```

**Important Notes:**
- `WEBSUPPORT_TEST_ZONE` is your **root domain** (e.g., `example.com`), NOT a subdomain
- `WEBSUPPORT_SERVICE_ID` is **required** - this is the numeric ID for your domain
- When creating records for subdomains like `test.example.com`, use `Name: "test"` in the record

**Why is WEBSUPPORT_SERVICE_ID required?**

The Websupport REST API v2 uses service-based endpoints (`/v2/service/{id}/dns/record`) rather than domain-based endpoints. The API does not provide a working endpoint to automatically discover service IDs from domain names, so you must provide it manually.

**How to find your Service ID:**

1. Log in to [Websupport Admin Panel](https://admin.websupport.sk/)
2. Click on your domain from the services list
3. Look at the URL in your browser address bar
4. The service ID is the number at the end of the URL

Example: `https://admin.websupport.sk/en/dashboard/service/1234567` → Service ID is `1234567`

### Testing the Provider

To test the provider with your credentials:

```bash
# Build the project
go build .

# Set environment variables
export WEBSUPPORT_API_KEY="your-api-key"
export WEBSUPPORT_API_SECRET="your-api-secret"
export WEBSUPPORT_SERVICE_ID="your-service-id"
export WEBSUPPORT_TEST_ZONE="your-domain.com"

# Run tests
./libdns-websupport test
```

**Complete test command example:**
```bash
cd /path/to/websupport && \
  go build . && \
  export WEBSUPPORT_API_KEY="your-api-key" && \
  export WEBSUPPORT_API_SECRET="your-api-secret" && \
  export WEBSUPPORT_TEST_ZONE="example.com" && \
  export WEBSUPPORT_SERVICE_ID="1234567" && \
  ./libdns-websupport test
```

### Provider Struct

```go
type Provider struct {
    APIKey     string        // Websupport API Key
    APISecret  string        // Websupport API Secret
    APIBase    string        // API Base URL (default: https://rest.websupport.sk/v2)
    ServiceID  string        // Service ID for the domain (required)
    HTTPClient *http.Client  // Custom HTTP client (optional)
    Timeout    time.Duration // Request timeout (default: 30s)
}
```

---

## API Reference

### AppendRecords

Creates DNS records in the zone. Used for adding ACME challenge records.

```go
func (p *Provider) AppendRecords(ctx context.Context, zone string, recs []libdns.Record) ([]libdns.Record, error)
```

- **Parameters**:
  - `ctx`: Context for cancellation and timeouts
  - `zone`: Domain name (e.g., "example.com")
  - `recs`: Records to create (typically `libdns.TXT` records)
- **Returns**: Created records with populated IDs and any errors

### DeleteRecords

Removes DNS records from the zone by ID.

```go
func (p *Provider) DeleteRecords(ctx context.Context, zone string, recs []libdns.Record) ([]libdns.Record, error)
```

- **Parameters**:
  - `ctx`: Context for cancellation and timeouts
  - `zone`: Domain name
  - `recs`: Records to delete (must have valid IDs from creation)
- **Returns**: Deleted records and any errors

### GetRecords

Retrieves all DNS records from the zone.

```go
func (p *Provider) GetRecords(ctx context.Context, zone string) ([]libdns.Record, error)
```

- **Parameters**:
  - `ctx`: Context for cancellation and timeouts
  - `zone`: Domain name
- **Returns**: All TXT records in the zone and any errors

---

## Examples

### Complete ACME Challenge Workflow

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/libdns/libdns"
    "github.com/libdns/websupport/websupport"
)

func main() {
    provider := &websupport.Provider{
      APIKey:    os.Getenv("WEBSUPPORT_API_KEY"),
      APISecret: os.Getenv("WEBSUPPORT_API_SECRET"),
      APIBase:   "https://rest.websupport.sk/v2",
    }

    ctx := context.Background()
    zone := "example.com"

    // Step 1: Create challenge record
    challengeRecord := &libdns.TXT{
        Name: "_acme-challenge",
        Text: "your-challenge-token",
        TTL:  120 * time.Second,
    }

    created, err := provider.AppendRecords(ctx, zone, []libdns.Record{challengeRecord})
    if err != nil {
        fmt.Printf("Failed to create record: %v\n", err)
        return
    }
    fmt.Printf("Created record: %+v\n", created[0])

    // Step 2: Wait for DNS propagation
    time.Sleep(5 * time.Second)

    // Step 3: Verify record exists
    records, err := provider.GetRecords(ctx, zone)
    if err != nil {
        fmt.Printf("Failed to get records: %v\n", err)
        return
    }
    fmt.Printf("Found %d records\n", len(records))

    // Step 4: Clean up
    deleted, err := provider.DeleteRecords(ctx, zone, created)
    if err != nil {
        fmt.Printf("Failed to delete record: %v\n", err)
        return
    }
    fmt.Printf("Deleted %d records\n", len(deleted))
}
```

### Integration with Caddy (example JSON)

This provider can be integrated with Caddy's DNS plugin system via CertMagic:

```json
{
  "apps": {
    "tls": {
      "automation": {
        "policies": [
          {
            "issuers": [
              {
                "module": "acme",
                "challenges": {
                  "dns": {
                    "provider": {
                      "name": "websupport",
                      "api_key": "${WEBSUPPORT_API_KEY}",
                      "api_secret": "${WEBSUPPORT_API_SECRET}"
                    }
                  }
                }
              }
            ]
          }
        ]
      }
    }
  }
}
```

---

## Testing

The project includes a comprehensive test application that allows you to validate the DNS provider functionality and generate test certificates.

### Test Commands

Environment variables used by the test app:

- `WEBSUPPORT_API_KEY` — Your Websupport API key (required)
- `WEBSUPPORT_API_SECRET` — Your Websupport API secret (required)
- `WEBSUPPORT_SERVICE_ID` — Numeric service ID for your domain (required)
- `WEBSUPPORT_TEST_ZONE` — Your root domain (e.g., `example.com`) - NOT a subdomain
- `WEBSUPPORT_TEST_DOMAIN` — FQDN for cert/tests (default: `libdns.example.com`)

**Important:** `WEBSUPPORT_TEST_ZONE` should be your **root domain** like `example.com`, not a subdomain like `test.example.com`.

The test application supports three commands:

#### 1. Basic DNS Operations Test

Tests creating, retrieving, and deleting DNS records:

**Linux/Mac:**
```bash
export WEBSUPPORT_API_KEY="your-api-key"
export WEBSUPPORT_API_SECRET="your-api-secret"
export WEBSUPPORT_SERVICE_ID="1234567"
export WEBSUPPORT_TEST_ZONE="example.com"
./libdns-websupport test
```

**Windows:**
```powershell
$env:WEBSUPPORT_API_KEY = "your-api-key"
$env:WEBSUPPORT_API_SECRET = "your-api-secret"
$env:WEBSUPPORT_SERVICE_ID = "1234567"
$env:WEBSUPPORT_TEST_ZONE = "example.com"
.\libdns-websupport.exe test
```

This will:
- Create a test TXT record
- Retrieve all records from your zone
- Delete the test record
- Display success/failure information

#### 2. Create Self-Signed Certificate (Local Testing Only)

Generates a **self-signed certificate** for local testing purposes. This is NOT a real Let's Encrypt certificate and will show security warnings in browsers.

**Linux/Mac:**
```bash
./libdns-websupport create-cert
```

**Windows:**
```powershell
.\libdns-websupport.exe create-cert
```

This will:
- Generate a 2048-bit RSA private key
- Create a **self-signed certificate** (NOT trusted by browsers, for testing only)
- Certificate is valid for 1 year
- Save certificate to: `~/.caddy/certificates/libdns.example.com.crt` (Linux/Mac) or `C:\Users\<YourUsername>\.caddy\certificates\libdns.example.com.crt` (Windows)
- Save private key to: `~/.caddy/certificates/libdns.example.com.key` (Linux/Mac) or `C:\Users\<YourUsername>\.caddy\certificates\libdns.example.com.key` (Windows)

**Important:** This creates a self-signed certificate for testing purposes only. To get real, trusted SSL/TLS certificates, see the "Obtaining Real Let's Encrypt Certificates" section below.

#### 3. ACME DNS-01 Challenge Test (Simulation Only)

Simulates a complete ACME DNS-01 challenge workflow **WITHOUT** contacting Let's Encrypt. This tests that the DNS provider can create and clean up challenge records correctly.

**Linux/Mac:**
```bash
export WEBSUPPORT_API_KEY="your-api-key"
export WEBSUPPORT_API_SECRET="your-api-secret"
export WEBSUPPORT_SERVICE_ID="1234567"
export WEBSUPPORT_TEST_ZONE="example.com"
./libdns-websupport acme-test
```

**Windows:**
```powershell
$env:WEBSUPPORT_API_KEY = "your-api-key"
$env:WEBSUPPORT_API_SECRET = "your-api-secret"
$env:WEBSUPPORT_SERVICE_ID = "1234567"
$env:WEBSUPPORT_TEST_ZONE = "example.com"
.\libdns-websupport.exe acme-test
```

This will:
1. Create a DNS challenge record (`_acme-challenge` TXT record)
2. Wait for DNS propagation
3. Verify the record via public DNS lookup
4. Retrieve records from the API
5. Clean up the challenge record

**Important:** This command only **simulates** the ACME workflow for testing purposes. It does NOT contact Let's Encrypt and does NOT issue a real certificate. See the section below for obtaining real certificates.

---

## Obtaining Real Let's Encrypt Certificates

**None of the built-in test commands (`test`, `create-cert`, `acme-test`) obtain real Let's Encrypt certificates.** They are only for testing the DNS provider functionality.

To obtain **real, trusted SSL/TLS certificates** from Let's Encrypt for your domain or subdomains, you need to use this provider with an ACME client.

### Recommended ACME Clients

This provider works with any ACME client that supports the libdns interface:

1. **[Caddy](https://caddyserver.com/)** - Automatic HTTPS server (easiest option)
2. **[Traefik](https://traefik.io/)** - Reverse proxy with automatic HTTPS
3. **[Certbot](https://certbot.eff.org/)** - Official Let's Encrypt client
4. **[acme.sh](https://github.com/acmesh-official/acme.sh)** - Shell script ACME client
5. **[Lego](https://go-acme.github.io/lego/)** - Go-based ACME client

### For Subdomains

When obtaining certificates for subdomains like `test.example.com`:

1. Set `WEBSUPPORT_TEST_ZONE` to your **root domain** (e.g., `example.com`)
2. The ACME challenge will create `_acme-challenge.test.example.com` automatically
3. The provider will create the TXT record with `Name: "_acme-challenge.test"` in your root domain

**Example for subdomain certificate:**
```bash
export WEBSUPPORT_API_KEY="your-api-key"
export WEBSUPPORT_API_SECRET="your-api-secret"
export WEBSUPPORT_SERVICE_ID="1234567"
export WEBSUPPORT_TEST_ZONE="example.com"  # Root domain, NOT subdomain

# In your ACME client configuration, request cert for:
# - test.example.com
# - *.example.com (wildcard)
# - example.com (root)
```

### Using with Traefik

Traefik can use this provider for automatic certificate generation. Example configuration:

```yaml
certificatesResolvers:
  letsencrypt:
    acme:
      email: your-email@example.com
      storage: /acme.json
      dnsChallenge:
        provider: websupport
        resolvers:
          - "1.1.1.1:53"
          - "8.8.8.8:53"

# Environment variables for Traefik:
# WEBSUPPORT_API_KEY=your-api-key
# WEBSUPPORT_API_SECRET=your-api-secret
# WEBSUPPORT_SERVICE_ID=1234567
```

### Using with Caddy

Caddy can automatically obtain certificates using this DNS provider. You'll need to build Caddy with the Websupport DNS module or use the libdns interface directly.

---

## Building from Source

**Linux/Mac:**
```bash
git clone https://github.com/libdns/websupport.git
cd websupport
go build .
```

**Windows:**
```powershell
git clone https://github.com/libdns/websupport.git
cd websupport
go build .
```

# Simulate ACME challenge
.\libdns-websupport.exe acme-test
```

---

## Testing (Linux)

The test app works the same on Linux. Replace PowerShell with bash and note that files are written to `~/.caddy/certificates`.

### Test Commands

Environment variables used by the test app (optional but recommended):

- `WEBSUPPORT_TEST_ZONE` — your zone (default: `example.com`)
- `WEBSUPPORT_TEST_DOMAIN` — FQDN for cert/tests (default: `libdns.example.com`)

#### 1. Basic DNS Operations Test

```bash
export WEBSUPPORT_API_KEY="your-api-key"
## Task Runners

### Linux/macOS (Makefile)

Common tasks:

```bash
# Build binary
make build

# Run locally
make run

# Run tests
make test

# Create self-signed certificate (writes to ~/.caddy/certificates)
make cert

# DNS operations test (requires API env vars)
make dns-test

# ACME simulation (requires API env vars)
make acme-test
```

### Windows (PowerShell)

Use the provided `make.ps1` script:

```powershell
export WEBSUPPORT_API_SECRET="your-api-secret"
./libdns-websupport test
```

#### 2. Create Self-Signed Certificate

```bash
./libdns-websupport create-cert
ls -l ~/.caddy/certificates/libdns.example.com.*
```

Expected files:

- `~/.caddy/certificates/libdns.example.com.crt`
- `~/.caddy/certificates/libdns.example.com.key`

#### 3. ACME DNS-01 Challenge Test


```bash
export WEBSUPPORT_API_KEY="your-api-key"
export WEBSUPPORT_API_SECRET="your-api-secret"
./libdns-websupport acme-test
```

### Building from Source

```bash
git clone https://github.com/goozoon/libdns-websupport.git
cd libdns-websupport
go build .
```

### Quick Run

```bash
go run .
```

---

## Development

### Project Layout

```
libdns-websupport/
├── go.mod                  # Go module definition
├── go.sum                  # Go module checksums
├── main.go                 # Test application
├── readme.md               # This file
└── websupport/
    └── provider.go         # libdns provider implementation
```

### Building

```bash
go build ./websupport
```

### Quick Run

```powershell
go run .
```

### Running Tests

```bash
go test ./...
```

---

## Security & Publishing Checklist

- **Credentials**: Never hardcode API credentials. Use environment variables or a secrets vault.
- **HTTPS**: All API calls use HTTPS for secure communication
- **Basic Auth**: Credentials are sent via HTTP Basic Authentication
- **Rate Limiting**: Be mindful of Websupport's API rate limits when managing records
- **GitHub Safety**: Before publishing, search the repo for your domain or secrets and remove any accidental commits.
- **Local Testing**: The `create-cert` command generates a self‑signed cert; do not use it in production.

---

## Resources

- [libdns Documentation](https://pkg.go.dev/github.com/libdns/libdns)
- [Websupport API Documentation](https://rest.websupport.sk/v2/docs)
- [Caddy Documentation](https://caddyserver.com/docs/)
- [ACME DNS-01 Challenge](https://letsencrypt.org/docs/challenge-types/#dns-01)

---

## License

This project is open source and available under the MIT License.

---

## Contributing

Contributions are welcome! Please feel free to submit pull requests or open issues for bugs and feature requests.

---

## Support

For issues, questions, or suggestions, please open an issue on GitHub.