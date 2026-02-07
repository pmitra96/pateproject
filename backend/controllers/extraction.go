package controllers

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"os"

	"github.com/pmitra96/pateproject/config"
	"github.com/pmitra96/pateproject/extractor"
	"github.com/pmitra96/pateproject/logger"
)

func ExtractItems(w http.ResponseWriter, r *http.Request) {
	logger.Info("Received extraction request")

	// Parse multipart form
	err := r.ParseMultipartForm(10 << 20) // 10 MB limit
	if err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("invoice")
	if err != nil {
		http.Error(w, "Error retrieving file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Save temp file (needed by both extractors)
	tempDir := os.TempDir()
	tempFile, err := os.CreateTemp(tempDir, "upload-*.pdf")
	if err != nil {
		http.Error(w, "Failed to create temp file", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	logger.Info("Saving invoice to temp file", "path", tempFile.Name())
	_, err = io.Copy(tempFile, file)
	if err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	// Try Python extractor first
	pythonURL := config.GetEnv("PYTHON_EXTRACTOR_URL", "http://localhost:8081")
	result, err := callPythonExtractor(pythonURL, tempFile.Name())

	if err != nil {
		logger.Warn("Python extractor failed, falling back to Go extractor", "error", err)

		// Fallback to Go extractor
		result, err = extractor.ParseInvoice(tempFile.Name())
		if err != nil {
			logger.Error("Go extraction also failed", "error", err)
			http.Error(w, "Failed to extract data: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	logger.Info("Extraction completed successfully", "provider", result.Provider, "items_found", len(result.Items))
	for _, item := range result.Items {
		logger.Info("Item found", "name", item.Name, "count", item.Count, "unit_val", item.UnitValue, "unit", item.Unit)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func callPythonExtractor(baseURL string, pdfPath string) (*extractor.ExtractionResult, error) {
	// Open the PDF file
	file, err := os.Open(pdfPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", "invoice.pdf")
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return nil, err
	}

	writer.Close()

	// Make request to Python service
	req, err := http.NewRequest("POST", baseURL+"/extract", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, err
	}

	// Parse response
	var result extractor.ExtractionResult
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}
