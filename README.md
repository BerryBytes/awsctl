# AWS CLI Tools 🔧

## Table of Contents 📋

- [Overview](#overview)
- [Requirements](#requirements)
- [Features](#feaures)
- [Installation](#installation)
- [Usage](#usage)
- [Commands](#commands)
- [Contributing](#contributing)
- [Releasing](#releasing)
- [Credits](#credits)
- [License](#license)

## Overview

`awsctl` is a CLI tool designed to simplify AWS environment access and resource management using AWS Single Sign-On (SSO). It provides interactive commands to configure profiles, SSH/SSM into bastion hosts, manage RDS connections, update EKS configurations, and login to ECR.

AWS CLI leverages the powerful [Cobra](https://github.com/spf13/cobra) framework to build a robust and user-friendly command-line interface.

## Requirements

### System Requirements

- **OS**:
  - Linux (Kernel 4.17+)
  - macOS (10.13+)
  - Windows ( limited support )
- **Architecture**: x86_64 or ARM64

### Dependencies

| Dependency             | Version             | Installation Guide                                                                                                                      | Verification Command               |
| ---------------------- | ------------------- | --------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------- |
| Go                     | 1.20+               | [Go Installation](https://go.dev/doc/install)                                                                                           | `go version`                       |
| AWS CLI                | v2 with SSO support | [AWS CLI Installation](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html)                                   | `aws --version`                    |
| Session Manager Plugin | Latest              | [Session Manager Plugin](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html) | `session-manager-plugin --version` |
| kubectl                | Latest              | [kubectl Installation](https://kubernetes.io/docs/tasks/tools/install-kubectl/)                                                         | `kubectl version --client`         |
| Docker                 | Latest              | [Docker Installation](https://docs.docker.com/get-docker/)                                                                              | `docker --version`                 |
| ssh                    | Latest              | [OpenSSH Installation](https://www.openssh.com/)                                                                                        | `ssh -V`                           |

**Notes**:

- You need an AWS account with SSO enabled and appropriate permissions to configure SSO profiles.
- Installation commands like `sudo apt install -y kubectl` or `sudo apt install -y docker.io` are Ubuntu-specific. For other systems (e.g., macOS, Windows, or other Linux distributions), refer to the linked installation guides.
- The `ssh` (OpenSSH client) is typically pre-installed on Linux and macOS. If not, install it on Debian-based systems with `sudo apt install -y openssh-client` or use the equivalent for your OS.

## Features

- **SSO Authentication**: Log in and manage AWS SSO profiles with a single command, ensuring secure access to AWS resources.

- **Bastion/EC2 SSH Access**: Connect to bastion hosts or EC2 instances via SSH or SSM with automated session setup.

- **Port Forwarding & SOCKS Proxy**: Access internal AWS resources securely using dynamic port forwarding or a SOCKS5 proxy for flexible networking.

- **RDS Connectivity**: Connect to RDS databases directly or via SSH/SSM tunnels, supporting both direct endpoints and secure tunneling.

- **EKS Cluster Management**: Update your local kubeconfig to access Amazon EKS clusters in seconds, simplifying Kubernetes workflows.

- **ECR Authentication**: Authenticate to Amazon ECR with SSO credentials to securely push and pull container images.

## Installation

### Releases

- Check out the latest releases at [GitHub Releases](https://github.com/berrybytes/awsctl/releases).
  You can also install the awsctl using following command.

1. To install the latest version.

```
curl -sS https://raw.githubusercontent.com/berrybytes/awsctl/main/installer.sh | bash
```

2. To install specific version (e.g: `v0.0.1`)

```
curl -sS https://raw.githubusercontent.com/berrybytes/awsctl/main/installer.sh | bash -s -- v0.0.1
```

### Manual

1. Clone this repository

```
git clone git@github.com:BerryBytes/awsctl.git
```

2. Make the `install-awsctl.sh` executable:

```
chmod +x install-awsctl.sh
```

3. Run the startup script:

```
./install-awsctl.sh
```

### Usage

Start with `awsctl --help` OR `awsctl -h` to get started.

### Configuration File

- The `awsctl sso setup` command checks for a configuration file at `~/.config/awsctl/` (supported formats: `config.json`, `config.yml`, or `config.yaml`). If none exists, new configuration will be setup.

- Below is sample `config.yaml` file in:

```
aws:
  profiles:
    - profileName: "sample-profile"
      region: "xx-xxxx-x"
      accountId: "xxxxxxxxx"
      ssoStartUrl: "xxxxxxxxxxxxxxxxxxxxxx"
      accountList:
        - accountId: "xxxxxxxxx"
          accountName: "xxxxxxxx"
          emailAddress: "xxxxx@xxx.xxx"
          roles:
            - "xxxx"
            - "xxxx"
        - accountId: "xxxxxxxxxx"
          accountName: "xxxxx"
          emailAddress: "xxxxx@xxx.xxx"
          roles:
            - "xxxx"
    - profileName: "sample2-profile"
      region: "xx-xxxxx-x"
      accountId: "xxxxxxxxx"
      ssoStartUrl: "xxxxxxxxxxxxxxxxxxxxxx"
      accountList:
        - accountId: "xxxxxxxxx"
          accountName: "xxxxxx"
          emailAddress: "xxxxx@xxx.xxx"
          roles:
           - "xxxx"

```

### Commands

The following table summarizes the available `awsctl` commands:

| Command            | Description                                                                                        |
| ------------------ | -------------------------------------------------------------------------------------------------- | ------------------ |
| `awsctl sso setup` | Configures AWS SSO profiles, creating or updating a config file at `~/.config/awsctl/config.yaml`. | `awsctl sso setup` |
| `awsctl sso init`  | Initializes SSO authentication using a configured profile via `awsctl sso setup`.                  | `awsctl sso init`  |
| `awsctl bastion`   | Manages SSH/SSM connections, SOCKS proxy, or port forwarding to bastion hosts or EC2 instances.    | `awsctl bastion`   |
| `awsctl rds`       | Connects to RDS databases directly or via SSH/SSM tunnels.                                         | `awsctl rds`       |
| `awsctl eks`       | Updates kubeconfig for accessing Amazon EKS clusters.                                              | `awsctl eks`       |
| `awsctl ecr`       | Authenticates to Amazon ECR for container image operations.                                        | `awsctl ecr`       |

### Contributing

We welcome contributions! Please see our [contributing guidelines](CONTRIBUTING.md) for more details.

### Releasing

To trigger a release, push a commit to `main` with `[release]` in the commit message (e.g., `git commit -m "Add feature [release]"`). The workflow will auto-increment the version, tag it, and create a draft release.

## Credits

Special thanks to [Berrybytes](https://www.berrybytes.com) for bringing this project to life!

## License

AWS CLI Tools is open-source software licensed under the [MIT License](LICENSE).

This revised README is more visually appealing and user-friendly while maintaining its clarity and professionalism.
