# awsctl CLI Documentation

## Commands

### `awsctl sso setup`

Creates or updates AWS SSO profiles.

#### Basic Usage

```bash
awsctl sso setup [flags]
```

#### Flags

| Flag          | Description                                    | Required | Example                                        |
| ------------- | ---------------------------------------------- | -------- | ---------------------------------------------- |
| `--name`      | SSO session name                               | No       | `--name my-sso-session`                        |
| `--start-url` | AWS SSO start URL (must begin with `https://`) | No       | `--start-url https://my-sso.awsapps.com/start` |
| `--region`    | AWS region for the SSO session                 | No       | `--region us-east-1`                           |

#### Behavior

- **Interactive Mode** (default when no flags):

  - Prompts for:
    - SSO Start URL
    - AWS Region
    - Session name (default: "default-sso")
  - Uses defaults from `~/.config/awsctl/config.yml` if available
  - Validates all inputs before creating session

- **Non-interactive Mode** (when all flags provided):

  - Creates session immediately without prompts
  - Validates:
    - Start URL format (`https://`)
    - Valid AWS region
    - Proper session name format

#### Examples

1. Fully interactive:

```bash
awsctl sso setup
```

2. Fully non-interactive:

```bash
awsctl sso setup --name dev-session --start-url https://dev.awsapps.com/start --region us-east-1
```

#### Validation Rules

- `--start-url`: Must begin with `https://`
- `--region`: Must be valid AWS region code
- `--name`:
  - Alphanumeric with dashes/underscores
  - Cannot start/end with special chars
  - 3-64 characters

---

### `awsctl sso init`

Starts SSO authentication using one of the configured SSO profiles.

- Selects from available profiles created via `awsctl sso setup`
- Useful for switching between AWS accounts or roles quickly.
- If you have multiple SSO profiles configured by `awsctl sso setup`, you can easily set the default one by running `awsctl sso init`

---

### `awsctl bastion`

Manages connections to bastion hosts via SSH, SSM, or tunnels.

#### Instance Detection

- If SSO is configured, prompts:

  - "Look for bastion hosts in AWS?"
  - If yes, searches for EC2 instances with the name or tags containing `bastion` for the **selected profile**.
  - Allows easier selection from discovered instances.
  - Prompts for SSH username and SSH key path.

- If SSO is not configured or user chooses not to search AWS:
  - Allows manual entry of bastion host, SSH username, and SSH key.

#### Connection Options

1. SSH:
   - Public or Private IP (uses EC2 Instance Connect if needed).
2. SSM:
   - No SSH key or public IP required.
   - Works with private subnet instances.

#### Requirements for SSM and EC2 Instance Connect

**1. SSM (AWS Systems Manager) Requirements**

- **IAM Role Attached to Instance**:
  - Must have the following AWS managed policies (or equivalent custom policies):
    - `AmazonSSMManagedInstanceCore`
    - `AmazonSSMFullAccess` (for broad access, optional)
- **VPC Endpoints (for private subnets)**:
  - If the instance is in a private subnet (no internet access), **SSM requires the following VPC endpoints**:
    - `com.amazonaws.<region>.ssm`
    - `com.amazonaws.<region>.ec2messages`
    - `com.amazonaws.<region>.ssmmessages`
- **SSM Agent**:
  - Ensure the **SSM Agent** is installed and running on the EC2 instance.

**2. EC2 Instance Connect Requirements**

- **IAM Permissions for Caller/User**:
  - `ec2-instance-connect:SendSSHPublicKey`
  - `ec2:DescribeInstances`
  - `ec2:GetConsoleOutput` (optional)
- **Public DNS/IP Access**:

  - The instance **must have a public IPv4 address or public DNS**, unless used via a bastion or SSM tunnel.

- **VPC Endpoint (Required if the instance is in a private subnet without internet access)**:
  - Create an **Interface VPC Endpoint** for `com.amazonaws.<region>.ec2-instance-connect`
  - Required if the instance is in a private subnet without internet access, allowing EC2 Instance Connect API calls to AWS securely.

#### Extras

- **SOCKS5 Proxy**:
  - Prompts for:
    - SOCKS proxy port (default: `1080`)
  - Establishes a SOCKS proxy to route local traffic securely through the bastion
  - After establishing, follows the **normal bastion connection flow** for selecting or entering host details
- **Port Forwarding**:
  - Prompts for:
    - Local Port (default: `8080`)
    - Remote Host (IP or DNS of target service)
    - Remote Port (service port, e.g., `5432` for PostgreSQL)
  - Establishes SSH tunnel to remote resource via the bastion

---

### `awsctl rds`

Connects to RDS databases with flexibility.

#### Supported Modes:

- **Direct Connect**: If the RDS instance is publicly accessible.
- **Via Bastion**: SSH or SSM tunnel through a bastion host.

#### Supported Databases:

- PostgreSQL, MySQL, others (depending on configuration).
- Dynamic port assignment to avoid collisions.

#### Authentication Methods

- **Token** (IAM Database Authentication)
- **Native Password** (Database user password)

##### Native Password

- Use the **initial password** defined when creating the RDS instance or the password configured for that database user.

##### Token (IAM Authentication)

- Requires **IAM database authentication** to be enabled on the RDS instance.
- **For MySQL**:
  - Users must be configured with the `AWSAuthenticationPlugin`.
- **For PostgreSQL**:
  - Users must be granted the `rds_iam` role.
- You can either **create a new IAM-auth-enabled database user** or **alter existing users** to support IAM-based login.

###### Example: Enable IAM Authentication for Database Users

**MySQL:**

```sql
CREATE USER 'dbuser'@'%' IDENTIFIED WITH AWSAuthenticationPlugin as 'RDS';
GRANT ALL PRIVILEGES ON database_name.* TO 'dbuser'@'%';
```

**PostgreSQL:**

```sql
CREATE USER dbuser WITH LOGIN;
GRANT rds_iam TO dbuser;
```

---

### `awsctl eks`

Simplifies access to Amazon EKS clusters.

- Prompts for:
  - **AWS Region**
  - **SSO Profile** (fetched from `~/.aws/config`; must be set up via `awsctl sso setup`)
- Lists available EKS clusters for the selected region and profile.
- Prompts you to select a cluster and updates (or creates) your `~/.kube/config` with the cluster’s credentials.

---

### `awsctl ecr`

Handles authentication to Amazon ECR for Docker or container image workflows.

- Features:
  - Interactive or profile-based login.
  - Runs `aws ecr get-login-password` under the hood.
  - Supports both public and private Amazon ECR registries.

---

## Example Usage

```bash
awsctl sso setup
awsctl sso init
awsctl bastion
awsctl rds
awsctl eks
awsctl ecr
```
