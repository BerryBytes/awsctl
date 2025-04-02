# AWS CLI Tools 🔧

## Table of Contents 📋

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Features](#feaures)
- [Installation](#installation)
- [Usage](#usage)
- [Commands](#commands)
- [Credits](#credits)
- [License](#license)

## Overview

The `awsctl` sso command provides an easy way to manage AWS Single Sign-On (SSO) authentication and configuration. It includes commands for initializing SSO authentication and setting up AWS SSO profiles.

AWS CLI leverages the powerful [Cobra](https://github.com/spf13/cobra) framework to build a robust and user-friendly command-line interface.

## OS Requirements
- **Linux:** Above Linux 4.16.10-300.fc28.x86_64 x86_64
- **MacOS:** Above Mac OS X 10.7 Lion

## Prerequisites
Before using `awsctl`, ensure the following:

- AWS CLI Installed
- Verify installation:
```
aws --version
```

If not installed, follow the [AWS CLI installation guide](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html).


## Features
- **Authentication & Configuration**: Securely log in and manage your session with Single Sign-On (SSO).

## Installation

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

### Commands

```bash
awsctl sso setup
```
**Note**: This will check if the custom config is available on the path `$HOME/.config/awsctl/` with the names `config.json`, `config.yml` or `config.yaml`

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

- If the config file is not found, new configuration will be setup.

- After Setting up SSO Profile with `awsctl sso setup`, you can run below command to select profile.

```bash
awsctl sso init
```


## Credits

Special thanks to [Berrybytes](https://www.berrybytes.com) for bringing this project to life!


## License

AWS CLI Tools is open-source software licensed under the [MIT License](LICENSE).

This revised README is more visually appealing and user-friendly while maintaining its clarity and professionalism.
