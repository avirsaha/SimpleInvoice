// script.js
document.addEventListener('DOMContentLoaded', () => {
  const API_ENDPOINT = 'http://localhost:8000/extract/';
  const fileInput = document.getElementById('file-input');
  const folderInput = document.getElementById('folder-input');
  const dropArea = document.getElementById('drop-area');
  const statusLog = document.getElementById('status-log');
  const errorMessage = document.getElementById('error-message');
  const resultsContainer = document.getElementById('results-container');
  const resultsTbody = document.getElementById('results-tbody');
  const downloadLink = document.getElementById('download-link');

  function logStatus(message) {
    statusLog.textContent = message;
    statusLog.classList.remove('shown');
    void statusLog.offsetWidth;
    statusLog.classList.add('shown');
  }

  function showError(message) {
    errorMessage.textContent = message;
    errorMessage.classList.remove('hidden');
  }

  function hideError() {
    errorMessage.classList.add('hidden');
    errorMessage.textContent = '';
  }

  function resetUI() {
    logStatus('');
    hideError();
    resultsContainer.classList.add('hidden');
    resultsTbody.innerHTML = '';
    downloadLink.classList.add('hidden');
    downloadLink.href = '';
  }

  async function processFiles(fileList) {
    resetUI();
    const files = Array.from(fileList).filter(file => file.type === "application/pdf");
    if (files.length === 0) {
      showError("No PDF files detected.");
      return;
    }

    logStatus(`Uploading ${files.length} file(s)...`);

    try {
      const promises = files.map((file, i) => uploadSingleFile(file, i + 1, files.length));
      const allJsonResults = await Promise.all(promises);
      const allFilenames = files.map(file => file.name);

      if (files.length === 1) {
        showResultsTable(allJsonResults[0]);
        const blob = generateAggregatedExcelBlob(allJsonResults, allFilenames);
        triggerDownload(blob, "invoice.csv");
        logStatus("Done. Download ready.");
      } else {
        const blob = generateAggregatedExcelBlob(allJsonResults, allFilenames);
        triggerDownload(blob, "all_invoices.csv");
        logStatus("Done. Download ready.");
      }

    } catch (err) {
      showError(err.message || "An error occurred");
    }
  }

  async function uploadSingleFile(file, index, total) {
    logStatus(`Uploading file ${index} of ${total}: ${file.name}...`);
    const formData = new FormData();
    formData.append('file', file);

    const res = await fetch(API_ENDPOINT, {
      method: 'POST',
      body: formData
    });

    const text = await res.text();
    if (!res.ok) {
      throw new Error(`Error processing ${file.name}: ` + (JSON.parse(text).error || text));
    }

    return JSON.parse(text); // Return only the JSON
  }

  function showResultsTable(data) {
    resultsTbody.innerHTML = '';
    for (const [key, val] of Object.entries(data)) {
      const row = document.createElement('tr');
      row.innerHTML = `
        <td class="px-4 py-2 text-sm font-medium text-gray-800">${key.replace(/_/g, ' ')}</td>
        <td class="px-4 py-2 text-sm text-gray-600">${val || 'N/A'}</td>
      `;
      resultsTbody.appendChild(row);
    }
    resultsContainer.classList.remove('hidden');
  }

  function generateAggregatedExcelBlob(dataArray, filenames) {
    const rows = [];
    const headerSet = new Set(["Filename"]);

    dataArray.forEach(data => {
      Object.keys(data).forEach(k => headerSet.add(k));
    });

    const headers = Array.from(headerSet);
    rows.push(headers);

    dataArray.forEach((data, i) => {
      const row = headers.map(h => {
        if (h === "Filename") return filenames[i];
        return data[h] || '';
      });
      rows.push(row);
    });

    const csvContent = rows.map(r => r.map(cell => `"${cell}"`).join(',')).join('\n');
    return new Blob([csvContent], { type: 'text/csv' });
  }

  function triggerDownload(blob, filename) {
    const url = URL.createObjectURL(blob);
    downloadLink.href = url;
    downloadLink.download = filename;
    downloadLink.classList.remove('hidden');
  }

  // Event listeners
  fileInput.addEventListener('change', (e) => processFiles(e.target.files));
  folderInput.addEventListener('change', (e) => processFiles(e.target.files));

  dropArea.addEventListener('dragover', e => {
    e.preventDefault();
    dropArea.classList.add('border-indigo-500');
  });

  dropArea.addEventListener('dragleave', () => {
    dropArea.classList.remove('border-indigo-500');
  });

  dropArea.addEventListener('drop', e => {
    e.preventDefault();
    dropArea.classList.remove('border-indigo-500');
    const files = e.dataTransfer.files;
    processFiles(files);
  });
});

