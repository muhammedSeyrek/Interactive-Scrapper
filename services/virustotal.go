package services

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	vtIpURL  = "https://www.virustotal.com/api/v3/ip_addresses/"
	vtUrlURL = "https://www.virustotal.com/api/v3/urls/"
)

type VTResponse struct {
	Data struct {
		Attributes struct {
			LastAnalysisStats struct {
				Malicious  int `json:"malicious"`
				Suspicious int `json:"suspicious"`
			} `json:"last_analysis_stats"`
			Country string `json:"country"` // Sadece IP için döner
			Title   string `json:"title"`   // Sadece URL için döner
			ASOwner string `json:"as_owner"`
		} `json:"attributes"`
	} `json:"data"`
}

func CheckIPReputation(ip string, apiKey string) (string, int, error) {
	return performVTRequest(vtIpURL+ip, apiKey)
}

func CheckURLReputation(targetUrl string, apiKey string) (string, int, error) {
	id := base64.RawURLEncoding.EncodeToString([]byte(targetUrl))
	return performVTRequest(vtUrlURL+id, apiKey)
}

func performVTRequest(requestURL string, apiKey string) (string, int, error) {
	if apiKey == "" {
		return "No API Key", 0, nil
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return "", 0, err
	}

	req.Header.Add("x-apikey", apiKey)
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return "Not Found / Clean", 0, nil
	}
	if resp.StatusCode != 200 {
		return "VT Error", 0, fmt.Errorf("VT Status: %d", resp.StatusCode)
	}

	var result VTResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", 0, err
	}

	stats := result.Data.Attributes.LastAnalysisStats
	maliciousCount := stats.Malicious

	info := result.Data.Attributes.ASOwner
	if info == "" {
		info = result.Data.Attributes.Title
	}

	report := fmt.Sprintf("%s - Malicious: %d", info, maliciousCount)
	return report, maliciousCount, nil
}
