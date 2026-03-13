# Upgrade & Migration Guide

Astra follows Semantic Versioning (SemVer) and aims for zero breaking changes within major versions. This guide will help you upgrade your Astra applications to the latest version.

## Versioning Policy

- **Major (vX.0.0)**: May contain breaking changes. Includes substantial new features and major architectural improvements.
- **Minor (v0.X.0)**: Backward-compatible features, improvements, and deprecations.
- **Patch (v0.0.X)**: Backward-compatible bug fixes and security patches.

## Upgrading Astra

To upgrade the Astra framework to the latest version, update your `go.mod` file:

```bash
go get github.com/astraframework/astra@latest
go mod tidy
```

## Migration Paths

### Upgrade from v0.8 to v1.0 (Current)

The v1.0 release is the first stable production-ready release. If you are coming from v0.8, please note the following changes:

#### 1. CLI Refactor
The CLI now uses templates for scaffolding. Ensure you have the latest CLI installed:
```bash
go install github.com/astraframework/astra/cli/astra@latest
```

#### 2. HTTP Context Pooling
Astra now pools `http.Context` objects for better performance. If you were storing custom data in the context that needs to persist beyond the request lifecycle (though you shouldn't), please use `c.Request.Context()` instead.

#### 3. ORM Generics
The ORM now uses Go generics extensively.
- **Old**: `orm.Find(&user, id)`
- **New**: `orm.Query[User](db).Find(&user, id)` or `orm.Find(user, id)` (Backward supported but Query is preferred for type safety).

## Deprecation Policy

When a feature is deprecated:
1. It is marked as `Deprecated` in the GoDoc.
2. It will remain supported until the next major release.
3. A migration path will be provided in this guide.

See our [Backward Compatibility Policy](RELEASE.md) for more details.
