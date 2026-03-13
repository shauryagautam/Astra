package middleware

import (
	"html/template"
	"net/http"
	"strings"
	"time"

	astrahttp "github.com/astraframework/astra/http"
)

// ErrorPageData represents the error page structure
type ErrorPageData struct {
	Error      ErrorDetails
	Request    RequestInfo
	Timestamp  time.Time
	AppVersion string
	Debug      DebugInfo
}

type ErrorDetails struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Type    string `json:"type"`
	Stack   string `json:"stack,omitempty"`
}

type RequestInfo struct {
	Method    string            `json:"method"`
	URL       string            `json:"url"`
	Headers   map[string]string `json:"headers"`
	Params    map[string]string `json:"params"`
	Query     map[string]string `json:"query"`
	Body      string            `json:"body,omitempty"`
	IP        string            `json:"ip"`
	UserAgent string            `json:"user_agent"`
}

type DebugInfo struct {
	Stack     string                 `json:"stack"`
	Variables map[string]interface{} `json:"variables"`
	Queries   []QueryInfo            `json:"queries"`
	Context   map[string]interface{} `json:"context"`
}

type QueryInfo struct {
	SQL      string        `json:"sql"`
	Duration time.Duration `json:"duration"`
	Params   []interface{} `json:"params"`
}

// InteractiveErrorPage provides Laravel Ignition-style error pages
func InteractiveErrorPage(appVersion string, debug bool) astrahttp.MiddlewareFunc {
	return func(next astrahttp.HandlerFunc) astrahttp.HandlerFunc {
		return func(c *astrahttp.Context) error {
			// Process the request
			err := next(c)

			// Only handle errors that haven't been processed
			if err == nil {
				return nil
			}

			// Check if this is an API request
			isAPI := strings.HasPrefix(c.Request.Header.Get("Accept"), "application/json") ||
				strings.HasPrefix(c.Request.URL.Path, "/api/") ||
				c.Request.Header.Get("X-Requested-With") == "XMLHttpRequest"

			if isAPI {
				// Return JSON error for API requests
				return handleAPIError(c, err)
			}

			// Return interactive HTML error page for web requests
			return handleHTMLError(c, err, appVersion, debug)
		}
	}
}

func handleAPIError(c *astrahttp.Context, err error) error {
	if httpErr, ok := err.(*astrahttp.HTTPError); ok {
		return c.Error(httpErr.Status, httpErr.Message)
	}

	return c.InternalError("Internal server error")
}

func handleHTMLError(c *astrahttp.Context, err error, appVersion string, debug bool) error {
	var errorDetails ErrorDetails
	var statusCode int

	if httpErr, ok := err.(*astrahttp.HTTPError); ok {
		errorDetails = ErrorDetails{
			Code:    httpErr.Status,
			Message: httpErr.Message,
			Type:    getErrorType(httpErr.Status),
		}
		statusCode = httpErr.Status
	} else {
		errorDetails = ErrorDetails{
			Code:    500,
			Message: "Internal server error",
			Type:    "server_error",
		}
		if debug {
			errorDetails.Stack = string(debug.Stack())
		}
		statusCode = 500
	}

	// Collect request information
	reqInfo := RequestInfo{
		Method:    c.Request.Method,
		URL:       c.Request.URL.String(),
		Headers:   getHeaders(c.Request),
		Params:    getAllParams(c),
		Query:     getAllQuery(c),
		IP:        c.ClientIP(),
		UserAgent: c.Request.Header.Get("User-Agent"),
	}

	// Include debug information if enabled
	var debugInfo DebugInfo
	if debug {
		debugInfo = DebugInfo{
			Stack:     string(debug.Stack()),
			Variables: getContextVariables(c),
			Queries:   getQueryInfo(c),
			Context:   getContextData(c),
		}
	}

	errorData := ErrorPageData{
		Error:      errorDetails,
		Request:    reqInfo,
		Timestamp:  time.Now(),
		AppVersion: appVersion,
		Debug:      debugInfo,
	}

	// Set content type and status
	c.SetHeader("Content-Type", "text/html")
	c.Writer.WriteHeader(statusCode)

	// Render the error page
	return renderErrorPage(c, errorData)
}

func renderErrorPage(c *astrahttp.Context, data ErrorPageData) error {
	tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Error.Code}} - {{.Error.Message}}</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .error-container {
            background: white;
            border-radius: 12px;
            box-shadow: 0 20px 40px rgba(0,0,0,0.1);
            max-width: 800px;
            width: 90%;
            max-height: 80vh;
            overflow-y: auto;
        }
        .error-header {
            background: linear-gradient(135deg, #ff6b6b 0%, #ee5a24 100%);
            color: white;
            padding: 20px;
            border-radius: 12px 12px 0 0;
            display: flex;
            align-items: center;
            justify-content: space-between;
        }
        .error-code {
            font-size: 2rem;
            font-weight: bold;
        }
        .error-message {
            font-size: 1.2rem;
            opacity: 0.9;
        }
        .tabs {
            display: flex;
            background: #f8f9fa;
            border-bottom: 1px solid #dee2e6;
        }
        .tab {
            padding: 15px 20px;
            cursor: pointer;
            border: none;
            background: none;
            font-size: 0.9rem;
            transition: all 0.3s ease;
            border-bottom: 3px solid transparent;
        }
        .tab.active {
            color: #007bff;
            border-bottom-color: #007bff;
            background: white;
        }
        .tab:hover {
            background: #e9ecef;
        }
        .tab-content {
            display: none;
            padding: 20px;
        }
        .tab-content.active {
            display: block;
        }
        .code-block {
            background: #282c34;
            color: #abb2bf;
            padding: 15px;
            border-radius: 6px;
            font-family: 'Monaco', 'Menlo', monospace;
            font-size: 0.85rem;
            overflow-x: auto;
            margin: 10px 0;
        }
        .request-info {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
            gap: 15px;
        }
        .info-item {
            background: #f8f9fa;
            padding: 15px;
            border-radius: 6px;
            border-left: 4px solid #007bff;
        }
        .info-label {
            font-weight: bold;
            color: #495057;
            margin-bottom: 5px;
        }
        .info-value {
            color: #6c757d;
            word-break: break-all;
        }
        .timestamp {
            color: #6c757d;
            font-size: 0.8rem;
            text-align: center;
            padding: 10px;
            background: #f8f9fa;
            border-top: 1px solid #dee2e6;
        }
    </style>
</head>
<body>
    <div class="error-container">
        <div class="error-header">
            <div>
                <div class="error-code">{{.Error.Code}}</div>
                <div class="error-message">{{.Error.Message}}</div>
            </div>
            <div style="text-align: right;">
                <div style="font-size: 0.8rem; opacity: 0.8;">Astra Framework</div>
                <div style="font-size: 0.7rem; opacity: 0.6;">v{{.AppVersion}}</div>
            </div>
        </div>

        <div class="tabs">
            <button class="tab active" onclick="showTab('overview')">Overview</button>
            <button class="tab" onclick="showTab('request')">Request</button>
            {{if .Debug.Stack}}<button class="tab" onclick="showTab('debug')">Debug</button>{{end}}
        </div>

        <div id="overview" class="tab-content active">
            <div class="info-item">
                <div class="info-label">Error Type</div>
                <div class="info-value">{{.Error.Type}}</div>
            </div>
            <div class="info-item">
                <div class="info-label">Status Code</div>
                <div class="info-value">{{.Error.Code}}</div>
            </div>
            <div class="info-item">
                <div class="info-label">Message</div>
                <div class="info-value">{{.Error.Message}}</div>
            </div>
        </div>

        <div id="request" class="tab-content">
            <div class="request-info">
                <div class="info-item">
                    <div class="info-label">Method</div>
                    <div class="info-value">{{.Request.Method}}</div>
                </div>
                <div class="info-item">
                    <div class="info-label">URL</div>
                    <div class="info-value">{{.Request.URL}}</div>
                </div>
                <div class="info-item">
                    <div class="info-label">IP Address</div>
                    <div class="info-value">{{.Request.IP}}</div>
                </div>
                <div class="info-item">
                    <div class="info-label">User Agent</div>
                    <div class="info-value">{{.Request.UserAgent}}</div>
                </div>
            </div>

            <h4 style="margin: 20px 0 10px 0; color: #495057;">Headers</h4>
            <div class="code-block">{{range $key, $value := .Request.Headers}}{{$key}}: {{$value}}
{{end}}</div>

            {{if .Request.Params}}<h4 style="margin: 20px 0 10px 0; color: #495057;">Route Parameters</h4>
            <div class="code-block">{{range $key, $value := .Request.Params}}{{$key}}: {{$value}}
{{end}}</div>{{end}}

            {{if .Request.Query}}<h4 style="margin: 20px 0 10px 0; color: #495057;">Query Parameters</h4>
            <div class="code-block">{{range $key, $value := .Request.Query}}{{$key}}: {{$value}}
{{end}}</div>{{end}}
        </div>

        {{if .Debug.Stack}}<div id="debug" class="tab-content">
            <h4 style="margin: 20px 0 10px 0; color: #495057;">Stack Trace</h4>
            <div class="code-block">{{.Debug.Stack}}</div>

            {{if .Debug.Context}}<h4 style="margin: 20px 0 10px 0; color: #495057;">Context Data</h4>
            <div class="code-block">{{range $key, $value := .Debug.Context}}{{$key}}: {{$value}}
{{end}}</div>{{end}}
        </div>{{end}}

        <div class="timestamp">
            Error occurred at {{.Timestamp.Format "2006-01-02 15:04:05 MST"}}
        </div>
    </div>

    <script>
        function showTab(tabName) {
            // Hide all tab contents
            const contents = document.querySelectorAll('.tab-content');
            contents.forEach(content => content.classList.remove('active'));
            
            // Remove active class from all tabs
            const tabs = document.querySelectorAll('.tab');
            tabs.forEach(tab => tab.classList.remove('active'));
            
            // Show selected tab content
            document.getElementById(tabName).classList.add('active');
            
            // Add active class to clicked tab
            event.target.classList.add('active');
        }

        // Auto-refresh functionality
        let refreshInterval;
        
        function startAutoRefresh() {
            refreshInterval = setInterval(() => {
                window.location.reload();
            }, 5000);
        }
        
        function stopAutoRefresh() {
            if (refreshInterval) {
                clearInterval(refreshInterval);
            }
        }

        // Keyboard shortcuts
        document.addEventListener('keydown', (e) => {
            if (e.key === 'r' && e.ctrlKey) {
                e.preventDefault();
                window.location.reload();
            }
            if (e.key === 'Escape') {
                window.history.back();
            }
        });

        // Start auto-refresh in development
        {{if .Debug.Stack}}startAutoRefresh();{{end}}
    </script>
</body>
</html>`

	t, err := template.New("error").Parse(tmpl)
	if err != nil {
		return c.InternalError("Failed to render error page")
	}

	var buf strings.Builder
	if err := t.Execute(&buf, data); err != nil {
		return c.InternalError("Failed to render error page")
	}

	return c.SendString(buf.String(), statusCode)
}

// Helper functions
func getErrorType(code int) string {
	switch {
	case code >= 400 && code < 500:
		return "client_error"
	case code >= 500:
		return "server_error"
	default:
		return "unknown"
	}
}

func getHeaders(r *http.Request) map[string]string {
	headers := make(map[string]string)
	for k, v := range r.Header {
		headers[k] = strings.Join(v, ", ")
	}
	return headers
}

func getAllParams(c *astrahttp.Context) map[string]string {
	params := make(map[string]string)

	// Route parameters
	for k, v := range c.Request.URL.Query() {
		params[k] = v[0]
	}

	return params
}

func getAllQuery(c *astrahttp.Context) map[string]string {
	query := make(map[string]string)
	for k, v := range c.Request.URL.Query() {
		query[k] = strings.Join(v, ", ")
	}
	return query
}

func getContextVariables(c *astrahttp.Context) map[string]interface{} {
	// Extract relevant context variables for debugging
	return map[string]interface{}{
		"user_id":    c.Get("user_id"),
		"request_id": c.Get("request_id"),
		"locale":     c.Get("locale"),
	}
}

func getQueryInfo(c *astrahttp.Context) []QueryInfo {
	// This would need to be integrated with your ORM to capture actual queries
	// For now, return empty slice
	return []QueryInfo{}
}

func getContextData(c *astrahttp.Context) map[string]interface{} {
	// Extract all context data for debugging
	data := make(map[string]interface{})

	// Common context keys
	if userID := c.Get("user_id"); userID != nil {
		data["user_id"] = userID
	}
	if requestID := c.Get("request_id"); requestID != nil {
		data["request_id"] = requestID
	}
	if locale := c.Get("locale"); locale != nil {
		data["locale"] = locale
	}

	return data
}
