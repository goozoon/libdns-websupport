package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/libdns/websupport/websupport"

	"github.com/libdns/libdns"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: libdns-websupport <command> [args]")
		fmt.Println("")
		fmt.Println("Commands:")
		fmt.Println("  test              - Test basic DNS record operations")
		fmt.Println("  create-cert       - Create a self-signed certificate (local testing only, NOT Let's Encrypt)")
		fmt.Println("  acme-test         - Simulate ACME DNS-01 challenge (does NOT obtain real certificate)")
		fmt.Println("")
		fmt.Println("⚠️  Note: These commands are for TESTING only. To get real Let's Encrypt certificates,")
		fmt.Println("    use this provider with Caddy, Traefik, Certbot, or another ACME client.")
		fmt.Println("")
		fmt.Println("Required Environment Variables:")
		fmt.Println("  WEBSUPPORT_API_KEY       - Your Websupport API key")
		fmt.Println("  WEBSUPPORT_API_SECRET    - Your Websupport API secret")
		fmt.Println("  WEBSUPPORT_SERVICE_ID    - Numeric service ID for your domain")
		fmt.Println("  WEBSUPPORT_TEST_ZONE     - Your domain name (e.g., example.com)")
		fmt.Println("")
		fmt.Println("Example:")
		fmt.Println("  export WEBSUPPORT_API_KEY=\"your-api-key\"")
		fmt.Println("  export WEBSUPPORT_API_SECRET=\"your-api-secret\"")
		fmt.Println("  export WEBSUPPORT_SERVICE_ID=\"1234567\"")
		fmt.Println("  export WEBSUPPORT_TEST_ZONE=\"example.com\"")
		fmt.Println("  ./libdns-websupport test")
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "test":
		testBasicOperations()
	case "create-cert":
		createSelfSignedCert()
	case "acme-test":
		testACMEChallenge()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		os.Exit(1)
	}
}

// testBasicOperations tests basic DNS record operations
func testBasicOperations() {
	provider := &websupport.Provider{
		APIKey:    os.Getenv("WEBSUPPORT_API_KEY"),
		APISecret: os.Getenv("WEBSUPPORT_API_SECRET"),
		APIBase:   "https://rest.websupport.sk/v2",
		ServiceID: os.Getenv("WEBSUPPORT_SERVICE_ID"),
	}
	// SECURITY NOTE:
	// This demo loads credentials from environment variables.
	// Ensure you do not commit real API keys or secrets to Git.
	// Set `WEBSUPPORT_API_KEY` and `WEBSUPPORT_API_SECRET` locally before running.

	if provider.APIKey == "" || provider.APISecret == "" {
		log.Fatal("Error: WEBSUPPORT_API_KEY and WEBSUPPORT_API_SECRET environment variables must be set")
	}

	if provider.ServiceID == "" {
		log.Fatal("Error: WEBSUPPORT_SERVICE_ID environment variable must be set")
	}

	ctx := context.Background()
	zone := os.Getenv("WEBSUPPORT_TEST_ZONE")
	if zone == "" {
		zone = "example.com"
	}

	log.Println("🔧 Testing DNS Record Operations")
	log.Println("================================")

	// Test 1: Create a TXT record
	log.Println("\n1️⃣  Creating TXT record...")
	testRecord := &libdns.TXT{
		Name: "_websupport-test",
		Text: fmt.Sprintf("test-value-%d", time.Now().Unix()),
		TTL:  120 * time.Second,
	}

	created, err := provider.AppendRecords(ctx, zone, []libdns.Record{testRecord})
	if err != nil {
		log.Fatalf("❌ Failed to create record: %v", err)
	}
	log.Printf("✅ Created record: %+v\n", created[0])

	// Test 2: Retrieve records
	log.Println("\n2️⃣  Retrieving all records...")
	records, err := provider.GetRecords(ctx, zone)
	if err != nil {
		log.Fatalf("❌ Failed to get records: %v", err)
	}
	log.Printf("✅ Found %d TXT records\n", len(records))
	for _, rec := range records {
		if txtRec, ok := rec.(*libdns.TXT); ok {
			log.Printf("   - Name: %s, Text: %s, TTL: %v, ID: %v\n",
				txtRec.Name, txtRec.Text, txtRec.TTL, txtRec.ProviderData)
		}
	}

	// Test 3: Delete the record
	log.Println("\n3️⃣  Deleting record...")
	deleted, err := provider.DeleteRecords(ctx, zone, created)
	if err != nil {
		log.Fatalf("❌ Failed to delete record: %v", err)
	}
	log.Printf("✅ Deleted %d records\n", len(deleted))

	log.Println("\n✨ All tests passed!")
}

// createSelfSignedCert creates a self-signed certificate for testing
func createSelfSignedCert() {
	domain := os.Getenv("WEBSUPPORT_TEST_DOMAIN")
	if domain == "" {
		domain = "libdns.example.com"
	}
	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		homeDir = "." // fallback to current directory if home cannot be resolved
	}
	outputDir := filepath.Join(homeDir, ".caddy", "certificates")

	if err := os.MkdirAll(outputDir, 0700); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	log.Printf("📝 Creating self-signed certificate for: %s\n", domain)
	log.Printf("📁 Output directory: %s\n", outputDir)

	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatalf("Failed to generate private key: %v", err)
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: domain,
		},
		DNSNames:    []string{domain},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(1, 0, 0), // Valid for 1 year
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	// Create certificate
	certBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		log.Fatalf("Failed to create certificate: %v", err)
	}

	// Save certificate
	certFile := filepath.Join(outputDir, domain+".crt")
	certOut, err := os.Create(certFile)
	if err != nil {
		log.Fatalf("Failed to create cert file: %v", err)
	}
	defer certOut.Close()

	err = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
	if err != nil {
		log.Fatalf("Failed to encode certificate: %v", err)
	}

	// Save private key
	keyFile := filepath.Join(outputDir, domain+".key")
	keyOut, err := os.Create(keyFile)
	if err != nil {
		log.Fatalf("Failed to create key file: %v", err)
	}
	defer keyOut.Close()

	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	err = pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privateKeyBytes})
	if err != nil {
		log.Fatalf("Failed to encode private key: %v", err)
	}

	log.Printf("✅ Certificate saved to: %s\n", certFile)
	log.Printf("✅ Private key saved to: %s\n", keyFile)

	// Display certificate info
	cert, _ := x509.ParseCertificate(certBytes)
	log.Printf("\n📋 Certificate Details:\n")
	log.Printf("   Subject: %s\n", cert.Subject.String())
	log.Printf("   NotBefore: %s\n", cert.NotBefore.Format(time.RFC3339))
	log.Printf("   NotAfter: %s\n", cert.NotAfter.Format(time.RFC3339))
	log.Printf("   DNS Names: %v\n", cert.DNSNames)
}

// testACMEChallenge simulates an ACME DNS-01 challenge
func testACMEChallenge() {
	provider := &websupport.Provider{
		APIKey:    os.Getenv("WEBSUPPORT_API_KEY"),
		APISecret: os.Getenv("WEBSUPPORT_API_SECRET"),
		APIBase:   "https://rest.websupport.sk/v2",
		ServiceID: os.Getenv("WEBSUPPORT_SERVICE_ID"),
	}
	// SECURITY NOTE:
	// Use environment variables or a secrets manager for credentials.
	// Never hardcode or commit sensitive values.

	if provider.APIKey == "" || provider.APISecret == "" {
		log.Fatal("Error: WEBSUPPORT_API_KEY and WEBSUPPORT_API_SECRET environment variables must be set")
	}

	if provider.ServiceID == "" {
		log.Fatal("Error: WEBSUPPORT_SERVICE_ID environment variable must be set")
	}

	ctx := context.Background()
	zone := os.Getenv("WEBSUPPORT_TEST_ZONE")
	if zone == "" {
		zone = "example.com"
	}
	domain := os.Getenv("WEBSUPPORT_TEST_DOMAIN")
	if domain == "" {
		domain = "libdns.example.com"
	}

	log.Println("🔐 Simulating ACME DNS-01 Challenge")
	log.Println("===================================")

	// Step 1: Create challenge record
	log.Printf("\n1️⃣  Creating DNS challenge record for: %s\n", domain)
	challengeValue := base64.RawURLEncoding.EncodeToString([]byte("test-acme-challenge-value-" + fmt.Sprintf("%d", time.Now().Unix())))

	challengeRecord := &libdns.TXT{
		Name: "_acme-challenge",
		Text: challengeValue,
		TTL:  120 * time.Second,
	}

	created, err := provider.AppendRecords(ctx, zone, []libdns.Record{challengeRecord})
	if err != nil {
		log.Fatalf("❌ Failed to create challenge record: %v", err)
	}
	log.Printf("✅ Created challenge record with ID: %v\n", created[0].(*libdns.TXT).ProviderData)

	// Step 2: Wait for DNS propagation
	log.Println("\n2️⃣  Waiting for DNS propagation (5 seconds)...")
	time.Sleep(5 * time.Second)

	// Step 3: Verify DNS record exists
	log.Println("\n3️⃣  Verifying DNS record via public DNS lookup...")
	dnsName := fmt.Sprintf("_acme-challenge.%s", domain)
	if testDNSLookup(dnsName, challengeValue) {
		log.Printf("✅ DNS record verified publicly!\n")
	} else {
		log.Printf("⚠️  DNS record not yet publicly available (this is normal for recent changes)\n")
	}

	// Step 4: Retrieve records
	log.Println("\n4️⃣  Retrieving all records from API...")
	records, err := provider.GetRecords(ctx, zone)
	if err != nil {
		log.Fatalf("❌ Failed to get records: %v", err)
	}

	found := false
	for _, rec := range records {
		if txtRec, ok := rec.(*libdns.TXT); ok {
			if txtRec.Name == "_acme-challenge" && strings.Contains(txtRec.Text, "test-acme-challenge") {
				log.Printf("✅ Found challenge record: %s = %s\n", txtRec.Name, txtRec.Text)
				found = true
			}
		}
	}

	if !found {
		log.Printf("⚠️  Challenge record not found in API response\n")
	}

	// Step 5: Clean up
	log.Println("\n5️⃣  Cleaning up (deleting challenge record)...")
	deleted, err := provider.DeleteRecords(ctx, zone, created)
	if err != nil {
		log.Fatalf("❌ Failed to delete challenge record: %v", err)
	}
	log.Printf("✅ Deleted %d records\n", len(deleted))

	log.Println("\n✨ ACME DNS-01 test completed successfully!")
}

// testDNSLookup attempts to verify DNS record via public DNS
func testDNSLookup(name, expectedValue string) bool {
	// Try using Google DNS API
	url := fmt.Sprintf("https://8.8.8.8:443/resolve?name=%s&type=TXT", name)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return false
	}

	if answers, ok := result["Answer"].([]interface{}); ok {
		for _, answer := range answers {
			if answerMap, ok := answer.(map[string]interface{}); ok {
				if data, ok := answerMap["data"].(string); ok {
					if strings.Contains(data, expectedValue) {
						return true
					}
				}
			}
		}
	}

	return false
}
