import io
from fastapi import FastAPI, File, UploadFile, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from extractor import extract_invoice_details_from_stream
import logging

# --- Setup Logging ---
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Initialize FastAPI app
app = FastAPI(
    title="Simple Invoice Extractor API (Regex)",
    description="An API using regular expressions to extract structured data from fixed-format invoice PDFs.",
    version="1.1.0",
)

# --- CORS Middleware ---
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

@app.get("/")
def read_root():
    """A simple root endpoint to confirm the API is running."""
    return {"message": "Welcome to the Simple Regex-Powered Invoice Extractor API."}

@app.post("/extract/")
async def extract_data_from_pdf(file: UploadFile = File(...)):
    """
    Receives a PDF, processes it with a regex function, and returns the data.
    """
    if file.content_type != 'application/pdf':
        raise HTTPException(status_code=400, detail="Invalid file type. Please upload a PDF.")

    try:
        pdf_content = await file.read()
        
        logger.info(f"Processing file: {file.filename}")
        # Call the rule-based extraction function from our extractor module
        extracted_data = extract_invoice_details_from_stream(io.BytesIO(pdf_content))
        logger.info(f"Successfully extracted data for file: {file.filename}")

        if "error" in extracted_data:
            raise HTTPException(status_code=500, detail=extracted_data["error"])
            
        return extracted_data

    except Exception as e:
        logger.error(f"An unexpected error occurred during extraction: {e}")
        raise HTTPException(status_code=500, detail=f"An unexpected error occurred: {str(e)}")


