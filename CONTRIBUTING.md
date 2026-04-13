# Contributing to Astra

Thank you for your interest in contributing to the Astra framework! We appreciate your help in making this a better tool for the Go developer community.

## 📍 Table of Contents

- [Code of Conduct](#code-of-conduct)
- [How Can I Contribute?](#how-can-i-contribute)
- [Setting Up Your Development Environment](#setting-up-your-development-environment)
- [Style Guide](#style-guide)
- [Pull Request Process](#pull-request-process)

---

## Code of Conduct

All contributors are expected to uphold our Code of Conduct (standard Contributor Covenant). Please be respectful and professional in all interactions.

## How Can I Contribute?

### Reporting Bugs

- **Search first**: Check if the bug has already been reported in the issue tracker.
- **Provide detail**: Include your OS, Go version, and a minimal reproducible example (MRE).
- **Screenshots/Logs**: Attach any relevant error logs or UI screenshots if applicable.

### Suggesting Enhancements

- Check if the feature is already planned in the Roadmap or discussed in existing issues.
- Explain the **Why**: What problem does this feature solve? Who is it for?

## Setting Up Your Development Environment

### 1. Prerequisites

- **Go**: 1.22 or higher.
- **Docker**: For running infrastructure tests (Postgres, Redis).
- **Make**: For running automation scripts.

### 2. Fork and Clone

```bash
git clone https://github.com/YOUR_USERNAME/Astra.git
cd Astra
```

### 3. Install Dependencies

```bash
go mod download
```

### 4. Running Tests

Ensure Docker is running, then execute the test suite:

```bash
# Run all tests
make test

# Run benchmarks
go test -bench=. ./benchmark/...
```

## Style Guide

### Coding Standards

- Follow standard [Go Proverbs](https://go-proverbs.github.io/) and conventions.
- Run `golangci-lint` before submitting.
- Use `slog` for all logging.
- Ensure all new features include unit or integration tests in the `test/` directory.

### Commit Messages

- Use the [Conventional Commits](https://www.conventionalcommits.org/) specification:
  - `feat`: A new feature
  - `fix`: A bug fix
  - `docs`: Documentation changes
  - `refactor`: Code changes that neither fix a bug nor add a feature
  - `test`: Adding missing tests or correcting existing tests

## Pull Request Process

1. **Branching**: Create a feature branch (`feat/my-awesome-feature`) from `main`.
2. **Atomic Commits**: Keep your commits small and focused.
3. **Tests**: Ensure all tests pass.
4. **Docs**: Update the relevant documentation if you change an API or add a feature.
5. **Review**: Once submitted, a maintainer will review your code. Be prepared for feedback!

---

Thank you for contributing to Astra! 🚀
