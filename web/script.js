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
    const files = Array.from(fileList).filter(file =>
      file.name.toLowerCase().endsWith('.pdf')
    );

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
        const blob = generateExcelBlob(allJsonResults, allFilenames);
        triggerDownload(blob, "invoice.xlsx");
        logStatus("Done. Download ready.");
      } else {
        const blob = generateExcelBlob(allJsonResults, allFilenames);
        triggerDownload(blob, "all_invoices.xlsx");
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

    return JSON.parse(text);
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

  function generateExcelBlob(dataArray, filenames) {
    const wsData = [];
    const headerSet = new Set(["Filename"]);

    dataArray.forEach(data => {
      Object.keys(data).forEach(k => headerSet.add(k));
    });

    const headers = Array.from(headerSet);
    wsData.push(headers);

    dataArray.forEach((data, i) => {
      const row = headers.map(h => {
        if (h === "Filename") return filenames[i];
        return data[h] || '';
      });
      wsData.push(row);
    });

    const ws = XLSX.utils.aoa_to_sheet(wsData);

    headers.forEach((header, colIdx) => {
      const isDateColumn = header.toLowerCase().includes('date');
      const isAmountColumn = ["amount", "total", "price"].some(k => header.toLowerCase().includes(k));

      for (let rowIdx = 1; rowIdx < wsData.length; rowIdx++) {
        const cellAddress = XLSX.utils.encode_cell({ r: rowIdx, c: colIdx });
        const cell = ws[cellAddress];
        if (!cell) continue;

        const rawValue = cell.v;

        if (isDateColumn && typeof rawValue === 'string') {
          let parsedDate = null;

          const ddmmyyyyMatch = rawValue.match(/^(\d{2})\.(\d{2})\.(\d{4})$/);
          if (ddmmyyyyMatch) {
            const [_, d, m, y] = ddmmyyyyMatch;
            parsedDate = new Date(`${y}-${m}-${d}`);
          } else {
            parsedDate = new Date(rawValue);
          }

          if (!isNaN(parsedDate)) {
            cell.v = parsedDate;
            cell.t = 'd';
            cell.z = XLSX.SSF._table[14]; // m/d/yy
          }
        }

        if (isAmountColumn && typeof rawValue === 'string') {
          const num = parseFloat(rawValue.replace(/[^\d.-]/g, ''));
          if (!isNaN(num)) {
            cell.v = num;
            cell.t = 'n';
          }
        }
      }
    });

    const wb = XLSX.utils.book_new();
    XLSX.utils.book_append_sheet(wb, ws, "Invoices");

    const wbout = XLSX.write(wb, { bookType: 'xlsx', type: 'array' });
    return new Blob([wbout], {
      type: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
    });
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

  dropArea.addEventListener('drop', async (e) => {
    e.preventDefault();
    dropArea.classList.remove('border-indigo-500');

    const items = e.dataTransfer.items;
    if (!items || items.length === 0) {
      showError("No items dropped.");
      return;
    }

    try {
      const files = await getAllFilesFromItems(items);
      processFiles(files);
    } catch (err) {
      showError("Failed to read dropped files: " + err.message);
    }
  });

  // Helper to recursively read dropped folders
  async function getAllFilesFromItems(items) {
    const filePromises = [];

    for (const item of items) {
      const entry = item.webkitGetAsEntry?.();
      if (entry) {
        filePromises.push(...(await traverseFileTree(entry)));
      }
    }

    const allFiles = await Promise.all(filePromises);
    return allFiles.filter(f => f);
  }

  function traverseFileTree(item, path = '') {
    return new Promise((resolve) => {
      if (item.isFile) {
        item.file(file => resolve([file]), () => resolve([]));
      } else if (item.isDirectory) {
        const dirReader = item.createReader();
        const entries = [];

        const readEntries = () => {
          dirReader.readEntries(async results => {
            if (!results.length) {
              const promises = entries.map(entry => traverseFileTree(entry, path + item.name + "/"));
              const nestedFiles = await Promise.all(promises);
              resolve(nestedFiles.flat());
            } else {
              entries.push(...results);
              readEntries();
            }
          }, () => resolve([]));
        };

        readEntries();
      } else {
        resolve([]);
      }
    });
  }
});

