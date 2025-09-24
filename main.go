// main.go
package main

import (
	"encoding/json"
	"log"
	"net/http"

	"simple_invoice/internal/extractor"
)

// setupCORS enables Cross-Origin Resource Sharing for the frontend.
func setupCORS(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-control-allow-methods", "POST, GET, OPTIONS")
	(*w).Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
}

// extractHandler handles the file upload and extraction process.
func extractHandler(w http.ResponseWriter, r *http.Request) {
	// Set up CORS for preflight and actual requests.
	setupCORS(&w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	log.Println("Received request for /extract")

	// 1. Parse the multipart form (the uploaded file)
	// 10 << 20 specifies a maximum upload of 10 MB.
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Could not parse multipart form: "+err.Error(), http.StatusBadRequest)
		return
	}

	// 2. Retrieve the file from the form data
	file, handler, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Error retrieving the file from form-data: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	log.Printf("Processing file: %s (Size: %d bytes)", handler.Filename, handler.Size)

	// 3. Pass the file to the extractor logic
	details, err := extractor.ExtractDetails(file)
	if err != nil {
		log.Printf("Error during extraction: %v", err)
		http.Error(w, "Failed to extract details from PDF: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 4. Send the successful JSON response back to the client
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(details); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
		// If encoding fails, the connection might be closed, but we try to send an error.
		http.Error(w, "Failed to encode response to JSON", http.StatusInternalServerError)
	}
	log.Println("Successfully processed and sent response.")
}

// main is the entry point of the application.
func main() {
	// Define the HTTP routes
	http.HandleFunc("/extract/", extractHandler)

	// Serve the frontend HTML file at the root
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	// Start the server
	port := "8000" // Changed to 8000 to match the frontend JS
	log.Printf("Starting server on http://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Could not start server: %s\n", err)
	}
}
