<div align="center">
  <img src="assets/logo.svg" alt="Databasus Logo" width="250"/>

  <h3>MySQL/MariaDB backup tool for Windows Server — standalone <code>.exe</code>, no Docker required</h3>
  <p>This is a fork of <a href="https://github.com/databasus/databasus">databasus/databasus</a> focused on Windows Server deployment as a self-contained <code>.exe</code>. No Docker or Kubernetes required. Primary backup targets are MySQL and MariaDB. The original project also supports PostgreSQL, MongoDB, Docker-based and Kubernetes deployments — see the upstream repository.</p>
  
  <!-- Badges -->
  [![MySQL](https://img.shields.io/badge/MySQL-4479A1?logo=mysql&logoColor=white)](https://www.mysql.com/)
  [![MariaDB](https://img.shields.io/badge/MariaDB-003545?logo=mariadb&logoColor=white)](https://mariadb.org/)
  [![MongoDB](https://img.shields.io/badge/MongoDB-47A248?logo=mongodb&logoColor=white)](https://www.mongodb.com/)
  <br />
  [![Apache 2.0 License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
  [![Platform](https://img.shields.io/badge/platform-windows--server-0078d4?logo=windows)](https://github.com/pierresh/databasus)
  [![Self Hosted](https://img.shields.io/badge/self--hosted-yes-brightgreen)](https://github.com/pierresh/databasus)
  [![Open Source](https://img.shields.io/badge/open%20source-❤️-red)](https://github.com/pierresh/databasus)
  [![Fork of](https://img.shields.io/badge/fork%20of-databasus%2Fdatabasus-grey)](https://github.com/databasus/databasus)

  <p>
    <a href="#-features">Features</a> •
    <a href="#-installation">Installation</a> •
    <a href="#-usage">Usage</a> •
    <a href="#-license">License</a> •
    <a href="#-contributing">Contributing</a>
  </p>

  <p style="margin-top: 20px; margin-bottom: 20px; font-size: 1.2em;">
    <a href="https://databasus.com" target="_blank"><strong>🌐 Databasus website</strong></a>
  </p>
  
  <img src="assets/dashboard-dark.svg" alt="Databasus Dark Dashboard" width="800" style="margin-bottom: 10px;"/>

  <img src="assets/dashboard.svg" alt="Databasus Dashboard" width="800"/>
</div>

---

## ✨ Features

### 💾 **Supported databases**

- **MySQL**: 5.7, 8 and 9
- **MariaDB**: 10, 11 and 12
- **MongoDB**: 4.2+, 5, 6, 7 and 8
- **PostgreSQL**: 12–18 — not recommended in standalone mode (client tools not bundled; app starts with a warning)

### 🔄 **Scheduled backups**

- **Flexible scheduling**: hourly, daily, weekly, monthly or cron
- **Precise timing**: run backups at specific times (e.g., 4 AM during low traffic)
- **Smart compression**: 4-8x space savings with balanced compression (~20% overhead)

### 🧪 **Restore verification** <a href="https://databasus.com/restore-verification">(docs)</a>

> **Not available in standalone mode.** Restore verification spins up a temporary database container and requires Docker. It is supported in the upstream Docker-based deployment.

Databasus performs a real restore to confirm backups are usable, not just intact on disk or checksum check.

- **Triggers**: after each backup or on a flexible schedule (hourly, daily, weekly, monthly or cron)
- **Real restore**: spins up a database container, runs the restore and checks the restored size against the backup
- **Report**: lists every table with its row count
- **Optional notifications**: send the report or failure-only alerts through any configured notifier

### 🗑️ **Retention policies**

- **Time period**: Keep backups for a fixed duration (e.g., 7 days, 3 months, 1 year)
- **Count**: Keep a fixed number of the most recent backups (e.g., last 30)
- **GFS (Grandfather-Father-Son)**: Layered retention — keep hourly, daily, weekly, monthly and yearly backups independently for fine-grained long-term history (enterprises requirement)
- **Size limits**: Set per-backup and total storage size caps to control storage usage

### 🗄️ **Multiple storage destinations** <a href="https://databasus.com/storages">(view supported)</a>

- **Local storage**: Keep backups on your VPS/server
- **Cloud storage**: S3, Cloudflare R2, Google Drive, NAS, Dropbox, SFTP, Rclone and more
- **Secure**: All data stays under your control

### 📱 **Notifications** <a href="https://databasus.com/notifiers">(view supported)</a>

- **Multiple channels**: Email, Telegram, Slack, Discord, webhooks
- **Real-time updates**: Success and failure notifications
- **Team integration**: Perfect for DevOps workflows

### 🔒 **Enterprise-grade security** <a href="https://databasus.com/security">(docs)</a>

- **AES-256-GCM encryption**: Enterprise-grade protection for backup files
- **Zero-trust storage**: Backups are encrypted and remain useless to attackers, so you can safely store them in shared storage like S3, Azure Blob Storage, etc.
- **Encryption for secrets**: Any sensitive data is encrypted and never exposed, even in logs or error messages
- **Read-only user**: Databasus uses a read-only user by default for backups and never stores anything that can modify your data

It is also important for Databasus that you are able to decrypt and restore backups from storages (local, S3, etc.) without Databasus itself. To do so, read our guide on [how to recover directly from storage](https://databasus.com/how-to-recover-without-databasus). We avoid "vendor lock-in" even to open source tool!

### 👥 **Suitable for teams** <a href="https://databasus.com/access-management">(docs)</a>

- **Workspaces**: Group databases, notifiers and storages for different projects or teams
- **Access management**: Control who can view or manage specific databases with role-based permissions
- **Audit logs**: Track all system activities and changes made by users
- **User roles**: Assign viewer, member, admin or owner roles within workspaces

### 🎨 **UX-Friendly**

- **Designer-polished UI**: Clean, intuitive interface crafted with attention to detail
- **Dark & light themes**: Choose the look that suits your workflow
- **Mobile adaptive**: Check your backups from anywhere on any device

### 🔌 **Connection types**

- **Remote** — Databasus connects directly to the database over the network (recommended in read-only mode). No agent or additional software required. Works with cloud-managed and self-hosted databases
- **Agent** — A lightweight Databasus agent (written in Go) runs alongside the database. The agent streams backups directly to Databasus, so the database never needs to be exposed publicly. Supports host-installed databases and Docker containers

### 📦 **Backup types**

- **Logical** — Native dump of the database in its engine-specific binary format. Compressed and streamed directly to storage with no intermediate files
- **Physical** — File-level copy of the entire database cluster. PostgreSQL only; not applicable to MySQL/MariaDB targets

### 🐳 **Self-hosted & secure**

- **Standalone `.exe`**: runs on Windows Server with no Docker, no Kubernetes, and no external services required
- **Privacy-first**: All your data stays on your infrastructure
- **Open source**: Apache 2.0 licensed, inspect every line of code

<img src="assets/healthchecks.svg" alt="Databasus Dashboard" width="800"/>

---

## 📦 Installation

Pre-built binaries are published to the [GitHub releases page](https://github.com/pierresh/databasus/releases) automatically when a version tag is pushed:

```bash
git tag windows-v1.0.0
git push origin windows-v1.0.0
```

The CI builds `databasus-windows-x64.zip` and attaches it to the release. If no release is available yet, follow the [Building from source](#-building-from-source) section below.

Download `databasus-windows-x64.zip` and extract it to a dedicated directory, for example `C:\databasus\`:

```
C:\databasus\
├── databasus.exe
├── install-service.bat
└── install-service.ps1
```

That's the entire installation — no Docker, no extra tools, no configuration file. The UI and all database client tools (MySQL, MariaDB, MongoDB) are embedded inside `databasus.exe` and extracted automatically on first launch.

MySQL and MariaDB backup targets are fully supported. MongoDB is also bundled. PostgreSQL backup targets are not recommended — client tools are not included and the app starts with a warning.

### First run (manual)

To run Databasus manually (e.g. for testing), open PowerShell inside `C:\databasus\` and run:

```powershell
.\databasus.exe --standalone
```

The binary extracts client tools, initialises an embedded database, applies all migrations, and serves the web UI on **port 4005**. Access the dashboard at `http://localhost:4005`. No configuration file is required.

### Installing as a Windows Service (recommended)

To have Databasus start automatically at every Windows boot, install it as a service. Right-click `install-service.bat` and choose **Run as administrator**.

The script will:
1. Register Databasus as a Windows Service set to start automatically
2. Configure automatic restart on crash (restarts after 5 seconds)
3. Start the service immediately
4. Display the service status and log file location

To manage the service afterwards (run in PowerShell as Administrator):

```powershell
Start-Service Databasus   # start
Stop-Service  Databasus   # stop
Get-Service   Databasus   # status
```

To uninstall the service:

```powershell
.\databasus.exe --uninstall-service
```

### Updating

The service registration and `databasus-data\` folder are untouched during an update — only the exe is replaced. Windows locks executables while they are running, so the service must be stopped first:

```powershell
# Run as Administrator
Stop-Service Databasus
# Replace databasus.exe with the new version here
Start-Service Databasus
```

### Firewall

To access the dashboard from other machines on the network, open port 4005:

```powershell
netsh advfirewall firewall add rule `
    name="Databasus" protocol=TCP dir=in action=allow localport=4005
```

### Data and encryption key

All runtime data — internal database, encryption key, client tools, and any locally-stored backups — is written to `databasus-data\` in the same directory as `databasus.exe`. **Back up this directory regularly.** The encryption key in particular must be preserved: without it, encrypted backups stored on S3 or other remote storage cannot be decrypted, even if you reinstall Databasus.

Service logs are written to `databasus-data\databasus.log`.

---

## 🚀 Usage

1. **Access the dashboard**: Navigate to `http://localhost:4005`
2. **Add your first database for backup**: Click "New Database" and follow the setup wizard
3. **Configure schedule**: Choose from hourly, daily, weekly, monthly or cron intervals
4. **Set database connection**: Enter your database credentials and connection details
5. **Choose storage**: Select where to store your backups (local, S3, Google Drive, etc.)
6. **Configure retention policy**: Choose time period, count or GFS to control how long backups are kept
7. **Add notifications** (optional): Configure email, Telegram, Slack, or webhook notifications
8. **Save and start**: Databasus will validate settings and begin the backup schedule

### 🔑 Resetting password

If you need to reset the password, stop Databasus and run:

```powershell
.\databasus.exe --new-password="YourNewSecurePassword123" --email="admin@example.com"
```

Replace the email with the actual address of the user whose password you want to reset.

### 💾 Backing up Databasus itself

See [Data and encryption key](#data-and-encryption-key) in the Installation section.

---

## 🛡️ Security & reliability engineering

Databasus works with sensitive data, so preventing vulnerabilities, unauthorised access and data leaks is a primary concern. We invest in this on both sides of the system: in the code itself (permission checks, encryption, careful handling of secrets) and in the infrastructure around it (dependency analysis, CVE response, DevSecOps best practices). The pipeline below runs automatically on every commit and PR. No single layer is enough on its own, but together they reduce the chance of vulnerable code, unsafe dependencies, broken images, or non-restorable backups reaching a release.

For static analysis we combine several independent passes. CodeQL scans the full codebase for security issues. CodeRabbit reviews every PR and runs gitleaks for secret scanning and semgrep for security rules inline. Dockerfiles and CI workflows get extra rules of their own (pinned action references, least-privilege permissions, suspicious base images), so insecure patterns are flagged before they ever merge. On top of these per-PR checks, Codex Security from OpenAI runs regular, deeper audits of the whole codebase. It's a separate program that catches architectural and cross-cutting issues narrow PR-time scans can miss.

On the dependency side, Dependabot watches all of our dependencies against the GitHub Advisory Database and surfaces CVEs within minutes of publication. Updates run through a cooldown so newly-published versions get a chance to mature before we adopt them. This is a deliberate defence against compromised-package incidents like supply-chain attack. The Dependency Review Action blocks any PR that introduces a new HIGH or CRITICAL CVE outright.

All GitHub Actions are pinned to full commit SHAs rather than floating tags like `@v4` or `@main`, which have been an active attack vector in 2025. Workflows default to least-privilege permissions and only elevate per-job when genuinely needed.

Critical paths are covered by both unit and integration tests. The CI/CD pipeline runs lint, type-check, and the full test suite on every PR. A release only ships if all of it passes.

Found a vulnerability? Report it via the GitHub Security tab. See [SECURITY.md](https://github.com/pierresh/databasus?tab=security-ov-file#readme). Security reports are the highest-priority work queue. For runtime application security (AES-256-GCM at rest, zero-trust storage, encrypted secrets, read-only DB user by default) see [Enterprise-grade security](#-enterprise-grade-security) in the Features section above.

---

## 🔨 Building from source

The CI release pipeline builds and packages `databasus-windows-x64.zip` automatically on every tagged release. If you need to build locally:

**Prerequisites:** Go 1.26.3+, Node.js 20+, pnpm, and the `swag` CLI for Swagger doc generation.

```bash
# Install swag
go install github.com/swaggo/swag/cmd/swag@v1.16.4

# Install frontend dependencies (once)
cd frontend && pnpm install --frozen-lockfile && cd ..

# Generate Swagger docs (required for the cmd package to compile)
cd backend && swag init -d . -g cmd/main.go -o swagger && cd ..
```

**Build and package:**

```bash
cd backend
make build-windows
```

This single command builds the React frontend, embeds it and all client tools (MySQL, MariaDB, MongoDB) into the binary, cross-compiles for Windows amd64, and produces `databasus-windows-x64.zip` at the repo root containing:

```
databasus.exe
install-service.bat
install-service.ps1
```

---

## 📝 License

This project is licensed under the Apache 2.0 License - see the [LICENSE](LICENSE) file for details

## 🤝 Contributing

Contributions are welcome! Open an issue or pull request on [GitHub](https://github.com/pierresh/databasus). For the upstream project's broader contributing guide see [databasus.com/contribute](https://databasus.com/contribute).

## FAQ

### AI disclaimer

#### Windows standalone port (this fork)

This port to a self-contained Windows `.exe` was written and reviewed entirely by AI (Claude Code).
The maintainer directed the work and made architectural decisions, but did not perform
line-by-line code review.

#### Original project (databasus/databasus)

AI is used as a helper for:

- verification of code quality and searching for vulnerabilities
- cleaning up and improving documentation, comments and code
- assistance during development
- double-checking PRs and commits after human review

AI is not used for:

- writing entire code
- "vibe code" approach
- code without line-by-line verification by a human
- code without tests

AI is an assistant to increase productivity and ensure code quality. All work is verified line-by-line by a human before merging.
