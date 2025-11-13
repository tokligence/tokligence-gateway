# Contributing to Tokligence Gateway

Thank you for your interest in contributing to Tokligence Gateway! We welcome contributions from the community to help build the future of distributed AI infrastructure.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [How to Contribute](#how-to-contribute)
- [Pull Request Process](#pull-request-process)
- [Coding Standards](#coding-standards)
- [Testing Guidelines](#testing-guidelines)
- [Documentation](#documentation)
- [Community](#community)

## Code of Conduct

This project and everyone participating in it is governed by our [Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code. Please report unacceptable behavior to [cs@tokligence.ai](mailto:cs@tokligence.ai).

## Getting Started

1. **Fork the repository** on GitHub
2. **Clone your fork** locally:
   ```bash
   git clone https://github.com/YOUR_USERNAME/tokligence-gateway.git
   cd tokligence-gateway
   ```
3. **Add upstream remote**:
   ```bash
   git remote add upstream https://github.com/tokligence/tokligence-gateway.git
   ```

## Development Setup

### Prerequisites

- Go 1.24+
- Make (optional, for convenience commands)
- Node.js 18+ (for frontend development)

### Quick Start

```bash
# Build the gateway
make build

# Run tests
make test

# Start the gateway daemon
make gds

# View all available commands
make help
```

For detailed setup instructions, see [CLAUDE.md](CLAUDE.md).

## How to Contribute

### Reporting Bugs

Before creating a bug report, please check the [existing issues](https://github.com/tokligence/tokligence-gateway/issues) to avoid duplicates.

When creating a bug report, include:
- A clear, descriptive title
- Steps to reproduce the issue
- Expected behavior vs actual behavior
- Your environment (OS, Go version, etc.)
- Relevant logs or error messages
- Screenshots if applicable

### Suggesting Enhancements

Enhancement suggestions are tracked as GitHub issues. When creating an enhancement suggestion, include:
- A clear, descriptive title
- Detailed description of the proposed feature
- Why this enhancement would be useful
- Possible implementation approach (optional)

### Contributing Code

1. **Find an issue to work on** or create a new one to discuss your idea
2. **Comment on the issue** to let others know you're working on it
3. **Create a feature branch** from `main`:
   ```bash
   git checkout -b feature/your-feature-name
   ```
4. **Make your changes** following our [coding standards](#coding-standards)
5. **Write tests** for your changes
6. **Run tests** to ensure everything passes:
   ```bash
   make test
   ```
7. **Commit your changes** with a descriptive commit message
8. **Push to your fork** and create a pull request

## Pull Request Process

### Before Submitting

- Ensure all tests pass (`make test`)
- Update documentation if needed
- Follow the commit message guidelines below
- Add yourself to `CONTRIBUTORS.md` if this is your first contribution

### Commit Message Guidelines

We follow conventional commits format:

```
<type>(<scope>): <subject>

<body>

<footer>
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `refactor`: Code refactoring
- `test`: Adding or updating tests
- `chore`: Maintenance tasks
- `perf`: Performance improvements
- `ci`: CI/CD changes

**Examples:**
```
feat(responses): add duplicate tool call detection

Implements emergency stop after 5 duplicate tool calls to prevent
infinite loops in LLM interactions.

Closes #123
```

```
fix(streaming): ensure item_id in SSE events

SSE events now properly include item_id for tool call deltas,
fixing client-side parsing issues.
```

**Important:** Do NOT add `Co-Authored-By` tags or attribution footers. Keep commit messages clean and concise.

### Pull Request Template

When creating a PR, please:
- Fill out the PR template completely
- Link to related issues
- Include screenshots for UI changes
- Describe how you tested the changes
- Ensure CI checks pass

### Review Process

1. **Automated checks** must pass (CI, tests, linting)
2. **Code review** by at least one maintainer
3. **Address feedback** and make requested changes
4. **Approval** from maintainer(s)
5. **Merge** by maintainer (squash and merge preferred)

## Coding Standards

### Go Code Style

- Follow [Effective Go](https://golang.org/doc/effective_go.html) guidelines
- Use `gofmt` for formatting
- Run `go vet` to catch common mistakes
- Add comments for exported functions and types
- Keep functions focused and concise

### Project Structure

Understand the key architecture layers:
- `internal/httpserver/` - HTTP handlers and routing
- `internal/adapter/` - Provider adapters (Anthropic, OpenAI)
- `internal/translation/` - Protocol translation logic
- `internal/userstore/` - Identity and ledger storage
- `cmd/` - CLI and daemon entry points

See [CLAUDE.md](CLAUDE.md) for detailed architecture documentation.

### Error Handling

- Always check and handle errors explicitly
- Use meaningful error messages
- Wrap errors with context using `fmt.Errorf`
- Log errors at appropriate levels

### Configuration

- Use environment variables via `TOKLIGENCE_*` prefix
- Add new config to `config/setting.ini`
- Document all new configuration options

## Testing Guidelines

### Unit Tests

```bash
# Run all tests
make test

# Run backend tests only
make bt

# Run specific package tests
go test ./internal/httpserver/...
```

### Integration Tests

```bash
# Run all integration tests
cd tests && ./run_all_tests.sh

# Run specific test suite
./tests/integration/tool_calls/test_tool_call_basic.sh
```

### Test Coverage

- Aim for >80% coverage for new code
- Write table-driven tests where appropriate
- Test error cases and edge conditions
- Mock external dependencies

## Documentation

### Code Documentation

- Add godoc comments for all exported types and functions
- Include examples in comments where helpful
- Keep comments up-to-date with code changes

### User Documentation

- Update `README.md` for user-facing changes
- Add/update docs in `docs/` directory
- Update `CLAUDE.md` for architecture changes
- Include examples and usage instructions

### Release Notes

- Update release notes in `docs/releases/` for significant changes
- Follow the existing format in release note files

## Community

### Getting Help

- **Documentation**: Start with [README.md](README.md) and [CLAUDE.md](CLAUDE.md)
- **Issues**: Search [existing issues](https://github.com/tokligence/tokligence-gateway/issues)
- **Email**: Contact us at [cs@tokligence.ai](mailto:cs@tokligence.ai)

### Recognition

We value all contributions! Contributors will be:
- Listed in `CONTRIBUTORS.md`
- Recognized in release notes for significant contributions
- Part of building the future of distributed AI infrastructure

## License

By contributing to Tokligence Gateway, you agree that your contributions will be licensed under the [Apache License 2.0](LICENSE).

---

Thank you for contributing to Tokligence Gateway! Together, we're building an open, distributed future for AI infrastructure.
