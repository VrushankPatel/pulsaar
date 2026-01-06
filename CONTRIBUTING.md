# Contributing to Pulsaar

Thank you for your interest in contributing to Pulsaar! This document provides guidelines and information for contributors.

## Development Setup

1. Ensure you have Go 1.25+ installed
2. Clone the repository: `git clone https://github.com/VrushankPatel/pulsaar.git`
3. Install dependencies: `go mod download`
4. Run validation: `./scripts/validate_repo.sh`
5. Run tests: `go test ./...`

## How to Contribute

1. **Fork the repository** on GitHub
2. **Create a feature branch** from `master`: `git checkout -b feature/your-feature-name`
3. **Make your changes** following the code style guidelines
4. **Add tests** for new functionality
5. **Run validation and tests**:
   - `./scripts/validate_repo.sh`
   - `go test ./...`
6. **Update documentation** if needed (README.md, API_REFERENCE.md, etc.)
7. **Commit your changes** with a clear, descriptive message
8. **Push to your fork** and **submit a pull request**

## Code Style

- Follow standard Go conventions
- Use `gofmt` to format your code
- Add comments for exported functions and types
- Keep functions small and focused
- Use meaningful variable and function names

## Testing

- Write unit tests for all new code
- Ensure all tests pass before submitting
- Test both success and error cases
- Use table-driven tests where appropriate
- Code coverage reports are generated and uploaded in CI

## Pull Request Guidelines

- Provide a clear description of the changes
- Reference any related issues
- Ensure CI checks pass
- Be open to feedback and iterate on your changes

## Releases

To create a new release:

1. Ensure all changes are committed and tested
2. Tag the commit: `git tag vX.Y.Z`
3. Push the tag: `git push origin vX.Y.Z`
4. The CI pipeline will automatically:
   - Build cross-platform binaries (Linux, macOS Intel/Apple Silicon, Windows)
   - Generate checksums and GPG signatures
   - Publish release assets to GitHub Releases
   - Build and publish Debian packages
   - Publish Homebrew formulas to the tap repository

Note: GPG signing requires the `GPG_PRIVATE_KEY` secret to be configured in the repository settings. The Homebrew tap repository (VrushankPatel/homebrew-pulsaar) must exist for formula publishing.

## Security Considerations

Pulsaar handles sensitive file access in Kubernetes environments. Please be mindful of security implications in your contributions. All changes should maintain the read-only, audited nature of the system.

## Getting Help

If you have questions or need help, please:
- Check existing issues and documentation
- Open a new issue for bugs or feature requests
- Join discussions in pull request comments

We appreciate your contributions to making Pulsaar better!</content>
</xai:function_call ><xai:function_call name="write">
<parameter name="filePath">LICENSE