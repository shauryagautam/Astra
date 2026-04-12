# 05. Frontend

Astra treats the frontend as part of the application, not as a disconnected asset folder. The goal is to make development fast, production predictable, and realtime behavior first-class.

## Why this integration exists

The frontend story has to work in two modes at the same time:

1. During development, the browser should talk to Vite for HMR.
2. In production, the Go application should serve fingerprinted assets from a manifest.

That dual-mode behavior keeps development fluid without making production depend on a dev server.

## Vite asset pipeline

Astra’s Vite integration is built around a manifest manager and a template helper registration path. 

### Template usage

Once the asset pipeline is registered, you use the `asset` helper in your HTML templates:

```html
<!DOCTYPE html>
<html>
<head>
    <title>Astra App</title>
    {{ asset "main.tsx" }}
</head>
<body>
    <div id="root"></div>
</body>
</html>
```

- **Development**: This emits `<script type="module" src="http://localhost:5173/src/main.tsx"></script>`.
- **Production**: This emits `<script type="module" src="/build/assets/main-Cxb8X.js"></script>` based on the manifest.

> [!NOTE]
> The manifest is a production concern. Do not hardcode output filenames in templates if the app is going to change builds over time.

## Server-side rendering

Astra’s SSR layer uses the standard template engine pattern with helper functions for assets and app data.

Use SSR when the page needs fast first paint, SEO-friendly HTML, or server-rendered state that should be visible before the client bundle runs.

## Realtime with SSE and WebSockets

Use SSE when you need one-way streaming: job progress, notifications, dashboard updates, or append-only event feeds.

Use WebSockets when the client and server both need to talk frequently: chat, collaborative editing, or a room-based realtime feature.

> [!TIP]
> Start with SSE when you can. It is easier to operate, easier to debug, and usually enough for live updates.

## Copy-Paste Example

```go
package main

import (
	"html/template"
	"github.com/shauryagautam/Astra/assets"
	astrahttp "github.com/shauryagautam/Astra/pkg/engine/http"
)

func setupFrontend() {
	assetsPipeline := assets.New(assets.Config{
		Entrypoints: []string{"frontend/src/main.tsx"},
		OutputDir:    "frontend/dist",
		PublicPath:   "/build/",
	})

	templateEngine := astrahttp.NewTemplateEngine("frontend/views", 
        astrahttp.WithFuncMap(template.FuncMap(assetsPipeline.TemplateHelpers())),
    )
}
```

---

**Next Chapter: [06. Observability](./06-observability.md)**
