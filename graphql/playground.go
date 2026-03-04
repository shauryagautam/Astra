package graphql

import (
	"net/http"
	"text/template"
)

const playgroundHTML = `
<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8" />
  <title>Astra GraphQL Playground</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/graphiql/graphiql.min.css" />
</head>
<body style="margin: 0;">
  <div id="graphiql" style="height: 100vh;"></div>
  <script src="https://cdn.jsdelivr.net/npm/react/umd/react.production.min.js"></script>
  <script src="https://cdn.jsdelivr.net/npm/react-dom/umd/react-dom.production.min.js"></script>
  <script src="https://cdn.jsdelivr.net/npm/graphiql/graphiql.min.js"></script>
  <script>
    const fetcher = GraphiQL.createFetcher({ url: '{{.Endpoint}}' });
    ReactDOM.render(
      React.createElement(GraphiQL, { fetcher: fetcher }),
      document.getElementById('graphiql'),
    );
  </script>
</body>
</html>
`

// PlaygroundHandler returns an http.Handler that renders the GraphiQL playground.
func PlaygroundHandler(endpoint string) http.HandlerFunc {
	tmpl := template.Must(template.New("playground").Parse(playgroundHTML))
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		tmpl.Execute(w, struct{ Endpoint string }{Endpoint: endpoint})
	}
}
