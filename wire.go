// go:build wireinject
// +build wireinject

package astra

import (
	"net/http"
	"github.com/google/wire"
	"github.com/shauryagautam/Astra/pkg/engine"
	astrahttp "github.com/shauryagautam/Astra/pkg/engine/http"
	"github.com/shauryagautam/Astra/pkg/engine/runtime"
)

// ServerSet provides the default Astra server and its dependencies.
var ServerSet = wire.NewSet(
	runtime.ProviderSet,
	astrahttp.NewRouter,
	astrahttp.NewServer,
	wire.Bind(new(http.Handler), new(*astrahttp.Router)),
)

// InitializeServer initializes the application server.
func InitializeServer(addr string) (*astrahttp.Server, error) {
	wire.Build(ServerSet)
	return nil, nil
}

// InitializeApp initializes the core application kernel.
func InitializeApp() (*engine.App, error) {
	wire.Build(runtime.ProviderSet)
	return nil, nil
}
