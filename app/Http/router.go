package http

import (
	"fmt"
	"strings"
	"sync"

	"github.com/shaurya/adonis/contracts"
)

// Route represents a single registered route.
type Route struct {
	pattern    string
	methods    []string
	handler    contracts.HandlerFunc
	name       string
	middleware []string
	prefix     string
	paramNames []string // extracted from pattern like ":id"
}

// NewRoute creates a new Route.
func NewRoute(methods []string, pattern string, handler contracts.HandlerFunc) *Route {
	// Extract parameter names from pattern
	var paramNames []string
	for _, segment := range strings.Split(pattern, "/") {
		if strings.HasPrefix(segment, ":") {
			paramNames = append(paramNames, segment[1:])
		}
	}
	return &Route{
		pattern:    pattern,
		methods:    methods,
		handler:    handler,
		middleware: make([]string, 0),
		paramNames: paramNames,
	}
}

func (r *Route) Pattern() string                { return r.pattern }
func (r *Route) Methods() []string              { return r.methods }
func (r *Route) Handler() contracts.HandlerFunc { return r.handler }
func (r *Route) Name() string                   { return r.name }
func (r *Route) Prefix() string                 { return r.prefix }

func (r *Route) As(name string) contracts.RouteContract {
	r.name = name
	return r
}

func (r *Route) Middleware(names ...string) contracts.RouteContract {
	r.middleware = append(r.middleware, names...)
	return r
}

func (r *Route) GetMiddleware() []string {
	return r.middleware
}

// match checks if a given path matches this route's pattern and extracts params.
func (r *Route) match(path string) (map[string]string, bool) {
	patternParts := strings.Split(strings.Trim(r.pattern, "/"), "/")
	pathParts := strings.Split(strings.Trim(path, "/"), "/")

	if len(patternParts) != len(pathParts) {
		return nil, false
	}

	params := make(map[string]string)
	for i, part := range patternParts {
		if strings.HasPrefix(part, ":") {
			params[part[1:]] = pathParts[i]
		} else if strings.HasPrefix(part, "*") {
			// Wildcard — match the rest
			params[part[1:]] = strings.Join(pathParts[i:], "/")
			return params, true
		} else if part != pathParts[i] {
			return nil, false
		}
	}
	return params, true
}

// Ensure Route implements RouteContract at compile time.
var _ contracts.RouteContract = (*Route)(nil)

// RouteGroup represents a group of routes sharing common configuration.
type RouteGroup struct {
	prefix_    string
	middleware []string
	namespace  string
	namePrefix string
	routes     []*Route
}

func (g *RouteGroup) Prefix(prefix string) contracts.RouteGroupContract {
	g.prefix_ = prefix
	// Update all routes in the group
	for _, route := range g.routes {
		route.pattern = prefix + route.pattern
		route.prefix = prefix
	}
	return g
}

func (g *RouteGroup) Middleware(names ...string) contracts.RouteGroupContract {
	g.middleware = append(g.middleware, names...)
	for _, route := range g.routes {
		route.middleware = append(names, route.middleware...)
	}
	return g
}

func (g *RouteGroup) Namespace(namespace string) contracts.RouteGroupContract {
	g.namespace = namespace
	return g
}

func (g *RouteGroup) As(name string) contracts.RouteGroupContract {
	g.namePrefix = name
	for _, route := range g.routes {
		if route.name != "" {
			route.name = name + "." + route.name
		}
	}
	return g
}

// Ensure RouteGroup implements RouteGroupContract at compile time.
var _ contracts.RouteGroupContract = (*RouteGroup)(nil)

// Router is the main route registry.
// Mirrors AdonisJS's Route module: Route.get(), Route.group(), Route.resource().
type Router struct {
	mu            sync.RWMutex
	routes        []*Route
	staticRoutes  map[string]map[string]*Route // method -> path -> route
	dynamicRoutes map[string][]*Route          // method -> routes with params
	isCommitted   bool
}

// NewRouter creates a new Router.
func NewRouter() *Router {
	return &Router{
		routes:        make([]*Route, 0),
		staticRoutes:  make(map[string]map[string]*Route),
		dynamicRoutes: make(map[string][]*Route),
	}
}

func (router *Router) addRoute(methods []string, pattern string, handler contracts.HandlerFunc) *Route {
	route := NewRoute(methods, pattern, handler)
	router.mu.Lock()
	router.routes = append(router.routes, route)
	router.mu.Unlock()
	return route
}

// Get registers a GET route. Mirrors: Route.get('/path', handler)
func (router *Router) Get(pattern string, handler contracts.HandlerFunc) contracts.RouteContract {
	return router.addRoute([]string{"GET"}, pattern, handler)
}

// Post registers a POST route.
func (router *Router) Post(pattern string, handler contracts.HandlerFunc) contracts.RouteContract {
	return router.addRoute([]string{"POST"}, pattern, handler)
}

// Put registers a PUT route.
func (router *Router) Put(pattern string, handler contracts.HandlerFunc) contracts.RouteContract {
	return router.addRoute([]string{"PUT"}, pattern, handler)
}

// Patch registers a PATCH route.
func (router *Router) Patch(pattern string, handler contracts.HandlerFunc) contracts.RouteContract {
	return router.addRoute([]string{"PATCH"}, pattern, handler)
}

// Delete registers a DELETE route.
func (router *Router) Delete(pattern string, handler contracts.HandlerFunc) contracts.RouteContract {
	return router.addRoute([]string{"DELETE"}, pattern, handler)
}

// Any registers a route for all common HTTP methods.
func (router *Router) Any(pattern string, handler contracts.HandlerFunc) contracts.RouteContract {
	return router.addRoute([]string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}, pattern, handler)
}

// Group creates a route group with shared configuration.
// Mirrors: Route.group(() => { Route.get(...) }).prefix('/api')
func (router *Router) Group(callback func(group contracts.RouterContract)) contracts.RouteGroupContract {
	// Create a sub-router that collects routes, then merge them
	subRouter := NewRouter()
	callback(subRouter)

	group := &RouteGroup{
		routes: subRouter.routes,
	}

	// Add all group routes to the main router
	router.mu.Lock()
	router.routes = append(router.routes, subRouter.routes...)
	router.mu.Unlock()

	return group
}

// Resource registers RESTful resource routes.
// Mirrors: Route.resource('users', UsersController)
//
// Generates:
//
//	GET    /users          -> controller.Index
//	POST   /users          -> controller.Store
//	GET    /users/:id      -> controller.Show
//	PUT    /users/:id      -> controller.Update
//	DELETE /users/:id      -> controller.Destroy
func (router *Router) Resource(name string, controller contracts.ResourceController) contracts.RouteGroupContract {
	basePath := "/" + strings.Trim(name, "/")
	paramPath := basePath + "/:id"

	routes := make([]*Route, 0, 5)

	r1 := router.addRoute([]string{"GET"}, basePath, controller.Index)
	r1.name = name + ".index"
	routes = append(routes, r1)

	r2 := router.addRoute([]string{"POST"}, basePath, controller.Store)
	r2.name = name + ".store"
	routes = append(routes, r2)

	r3 := router.addRoute([]string{"GET"}, paramPath, controller.Show)
	r3.name = name + ".show"
	routes = append(routes, r3)

	r4 := router.addRoute([]string{"PUT", "PATCH"}, paramPath, controller.Update)
	r4.name = name + ".update"
	routes = append(routes, r4)

	r5 := router.addRoute([]string{"DELETE"}, paramPath, controller.Destroy)
	r5.name = name + ".destroy"
	routes = append(routes, r5)

	return &RouteGroup{routes: routes}
}

// GetRoutes returns all registered routes.
func (router *Router) GetRoutes() []contracts.RouteContract {
	router.mu.RLock()
	defer router.mu.RUnlock()
	result := make([]contracts.RouteContract, len(router.routes))
	for i, r := range router.routes {
		result[i] = r
	}
	return result
}

// FindRoute resolves a route for the given method and path.
func (router *Router) FindRoute(method string, path string) (contracts.RouteContract, map[string]string, bool) {
	router.mu.RLock()
	defer router.mu.RUnlock()

	// 1. Try static routes first (O(1))
	if methodRoutes, ok := router.staticRoutes[method]; ok {
		if route, ok := methodRoutes[path]; ok {
			return route, nil, true
		}
	}

	// 2. Try dynamic routes (ordered iteration)
	if dynamicRoutes, ok := router.dynamicRoutes[method]; ok {
		for _, route := range dynamicRoutes {
			if params, ok := route.match(path); ok {
				return route, params, true
			}
		}
	}

	// 3. Fallback to uncommitted routes if not committed yet
	if !router.isCommitted {
		for _, route := range router.routes {
			// Check method match
			methodMatch := false
			for _, m := range route.methods {
				if m == method {
					methodMatch = true
					break
				}
			}
			if !methodMatch {
				continue
			}

			// Check pattern match
			if params, ok := route.match(path); ok {
				return route, params, true
			}
		}
	}

	return nil, nil, false
}

// Commit finalizes route registration by compiling optimized structures.
func (router *Router) Commit() {
	router.mu.Lock()
	defer router.mu.Unlock()

	router.staticRoutes = make(map[string]map[string]*Route)
	router.dynamicRoutes = make(map[string][]*Route)

	for _, route := range router.routes {
		isDynamic := strings.Contains(route.pattern, ":") || strings.Contains(route.pattern, "*")

		for _, method := range route.methods {
			if isDynamic {
				router.dynamicRoutes[method] = append(router.dynamicRoutes[method], route)
			} else {
				if _, ok := router.staticRoutes[method]; !ok {
					router.staticRoutes[method] = make(map[string]*Route)
				}
				router.staticRoutes[method][route.pattern] = route
			}
		}
	}

	router.isCommitted = true
}

// PrintRoutes returns a formatted table of all registered routes.
func (router *Router) PrintRoutes() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%-8s %-30s %-20s %s\n", "METHOD", "PATTERN", "NAME", "MIDDLEWARE"))
	sb.WriteString(strings.Repeat("─", 80) + "\n")

	router.mu.RLock()
	defer router.mu.RUnlock()

	for _, route := range router.routes {
		methods := strings.Join(route.methods, "|")
		mw := strings.Join(route.middleware, ", ")
		sb.WriteString(fmt.Sprintf("%-8s %-30s %-20s %s\n", methods, route.pattern, route.name, mw))
	}
	return sb.String()
}

// Ensure Router implements RouterContract at compile time.
var _ contracts.RouterContract = (*Router)(nil)
