// Package extractor provides the core logic for parsing invoice details from a PDF.
// It uses an external Python script to extract text in different layouts
// and then applies regular expressions to parse the structured data.
package extractor

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"encoding/json"  // Debug
)

// InvoiceDetails holds the structured data extracted from the PDF.
// Each field is tagged for JSON serialization.
type InvoiceDetails struct {
	InvoiceNumber  string `json:"invoice_number"`
	InvoiceDate    string `json:"invoice_date"`
	OrderNumber    string `json:"order_number"`
	OrderDate      string `json:"order_date"`
	BillingName    string `json:"billing_name"`
	BillingAddress string `json:"billing_address"`
	StateCode      string `json:"state_code"`
	GSTNOClient    string `json:"gst_no_client"` // The client's GST number, if provided.
	TaxAmount      string `json:"tax_amount"`
	TotalAmount    string `json:"total_amount"`
	HSN            string `json:"hsn"`
	ASN            string `json:"asn"` // A unique product or item code.
}

// sellerGSTIN is the GST number of the seller, used to avoid misattributing it to the client.
const sellerGSTIN = "19APGPS1824K1ZI"

// pre-compiled regular expressions for efficient matching.
var (
	reInvoiceNumber = regexp.MustCompile(`(?i)Invoice\s*Number\s*[:\-]?\s*(\S+)`)
	reInvoiceDate = regexp.MustCompile(`(?i)Invoice\s*Date\s*[:\-]?\s*([0-9]{2}[./-][0-9]{2}[./-][0-9]{4})`)
	reOrderNo      = regexp.MustCompile(`(?i)Order\s*Number\s*[:\-]?\s*([A-Z0-9\-]+)`)
	reOrderDate    = regexp.MustCompile(`(?i)Order\s*Date\s*[:\-]?\s*([0-9]{2}[./-][0-9]{2}[./-][0-9]{4})`)
	reStateCode    = regexp.MustCompile(`(?i)State/UT\s*Code\s*[:\-]?\s*(\d{2})`)
	reGST          = regexp.MustCompile(`(?i)GST(?:IN)?(?: Registration)? No\s*[:\-]?\s*(\S+)`)
	reTaxAndTotal  = regexp.MustCompile(`(?i)TOTAL\s*[:\-]?\s*.*?([\d,]+\.\d{2})\s*.*?([\d,]+\.\d{2})`)
	reHSN          = regexp.MustCompile(`(?i)HSN\s*[:\-]?\s*(\d+)`)
	reASN          = regexp.MustCompile(`[\|\s]+([A-Z0-9]{10})[\s]*(\(|₹)`)
	reBillingBlock = regexp.MustCompile(`(?is)Billing Address\s*:\s*(.*?)\s*(?:Shipping Address|Invoice Number|State/UT Code)`)
)

// ExtractDetails is the primary function of the package. It takes a reader for a PDF file,
// orchestrates the text extraction via a Python script, and then parses the text
// to populate an InvoiceDetails struct.
func ExtractDetails(file io.Reader) (*InvoiceDetails, error) {
	// Buffer the reader content to allow it to be read multiple times.
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, file); err != nil {
		return nil, fmt.Errorf("failed to buffer pdf content: %w", err)
	}

	// Extract text using the Python script in two different layout modes.
	simpleText, err := extractTextWithPython(bytes.NewReader(buf.Bytes()), "simple")
	if err != nil {
		return nil, err
	}
	columnText, err := extractTextWithPython(bytes.NewReader(buf.Bytes()), "columns")
	if err != nil {
		return nil, err
	}
		//  DEBUG: Print the raw extracted text
	// fmt.Println("----- SIMPLE TEXT -----")
	// fmt.Println(simpleText)

	// fmt.Println("----- COLUMN TEXT -----")
	// fmt.Println(columnText)

	details := &InvoiceDetails{}

	// --- Parse simple, single-line fields from the 'simple' text layout ---
	details.InvoiceNumber = findStringSubmatchAndClean(reInvoiceNumber, simpleText, 1)
	details.InvoiceDate = findStringSubmatchAndClean(reInvoiceDate, simpleText, 1)
	details.OrderNumber = findStringSubmatchAndClean(reOrderNo, simpleText, 1)
	details.OrderDate = findStringSubmatchAndClean(reOrderDate, simpleText, 1)
	details.StateCode = findStringSubmatchAndClean(reStateCode, simpleText, 1)
	details.HSN = findStringSubmatchAndClean(reHSN, simpleText, 1)
	details.ASN = findStringSubmatchAndClean(reASN, simpleText, 1)

	// Extract Tax and Total amounts from the "TOTAL" line.
	if match := reTaxAndTotal.FindStringSubmatch(simpleText); len(match) >= 3 {
		details.TaxAmount = strings.TrimSpace(match[1])
		details.TotalAmount = strings.TrimSpace(match[2])
	}

	// --- Parse the multi-line billing block from the 'columns' text layout ---
	if billingBlockMatch := reBillingBlock.FindStringSubmatch(columnText); len(billingBlockMatch) > 1 {
		billingBlockText := billingBlockMatch[1]
		name, address, gst := parseBillingBlock(billingBlockText)
		details.BillingName = name
		details.BillingAddress = address
		// Avoid capturing the seller's GST as the client's.
		if !strings.EqualFold(gst, sellerGSTIN) {
			details.GSTNOClient = gst
		}
	}
	// DEBUG: Print the extracted details as JSON
	jsonData, err := json.MarshalIndent(details, "", "  ")
	if err != nil {
		fmt.Println("Failed to marshal details to JSON:", err)
	} else {
		fmt.Println("Extracted InvoiceDetails (JSON):")
		fmt.Println(string(jsonData))
	}

	return details, nil
}

// extractTextWithPython securely executes an external Python script to extract text from a PDF.
// It creates a temporary file for the PDF content and passes its path to the script.
// It returns the script's stdout or an error containing stderr for easier debugging.
//
// Parameters:
//   - reader: An io.Reader providing the PDF file content.
//   - mode: The extraction mode ('simple' or 'columns') to pass to the Python script.
func extractTextWithPython(reader io.Reader, mode string) (string, error) {
	// Create a temporary file to hold the PDF content. This is safer than passing raw bytes.
	tmpFile, err := os.CreateTemp("", "invoice-*.pdf")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	// Ensure the temporary file is cleaned up regardless of success or failure.
	defer os.Remove(tmpFile.Name())

	if _, err := io.Copy(tmpFile, reader); err != nil {
		return "", fmt.Errorf("failed to write to temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close temp file: %w", err)
	}

	// Sanitize the script path to prevent directory traversal vulnerabilities.
	scriptPath, err := filepath.Abs(filepath.Join("tools", "pdf_text_extractor.py"))
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute script path: %w", err)
	}

	cmd := exec.Command("./tools/venv/bin/python3", scriptPath, tmpFile.Name(), "--mode="+mode)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr // Capture stderr for better error reporting.

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("python script failed (mode: %s): %w. Stderr: %s", mode, err, stderr.String())
	}

	return out.String(), nil
}

// parseBillingBlock takes the raw text of the billing address section and extracts
// the name, full address, and the client's GST number (if present).
func parseBillingBlock(blockText string) (name, address, gst string) {
	lines := strings.Split(blockText, "\n")
	var addressParts []string
	foundAddressEnd := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check for the client's GSTIN, which can appear within the address block.
		if strings.Contains(strings.ToLower(line), "gst registration no") {
			gst = findStringSubmatchAndClean(reGST, line, 1)
			continue // Don't include the GST line in the address itself.
		}

		// The address often ends with a country code like "IN" or "CA".
		// We stop capturing lines after we find it.
		if !foundAddressEnd {
			addressParts = append(addressParts, line)
			if line == "IN" || line == "CA" {
				foundAddressEnd = true
			}
		}
	}

	if len(addressParts) > 0 {
		name = addressParts[0]
	}
	if len(addressParts) > 1 {
		address = strings.Join(addressParts[1:], ", ")
	}

	return name, address, gst
}

// findStringSubmatchAndClean is a helper function that applies a regex to a text,
// extracts a specific capture group, and cleans up whitespace.
func findStringSubmatchAndClean(re *regexp.Regexp, text string, group int) string {
	match := re.FindStringSubmatch(text)
	if len(match) > group {
		// Replace newlines and multiple spaces with a single space for consistency.
		cleaned := strings.ReplaceAll(match[group], "\n", " ")
		cleaned = regexp.MustCompile(`\s+`).ReplaceAllString(cleaned, " ")
		return strings.TrimSpace(cleaned)
	}
	return "" // Return an empty string if no match is found.
}


