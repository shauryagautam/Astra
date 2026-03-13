# Astra Release Management

Astra uses Semantic Versioning (SemVer) and a structured release process to ensure stability for production applications.

## Release Cadence

- **Patch Releases**: Weekly (as needed for bug fixes).
- **Minor Releases**: Monthly (new features, backward-compatible).
- **Major Releases**: Every 6-12 months (breaking changes, major overhauls).

## Versioning Scheme

Given a version number **MAJOR.MINOR.PATCH**:

1. **MAJOR** version when you make incompatible API changes.
2. **MINOR** version when you add functionality in a backward compatible manner.
3. **PATCH** version when you make backward compatible bug fixes.

## Stability Guarantees

- **Stable**: Versions v1.x and above. API remains stable for the life of the major version.
- **Beta**: Feature-complete but may have bugs. API is mostly stable.
- **Alpha**: Experimental. API subject to change without notice.

## Branching Strategy

- `main`: Current stable development. Target for PRs.
- `release-vX.Y`: Maintenance branches for older major/minor versions.
- `vX.Y.Z`: Git tags for specific releases.

## Reporting Vulnerabilities

Please report security vulnerabilities through the [Security Policy](SECURITY.md). Do not open public issues for security bugs.
