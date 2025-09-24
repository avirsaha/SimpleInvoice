import pdfplumber
import re
import logging

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

def extract_invoice_details_from_stream(file_stream):
    """
    Extracts specific details from a PDF file stream using robust regex.
    This is the core business logic, separate from the API.

    Args:
        file_stream: A file-like object (BytesIO stream) of the PDF content.

    Returns:
        A dictionary containing the extracted invoice details.
    """
    details = {}
    try:
        with pdfplumber.open(file_stream) as pdf:
            if not pdf.pages:
                return {"error": "The PDF document is empty."}
            
            page = pdf.pages[-1]      # Always looks at the last page of the PDF
            text = page.extract_text()

            if not text:
                return {"error": "Could not extract text. The PDF might be image-based."}

            # --- Robust Regex Patterns for Data Extraction ---
            
            # Helper function to clean up multi-line address text
            def clean_address(match):
                if not match:
                    return None
                # Joins all lines into a single line, separated by spaces
                return " ".join(match.group(1).strip().split('\n'))

            # GST No
            gst_match = re.search(r"GST Registration No\s*:\s*(\S+)", text, re.IGNORECASE)
            details["gst_no"] = gst_match.group(1) if gst_match else None

            # Order Number
            order_match = re.search(r"Order Number\s*:\s*([0-9\-]+)", text, re.IGNORECASE)
            details["order_number"] = order_match.group(1).strip() if order_match else None

            # Invoice Number
            invoice_match = re.search(r"Invoice Number\s*:\s*(\S+)", text, re.IGNORECASE)
            details["invoice_number"] = invoice_match.group(1) if invoice_match else None

            # Sold By (multi-line)
            sold_by_match = re.search(r"Sold By\s*:\s*(.*?)\s*Billing Address\s*:", text, re.DOTALL | re.IGNORECASE)
            details["sold_by"] = clean_address(sold_by_match)

            # Shipping Address (multi-line)
            shipping_match = re.search(r"Shipping Address\s*:\s*(.*?)\s*State/UT Code\s*:", text, re.DOTALL | re.IGNORECASE)
            details["shipping_address"] = clean_address(shipping_match)
                
            # Billing Address (multi-line)
            billing_match = re.search(r"Billing Address\s*:\s*(.*?)\s*PAN No\s*:", text, re.DOTALL | re.IGNORECASE)
            details["billing_address"] = clean_address(billing_match)

    except Exception as e:
        logger.error(f"An error occurred during PDF processing: {e}")
        return {"error": f"An error occurred during PDF processing: {str(e)}"}

    return details


