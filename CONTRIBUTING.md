# Contributing to FlashDB

First off, thank you for considering contributing to FlashDB! 🎉

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [How to Contribute](#how-to-contribute)
- [Development Setup](#development-setup)
- [Pull Request Process](#pull-request-process)
- [Style Guidelines](#style-guidelines)
- [Testing](#testing)

## Code of Conduct

This project and everyone participating in it is governed by our commitment to providing a welcoming and inclusive environment. Please be respectful and constructive in all interactions.

## Getting Started

1. **Fork the Repository**
   - Click the "Fork" button on GitHub
   - Clone your fork locally:
     ```bash
     git clone https://github.com/YOUR_USERNAME/flashdb.git
     cd flashdb
     ```

2. **Set Up Remote**
   ```bash
   git remote add upstream https://github.com/Scarage1/FlashDB.git
   ```

## How to Contribute

### Reporting Bugs

- Check if the bug has already been reported in [Issues](https://github.com/Scarage1/FlashDB/issues)
- If not, create a new issue with:
  - Clear, descriptive title
  - Steps to reproduce
  - Expected behavior
  - Actual behavior
  - Environment details (Go version, OS, etc.)

### Suggesting Features

- Open an issue with the `enhancement` label
- Describe the feature and its use case
- Explain why it would be valuable

### Code Contributions

1. **Pick an Issue**
   - Look for issues labeled `good first issue` or `help wanted`
   - Comment on the issue to let others know you're working on it

2. **Create a Branch**
   ```bash
   git checkout -b feature/your-feature-name
   # or
   git checkout -b fix/your-bug-fix
   ```

3. **Make Your Changes**
   - Write clean, well-documented code
   - Follow the style guidelines below

4. **Submit a Pull Request**

## Development Setup

### Prerequisites

- Go 1.22 or higher
- Git
- Node.js 18+ (for frontend development)

### Building

```bash
# Build the server
go build -o flashdb ./cmd/flashdb

# Run tests
go test -race ./...

# Lint
go vet ./...
gofmt -l .
```

### Frontend Development

```bash
cd frontend
npm ci
npm run dev     # Dev server at http://localhost:3000
npm run build   # Static export to out/
```

## Pull Request Process

1. **Update Documentation**
   - Update README.md if adding features
   - Add/update doc comments for exported functions
   - Update `/docs` if changing architecture or protocols

2. **Write Tests**
   - Add tests for new functionality
   - Ensure all tests pass: `go test ./...`
   - Check for race conditions: `go test -race ./...`

3. **Code Quality**
   - Run `go vet ./...` and fix any issues
   - Run `go fmt ./...` to format code
   - Ensure no compiler warnings

4. **Commit Messages**
   - Use clear, descriptive commit messages
   - Format: `type(scope): description`
   - Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`
   - Example: `feat(zset): add ZRANGEBYLEX command`

5. **Pull Request Description**
   - Reference any related issues
   - Describe what changes you made and why
   - Include any breaking changes

## Style Guidelines

### Go Code

- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use `gofmt` for formatting
- Add comments for exported functions, types, and constants
- Keep functions focused and small
- Use meaningful variable names

### Example

```go
// ZAdd adds members with scores to the sorted set at key.
// It returns the number of elements added to the sorted set.
func (e *Engine) ZAdd(key string, members ...ScoredMember) (int, error) {
    e.mu.Lock()
    defer e.mu.Unlock()
    
    // Implementation...
}
```

### Frontend Code (TypeScript/React)

- Use TypeScript for type safety
- Follow React best practices and hooks patterns
- Use Tailwind CSS utility classes
- Keep components small and reusable

## Testing

### Running Tests

```bash
# All tests
go test ./...

# With coverage
go test -cover ./...

# Specific package
go test ./internal/server/

# Verbose output
go test -v ./...

# Race detection
go test -race ./...
```

### Writing Tests

- Place tests in `_test.go` files
- Use table-driven tests where appropriate
- Test both success and error cases
- Mock external dependencies

### Example Test

```go
func TestEngine_ZAdd(t *testing.T) {
    tests := []struct {
        name     string
        key      string
        members  []ScoredMember
        expected int
    }{
        {
            name:     "add single member",
            key:      "myzset",
            members:  []ScoredMember{{Score: 1, Member: "one"}},
            expected: 1,
        },
        // More test cases...
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation...
        })
    }
}
```

## Questions?

Feel free to open an issue with the `question` label if you need help!

---

Thank you for contributing! 🙏
