package engine

import "context"

// Component is the lifecycle contract for a resource managed by App.
//
// Start must honor ctx, initialize the resource, and return only when it is
// ready. Stop must honor ctx, be safe after a partial Start, and release all
// resources owned by the component.
type Component interface {
	Start(context.Context) error
	Stop(context.Context) error
}
