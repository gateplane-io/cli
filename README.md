# GatePlane CLI

Command-line interface for GatePlane - Just-In-Time Access Management.

## 📖 Overview

GatePlane CLI interacts with Vault/OpenBao instances and
condumes the APIs provided by the [GatePlane Plugins](https://github.com/gateplane-io/vault-plugins).

## 🎬 Usage Examples

### Creating an Access Request
![requestor](./assets/requestor.gif)

### Approving an Access Request
![approver](./assets/approver.gif)

### Claiming Access
![requestor2](./assets/requestor2.gif)

### Synopsis

```bash
$ gateplane
GatePlane CLI provides command-line access to GatePlane gates for
requesting, approving, and claiming time-limited access to protected resources.

Usage:
 gateplane [command]

Available Commands:
 approve     Approve an access request
 auth        Authentication operations
 claim       Claim approved access
 completion  Generate the autocompletion script for the specified shell
 config      Manage configuration
 gates       Manage gates
 help        Help about any command
 request     Manage access requests
 status      Show dashboard of all active requests and pending approvals
 version     Show version information

Flags:
 -h, --help                 help for gateplane
 -o, --output string        Output format (table, json, yaml)
 -a, --vault-addr string    Vault server address
 -t, --vault-token string   Vault token for authentication
```

## ✨ Features

- **Request Access**: Submit access requests for protected resources
- **Approve Requests**: Review and approve pending access requests
- **Claim Credentials**: Retrieve time-limited credentials for approved requests
- **Status Tracking**: Monitor request status and access history

## 📦 Installation

Download the latest release binary for your platform from [GitHub Releases](https://github.com/gateplane-io/client-cli/releases) or build from source:

```bash
go build -o gateplane ./cmd
```

## ⚙️ Configuration

Configuration is stored under `~/.gateplane/config.yaml`.
Environment variables or CLI flags and override the stored configuration.

- `VAULT_ADDR`: Vault server address
- `VAULT_TOKEN`: Vault authentication token

Or use flags: `--vault-addr`, `--vault-token`



### ⚖️ License
This project is licensed under the [Elastic License v2](https://www.elastic.co/licensing/elastic-license).

This means:

- ✅ You can use, fork, and modify it for **yourself** or **within your company**.
- ✅ You can submit Pull Requests and redistribute modified versions (with the license attached).
- ❌ You may **not** sell it, offer it as a paid product, or use it in a hosted service (e.g: SaaS).
- ❌ You may **not** re-license it under a different license.

In short: You can use and extend the code freely, privately or inside your business - just don’t build a business around it without our permission.
[This FAQ by Elastic](https://www.elastic.co/licensing/elastic-license/faq) greatly summarizes things.

See the [`./LICENSES/Elastic-2.0.txt`](./LICENSES/Elastic-2.0.txt) file for full details.
