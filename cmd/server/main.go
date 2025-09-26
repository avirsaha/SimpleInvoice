// Package main is the entry point for the invoice extraction web service.
// It sets up a secure, concurrent, and production-ready HTTP server
// that exposes an API for uploading and processing PDF invoices.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"os/exec"
	"runtime"
	"syscall"
	"time"

	"github.com/avirsaha/SimpleInvoice/tree/stable-go/internal/extractor"

	"golang.org/x/time/rate"
)

// api holds application-wide dependencies like the logger and configuration.
type api struct {
	logger    *slog.Logger
	limiter   *rate.Limiter
	semaphore chan struct{} // Used to limit concurrent extractions.
}

// maxConcurrentExtractions defines how many PDF extractions can run at the same time.
// This prevents the server from being overwhelmed by spawning too many Python processes.
const maxConcurrentExtractions = 10

// NewAPI initializes and returns a new api struct with all dependencies.
func NewAPI(logger *slog.Logger) *api {
	return &api{
		logger:    logger,
		limiter:   rate.NewLimiter(rate.Limit(100), 20), // Allow 2 req/sec with a burst of 5.
		semaphore: make(chan struct{}, maxConcurrentExtractions),
	}
}

// routes sets up the application's router with all the necessary handlers and middleware.
func (app *api) routes() http.Handler {
	mux := http.NewServeMux()

	// Serve index.html at root
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			// Try to serve static files from ./web
			http.ServeFile(w, r, "./web"+r.URL.Path)
			return
		}
		http.ServeFile(w, r, "./web/index.html")
	})

	// API endpoints
	mux.HandleFunc("/health", app.healthCheckHandler)
	mux.Handle("/extract/", app.rateLimit(http.HandlerFunc(app.extractHandler)))

	return mux
}

// writeJSON is a helper for sending structured JSON responses to the client.
func (app *api) writeJSON(w http.ResponseWriter, status int, data any, headers http.Header) error {
	js, err := json.Marshal(data)
	if err != nil {
		return err
	}
	js = append(js, '\n') // Append a newline for easier parsing in terminals.

	for key, value := range headers {
		w.Header()[key] = value
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, err = w.Write(js)
	return err
}

// errorResponse is a helper for sending consistent, structured error messages.
func (app *api) errorResponse(w http.ResponseWriter, r *http.Request, status int, message any) {
	errPayload := map[string]any{"error": message}
	if err := app.writeJSON(w, status, errPayload, nil); err != nil {
		app.logger.Error("failed to write error json response", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// rateLimit is a middleware that checks if a request is allowed by the rate limiter.
func (app *api) rateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !app.limiter.Allow() {
			app.errorResponse(w, r, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// healthCheckHandler provides a simple health check endpoint for monitoring.
func (app *api) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	healthInfo := map[string]string{
		"status":      "available",
		"environment": "development",
		"version":     "1.0.0",
	}
	if err := app.writeJSON(w, http.StatusOK, healthInfo, nil); err != nil {
		app.logger.Error("failed to write health check response", "error", err)
		app.errorResponse(w, r, http.StatusInternalServerError, "server error")
	}
}

// extractHandler handles the primary logic of file upload and data extraction.
// It is wrapped with concurrency controls to ensure server stability.
func (app *api) extractHandler(w http.ResponseWriter, r *http.Request) {
	// Acquire a slot from the semaphore. This will block if all slots are in use,
	// providing a natural backpressure mechanism.
	app.semaphore <- struct{}{}
	// Defer releasing the slot so it's always freed when the function returns.
	defer func() { <-app.semaphore }()

	// 1. Parse multipart form with a reasonable limit (e.g., 10MB).
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		app.errorResponse(w, r, http.StatusBadRequest, "could not parse multipart form: "+err.Error())
		return
	}

	// 2. Retrieve the file from the form data.
	file, handler, err := r.FormFile("file")
	if err != nil {
		app.errorResponse(w, r, http.StatusBadRequest, "error retrieving the file from form-data")
		return
	}
	defer file.Close()

	app.logger.Info("processing file", "filename", handler.Filename, "size_bytes", handler.Size)

	// 3. Pass the file to the extractor logic.
	details, err := extractor.ExtractDetails(file)
	if err != nil {
		app.logger.Error("extraction failed", "error", err, "filename", handler.Filename)
		app.errorResponse(w, r, http.StatusInternalServerError, "failed to extract details from PDF")
		return
	}

	// 4. Send the successful JSON response.
	app.logger.Info("extraction successful", "filename", handler.Filename)
	if err := app.writeJSON(w, http.StatusOK, details, nil); err != nil {
		app.logger.Error("failed to write successful json response", "error", err)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow all origins (for development only)
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}


func openBrowser(url string) error {
    var cmd string
    var args []string

    switch runtime.GOOS {
    case "windows":
        cmd = "rundll32"
        args = []string{"url.dll,FileProtocolHandler", url}
    case "darwin":
        cmd = "open"
        args = []string{url}
    default: // Linux and other unix
        cmd = "xdg-open"
        args = []string{url}
    }

    return exec.Command(cmd, args...).Start()
}
func main() {
	// Use Go's new structured logger for machine-readable logs, essential for production.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	app := NewAPI(logger)

	// --- Production-Ready Server Configuration ---
	srv := &http.Server{
		Addr:         ":8000",
		Handler: corsMiddleware(app.routes()), // CORS enabled
		IdleTimeout:  time.Minute,      // Prevents slow-loris attacks.
		ReadTimeout:  10 * time.Second, // Max time to read request headers/body.
		WriteTimeout: 30 * time.Second, // Max time to write response.
	}

	// --- Graceful Shutdown Logic ---
	shutdownError := make(chan error)

	go func() {
		// Listen for interrupt signals (like Ctrl+C).
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		s := <-quit

		logger.Info("shutting down server", "signal", s.String())

		// Give active requests a deadline to finish.
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		// Attempt to gracefully shut down the server.
		if err := srv.Shutdown(ctx); err != nil {
			shutdownError <- err
		}

		logger.Info("completing background tasks")
		shutdownError <- nil
	}()

	logger.Info("starting server", "addr", srv.Addr)

	
    // Open browser in a goroutine after a tiny delay (to ensure server is ready)
    go func() {
        time.Sleep(500 * time.Millisecond) // give server a moment to start
        url := "http://localhost" + srv.Addr
        if err := openBrowser(url); err != nil {
            logger.Error("failed to open browser", "error", err)
        } else {
            logger.Info("opened browser", "url", url)
        }
    }()

    // Also log the URL so user can click or copy-paste
    logger.Info("web interface available at", "url", "http://localhost"+srv.Addr)

	// Start the server. This is a blocking call.
	err := srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server failed to start", "error", err)
		os.Exit(1)
	}

	// Wait for the shutdown process to complete.
	if err := <-shutdownError; err != nil {
		logger.Error("error during shutdown", "error", err)
		os.Exit(1)
	}

	logger.Info("server stopped gracefully")
}

