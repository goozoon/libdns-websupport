package websupport

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/libdns/libdns"
)

// Provider implements the libdns interfaces for Websupport's DNS API.
type Provider struct {
	APIKey    string `json:"api_key,omitempty"`
	APISecret string `json:"api_secret,omitempty"`
	APIBase   string `json:"api_base,omitempty"`
	ServiceID string `json:"service_id,omitempty"` // Service ID for the domain

	HTTPClient *http.Client
	Timeout    time.Duration
}

// SECURITY NOTE:
// - Do NOT hardcode real API credentials in source code.
// - Prefer loading `APIKey` and `APISecret` from environment variables or a secure secret store.
// - When publishing to GitHub, double-check no secrets are committed.

// ensureClient initializes the HTTP client if not set.
func (p *Provider) ensureClient() {
	if p.HTTPClient == nil {
		p.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	if p.Timeout == 0 {
		p.Timeout = 30 * time.Second
	}
}

// calculateSignature generates HMAC-SHA1 signature for Websupport API authentication
func (p *Provider) calculateSignature(method, path string, timestamp int64) string {
	canonicalRequest := fmt.Sprintf("%s %s %d", method, path, timestamp)
	h := hmac.New(sha1.New, []byte(p.APISecret))
	h.Write([]byte(canonicalRequest))
	return hex.EncodeToString(h.Sum(nil))
}

// addAuthHeaders adds required authentication headers to the request
func (p *Provider) addAuthHeaders(req *http.Request, method, path string) {
	timestamp := time.Now().Unix()
	signature := p.calculateSignature(method, path, timestamp)
	req.SetBasicAuth(p.APIKey, signature)
	req.Header.Set("X-Date", time.Unix(timestamp, 0).UTC().Format("20060102T150405Z"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
}

// AppendRecords creates DNS records (used for ACME TXT records).
func (p *Provider) AppendRecords(ctx context.Context, zone string, recs []libdns.Record) ([]libdns.Record, error) {
	p.ensureClient()

	if p.ServiceID == "" {
		return nil, fmt.Errorf("ServiceID is required - set WEBSUPPORT_SERVICE_ID environment variable")
	}

	var created []libdns.Record
	for _, rec := range recs {
		// Type assert to TXT record
		r, ok := rec.(*libdns.TXT)
		if !ok {
			continue
		}

		if r.TTL == 0 {
			r.TTL = 120 * time.Second
		}

		// Always create TXT records for ACME DNS-01
		body := fmt.Sprintf(`{"type":"TXT","name":"%s","content":"%s","ttl":%d}`,
			r.Name, r.Text, int(r.TTL.Seconds()))

		// URL path for HTTP request (without /v2 prefix)
		urlPath := fmt.Sprintf("/service/%s/dns/record", p.ServiceID)
		// Signature path must include /v2 prefix
		sigPath := fmt.Sprintf("/v2/service/%s/dns/record", p.ServiceID)

		req, err := http.NewRequestWithContext(ctx, "POST",
			p.APIBase+urlPath,
			strings.NewReader(body))
		if err != nil {
			return nil, err
		}

		p.addAuthHeaders(req, "POST", sigPath)

		resp, err := p.HTTPClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != 204 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("failed to create record: %s, body: %s", resp.Status, string(bodyBytes))
		}

		// Websupport returns 204 No Content on success
		// We need to fetch the record to get its ID
		time.Sleep(1 * time.Second) // Give DNS time to propagate
		
		allRecs, err := p.GetRecords(ctx, zone)
		if err == nil {
			// Find the record we just created by content (most reliable)
			// Name comparison needs to handle FQDN vs relative names
			normalizedName := strings.TrimSuffix(r.Name, ".")
			for _, existingRec := range allRecs {
				if txtRec, ok := existingRec.(*libdns.TXT); ok {
					existingName := strings.TrimSuffix(txtRec.Name, ".")
					existingName = strings.TrimSuffix(existingName, "."+strings.TrimSuffix(zone, "."))
					if existingName == normalizedName && txtRec.Text == r.Text {
						r.ProviderData = txtRec.ProviderData
						break
					}
				}
			}
		}

		created = append(created, r)
	}

	return created, nil
}

// DeleteRecords removes DNS records by ID.
func (p *Provider) DeleteRecords(ctx context.Context, zone string, recs []libdns.Record) ([]libdns.Record, error) {
	p.ensureClient()

	if p.ServiceID == "" {
		return nil, fmt.Errorf("ServiceID is required - set WEBSUPPORT_SERVICE_ID environment variable")
	}

	var deleted []libdns.Record
	for _, rec := range recs {
		// Type assert to TXT record
		r, ok := rec.(*libdns.TXT)
		if !ok {
			continue
		}

		// Extract ID from ProviderData
		id, ok := r.ProviderData.(string)
		if !ok || id == "" {
			// Try to find the record by name and content
			allRecs, err := p.GetRecords(ctx, zone)
			if err != nil {
				continue
			}
			for _, existingRec := range allRecs {
				if txtRec, ok := existingRec.(*libdns.TXT); ok {
					if txtRec.Name == r.Name && txtRec.Text == r.Text {
						id, _ = txtRec.ProviderData.(string)
						break
					}
				}
			}
			if id == "" {
				continue
			}
		}

		urlPath := fmt.Sprintf("/service/%s/dns/record/%s", p.ServiceID, id)
		sigPath := fmt.Sprintf("/v2/service/%s/dns/record/%s", p.ServiceID, id)

		req, err := http.NewRequestWithContext(ctx, "DELETE",
			p.APIBase+urlPath, nil)
		if err != nil {
			return nil, err
		}

		p.addAuthHeaders(req, "DELETE", sigPath)

		resp, err := p.HTTPClient.Do(req)
		if err != nil {
			return nil, err
		}
		resp.Body.Close()

		if resp.StatusCode != 204 {
			return nil, fmt.Errorf("failed to delete record: %s", resp.Status)
		}

		deleted = append(deleted, r)
	}
	return deleted, nil
}

// GetRecords retrieves all DNS records from the zone.
func (p *Provider) GetRecords(ctx context.Context, zone string) ([]libdns.Record, error) {
	p.ensureClient()

	if p.ServiceID == "" {
		return nil, fmt.Errorf("ServiceID is required - set WEBSUPPORT_SERVICE_ID environment variable")
	}

	var allRecords []libdns.Record
	page := 1

	for {
		urlPath := fmt.Sprintf("/service/%s/dns/record?page=%d&rowsPerPage=100", p.ServiceID, page)
		sigPath := fmt.Sprintf("/v2/service/%s/dns/record", p.ServiceID)

		req, err := http.NewRequestWithContext(ctx, "GET",
			p.APIBase+urlPath, nil)
		if err != nil {
			return nil, err
		}

		p.addAuthHeaders(req, "GET", sigPath)

		resp, err := p.HTTPClient.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != 200 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("failed to get records: %s, body: %s", resp.Status, string(bodyBytes))
		}

		// Parse response JSON
		var result struct {
			CurrentPage  int `json:"currentPage"`
			TotalPages   int `json:"totalPages"`
			TotalRecords int `json:"totalRecords"`
			Data         []struct {
				ID      int    `json:"id"`
				Name    string `json:"name"`
				Type    string `json:"type"`
				Content string `json:"content"`
				TTL     int    `json:"ttl"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode response: %v", err)
		}
		resp.Body.Close()

		for _, item := range result.Data {
			// Only include TXT records
			if item.Type != "TXT" {
				continue
			}

			txtRec := &libdns.TXT{
				Name:         item.Name,
				Text:         item.Content,
				TTL:          time.Duration(item.TTL) * time.Second,
				ProviderData: fmt.Sprintf("%d", item.ID),
			}
			allRecords = append(allRecords, txtRec)
		}

		// Check if there are more pages
		if result.CurrentPage >= result.TotalPages {
			break
		}
		page++
	}

	return allRecords, nil
}
