package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os/exec"
	"strings"
)

// StartWebLog starts a simple HTTP server for viewing service logs and status.
func StartWebLog(cfg *Config) {
	if !cfg.Web.Enabled || cfg.Web.Port == 0 {
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/api/logs", handleLogs)
	mux.HandleFunc("/api/status", handleStatus)

	addr := fmt.Sprintf(":%d", cfg.Web.Port)
	slog.Info("web log viewer starting", "addr", addr)

	go func() {
		if err := http.ListenAndServe(addr, mux); err != nil {
			slog.Error("web server failed", "err", err)
		}
	}()
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	html := strings.Replace(indexHTML, "{{VERSION}}", version, 1)
	w.Write([]byte(html))
}

func handleLogs(w http.ResponseWriter, r *http.Request) {
	lines := r.URL.Query().Get("lines")
	if lines == "" {
		lines = "100"
	}

	out, err := exec.Command("journalctl", "-u", "kfsSms", "--no-pager", "-n", lines, "--output=short-iso").Output()
	if err != nil {
		http.Error(w, "journalctl failed: "+err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write(out)
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	out, err := exec.Command("systemctl", "status", "kfsSms", "--no-pager").Output()
	if err != nil {
		// systemctl status returns non-zero for inactive services, still show output
		if len(out) > 0 {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Write(out)
			return
		}
		http.Error(w, "status failed: "+err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write(out)
}

var indexHTML = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>kfsSms Log Viewer</title>
<style>
  body { font-family: monospace; background: #1e1e2e; color: #cdd6f4; margin: 20px; }
  h1 { color: #89b4fa; }
  .info { color: #a6adc8; margin-bottom: 10px; }
  pre { background: #11111b; padding: 15px; border-radius: 8px; overflow-x: auto;
        max-height: 80vh; overflow-y: auto; font-size: 13px; line-height: 1.4; }
  .controls { margin: 15px 0; }
  button, select { background: #313244; color: #cdd6f4; border: 1px solid #45475a;
                   padding: 6px 12px; border-radius: 4px; cursor: pointer; margin-right: 8px; }
  button:hover { background: #45475a; }
  .ok { color: #a6e3a1; }
  .warn { color: #f9e2af; }
  .err { color: #f38ba8; }
</style>
</head>
<body>
<h1>kfsSms Log Viewer</h1>
<div class="info">Version: {{VERSION}}</div>
<div class="controls">
  <select id="lines">
    <option value="50">50 lines</option>
    <option value="100" selected>100 lines</option>
    <option value="300">300 lines</option>
    <option value="1000">1000 lines</option>
  </select>
  <button onclick="loadLogs()">Refresh</button>
  <button onclick="toggleAuto()">Auto-refresh: <span id="autoState">OFF</span></button>
  <button onclick="loadStatus()">Service Status</button>
</div>
<pre id="output">Loading...</pre>
<script>
let autoInterval = null;
function loadLogs() {
  const n = document.getElementById('lines').value;
  fetch('/api/logs?lines=' + n)
    .then(r => r.text())
    .then(t => {
      const el = document.getElementById('output');
      el.innerHTML = colorize(t);
      el.scrollTop = el.scrollHeight;
    });
}
function loadStatus() {
  fetch('/api/status').then(r => r.text()).then(t => {
    document.getElementById('output').innerHTML = colorize(t);
  });
}
function colorize(text) {
  return text.split('\n').map(line => {
    if (line.match(/ERROR|error|Error|FATAL/)) return '<span class="err">' + esc(line) + '</span>';
    if (line.match(/WARN|warn|Warn/)) return '<span class="warn">' + esc(line) + '</span>';
    if (line.match(/INFO.*SMS sent|INFO.*modem ready|INFO.*started/)) return '<span class="ok">' + esc(line) + '</span>';
    return esc(line);
  }).join('\n');
}
function esc(s) { return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;'); }
function toggleAuto() {
  if (autoInterval) { clearInterval(autoInterval); autoInterval = null; }
  else { autoInterval = setInterval(loadLogs, 5000); }
  document.getElementById('autoState').textContent = autoInterval ? 'ON (5s)' : 'OFF';
}
loadLogs();
</script>
</body>
</html>`
