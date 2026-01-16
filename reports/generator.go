package reports

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"interactive-scraper/models"
	"strings"

	"github.com/go-pdf/fpdf"
)

func GenerateJSON(content *models.DarkWebContent) ([]byte, error) {
	return json.MarshalIndent(content, "", "  ")
}

func GeneratePDF(content *models.DarkWebContent) (*bytes.Buffer, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	pdf.SetFont("Arial", "B", 20)
	pdf.SetTextColor(15, 98, 254)
	pdf.Cell(0, 10, "Dark Web Threat Intelligence Report")
	pdf.Ln(12)

	pdf.SetFillColor(240, 240, 240)
	pdf.Rect(10, 22, 190, 40, "F")

	pdf.SetFont("Arial", "B", 10)
	pdf.SetTextColor(0, 0, 0)

	pdf.SetXY(12, 25)
	pdf.Cell(0, 10, fmt.Sprintf("Report ID: #%d", content.ID))
	pdf.SetXY(120, 25)
	pdf.Cell(0, 10, fmt.Sprintf("Date: %s", content.PublishedDate.Format("2006-01-02 15:04")))

	pdf.SetXY(12, 32)
	pdf.SetFont("Courier", "", 9)
	pdf.Cell(0, 10, fmt.Sprintf("Source: %s", content.SourceURL))

	pdf.SetXY(12, 40)
	pdf.SetFont("Arial", "B", 12)
	riskColor := []int{66, 190, 101}
	if content.CriticalityScore >= 8 {
		riskColor = []int{250, 77, 86}
	} else if content.CriticalityScore >= 5 {
		riskColor = []int{241, 194, 27}
	}
	pdf.SetTextColor(riskColor[0], riskColor[1], riskColor[2])
	pdf.Cell(0, 10, fmt.Sprintf("Risk Score: %d/10 (%s)", content.CriticalityScore, content.Category))

	pdf.Ln(25)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Arial", "B", 14)
	pdf.Cell(0, 10, "Detected Threats / Entities:")
	pdf.Ln(8)

	pdf.SetFont("Courier", "", 10)
	pdf.SetFillColor(30, 30, 30)
	pdf.SetTextColor(255, 255, 255)

	findings := strings.Split(content.Matches, "|")
	for _, finding := range findings {
		cleanFinding := strings.TrimSpace(finding)
		if cleanFinding != "" {
			pdf.CellFormat(0, 8, " > "+cleanFinding, "0", 1, "", true, 0, "")
		}
	}

	pdf.Ln(5)

	if content.Screenshot != "" {
		pdf.SetTextColor(0, 0, 0)
		pdf.SetFont("Arial", "B", 14)
		pdf.Cell(0, 10, "Evidence (Screenshot):")
		pdf.Ln(10)

		// Base64 decode
		imgData, err := base64.StdEncoding.DecodeString(content.Screenshot)
		if err == nil {
			rdr := bytes.NewReader(imgData)
			pdf.RegisterImageOptionsReader("screenshot", fpdf.ImageOptions{ImageType: "PNG"}, rdr)

			pdf.Image("screenshot", 10, pdf.GetY(), 190, 0, false, "", 0, "")
		} else {
			pdf.SetFont("Arial", "I", 10)
			pdf.Cell(0, 10, "Error loading screenshot.")
		}
	}

	// Buffer'a yaz
	var buf bytes.Buffer
	err := pdf.Output(&buf)
	return &buf, err
}
