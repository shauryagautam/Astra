package orm

// AttachmentResolver allows the ORM to resolve attachment URLs
// without depending directly on the storage package.
var AttachmentResolver func(disk, path string) (string, error)

// SetAttachmentResolver sets the global attachment resolver.
func SetAttachmentResolver(fn func(disk, path string) (string, error)) {
	AttachmentResolver = fn
}
