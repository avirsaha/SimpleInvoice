package extractor

import (
	"bytes"
	"io"
	"regexp"
	"strings"

	"github.com/ledongthuc/pdf"
)

// InvoiceDetails holds the structured data extracted from the PDF.
// The `json:"..."` tags define the keys in the JSON response.
type InvoiceDetails struct {
	GSTNo           string `json:"gst_no"`
	OrderNumber     string `json:"order_number"`
	InvoiceNumber   string `json:"invoice_number"`
	SoldBy          string `json:"sold_by"`
	ShippingAddress string `json:"shipping_address"`
	BillingAddress  string `json:"billing_address"`
}

// readPDFText extracts raw text content from a PDF file.
func readPDFText(reader io.Reader) (string, error) {
	// The PDF reader needs the file size, so we read it into a buffer first.
	buf := new(bytes.Buffer)
	size, err := buf.ReadFrom(reader)
	if err != nil {
		return "", err
	}

	// pdf.NewReader expects an io.ReaderAt, which bytes.Reader provides.
	r := bytes.NewReader(buf.Bytes())
	pdfReader, err := pdf.NewReader(r, size)
	if err != nil {
		return "", err
	}

	var textBuilder strings.Builder
	numPages := pdfReader.NumPage()
	// We only care about the second page for this specific invoice format.
	if numPages > 0 {
		page := pdfReader.Page(2)
		if page.V.IsNull() {
			return "", nil
		}
		rows, err := page.GetTextByRow()
		if err != nil {
			return "", err
		}
		// Concatenate all text rows into a single string.
		for _, row := range rows {
			for _, word := range row.Content {
				textBuilder.WriteString(word.S + " ")
			}
			textBuilder.WriteString("\n")
		}
	}

	return textBuilder.String(), nil
}

// ExtractDetails is the main function that orchestrates the PDF text extraction and regex parsing.
func ExtractDetails(file io.Reader) (*InvoiceDetails, error) {
	text, err := readPDFText(file)
	if err != nil {
		return nil, err
	}

	details := &InvoiceDetails{}

	// Compile regex patterns. Compiling them once is more efficient.
	// `(?is)` flag: i = case-insensitive, s = dot matches newline.
	reGST := regexp.MustCompile(`(?i)GST Registration No\s*:\s*(\S+)`)
	reOrder := regexp.MustCompile(`(?i)Order Number\s*:\s*([0-9\-]+)`)
	reInvoice := regexp.MustCompile(`(?i)Invoice Number\s*:\s*(\S+)`)
	reSoldBy := regexp.MustCompile(`(?is)Sold By\s*:\s*(.*?)\s*Billing Address\s*:`)
	reShipping := regexp.MustCompile(`(?is)Shipping Address\s*:\s*(.*?)\s*State/UT Code\s*:`)
	reBilling := regexp.MustCompile(`(?is)Billing Address\s*:\s*(.*?)\s*PAN No\s*:`)

	// Helper function to find a match and clean up the result.
	findAndClean := func(re *regexp.Regexp, text string) string {
		match := re.FindStringSubmatch(text)
		if len(match) > 1 {
			// Replace newlines and multiple spaces with a single space.
			cleaned := strings.ReplaceAll(match[1], "\n", " ")
			cleaned = regexp.MustCompile(`\s+`).ReplaceAllString(cleaned, " ")
			return strings.TrimSpace(cleaned)
		}
		return ""
	}

	// Apply each regex pattern to the extracted text.
	details.GSTNo = findAndClean(reGST, text)
	details.OrderNumber = findAndClean(reOrder, text)
	details.InvoiceNumber = findAndClean(reInvoice, text)
	details.SoldBy = findAndClean(reSoldBy, text)
	details.ShippingAddress = findAndClean(reShipping, text)
	details.BillingAddress = findAndClean(reBilling, text)

	return details, nil
}
