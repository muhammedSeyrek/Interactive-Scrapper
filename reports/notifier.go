package reports

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// SlackMessage defines the JSON structure expected by Slack API
type SlackMessage struct {
	Text string `json:"text"`
}

// SendSlackAlert sends a detailed alert to Slack.
// Now accepts a 'details' string to include intelligence reports (MITRE, VT).
func SendSlackAlert(webhookURL string, targetURL string, riskScore int, newEntitiesCount int, details string) error {

	// Choose an icon based on the risk level
	icon := "⚠️"
	if riskScore >= 8 {
		icon = "🚨 CRITICAL"
	} else if riskScore >= 5 {
		icon = "🔥 HIGH"
	}

	// Construct the detailed message body
	messageBody := fmt.Sprintf(
		"%s **DarkWatch Alert!**\n"+
			"**Target:** %s\n"+
			"**Risk Score:** %d/10\n"+
			"**New Entities:** %d\n"+
			"--------------------------------------\n"+
			"**🔍 INTELLIGENCE REPORT:**\n%s\n"+
			"--------------------------------------\n"+
			"**Time:** %s",
		icon, targetURL, riskScore, newEntitiesCount, details, time.Now().Format("15:04:05"),
	)

	// Create JSON payload
	payload := SlackMessage{
		Text: messageBody,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Send POST request to Slack
	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("Slack API returned error: %d", resp.StatusCode)
	}

	return nil
}
