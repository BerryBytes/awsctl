# awsctl {{ .Version }} Release Notes

**Release Date**: {{ .Date }}

### Overview

`awsctl` is a Go-based command-line tool designed to simplify AWS resource management, including SSH connections to EC2 instances, RDS databases, EKS clusters, and ECR repositories. This initial release ({{ .Version }}) introduces core functionality to streamline AWS workflows for developers and DevOps engineers.

### New Features

### Bug Fixes

### Installation

1. To install the latest version.

```
curl -sS https://raw.githubusercontent.com/berrybytes/awsctl/main/installer.sh | bash
```

2. To install specific version (e.g: `v0.0.1`)

```
curl -sS https://raw.githubusercontent.com/berrybytes/awsctl/main/installer.sh | bash -s -- v0.0.1

```

# Verify installation
`awsctl --help`

### Getting Started

See the [README](https://github.com/berrybytes/awsctl) for setup and usage instructions.

### Feedback

This is our first release, and we’d love your input! Report issues or suggest features on our [GitHub Issues page](https://github.com/berrybytes/awsctl/issues).

### Technical Changelog

For a complete list of changes, including internal updates, see the [Changelog](https://github.com/berrybytes/awsctl/blob/develop/CHANGELOG.md).

Thank you for using `awsctl`!
