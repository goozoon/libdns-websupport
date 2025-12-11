//go:build ignore

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
		r, ok := rec.(*libdns.TXT)
		if !ok {
			continue
		}

		if r.TTL == 0 {
			r.TTL = 120 * time.Second
		}

		body := fmt.Sprintf(`{"type":"TXT","name":"%s","content":"%s","ttl":%d}`,
			r.Name, r.Text, int(r.TTL.Seconds()))

		urlPath := fmt.Sprintf("/service/%s/dns/record", p.ServiceID)
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

		time.Sleep(1 * time.Second)

		allRecs, err := p.GetRecords(ctx, zone)
		if err == nil {
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
		r, ok := rec.(*libdns.TXT)
		if !ok {
			continue
		}

		id, ok := r.ProviderData.(string)
		if !ok || id == "" {
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

// GetRecords is implemented in the original provider package file.
