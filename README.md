Core dashboard

* Server overview
* CPU, RAM, disk, and network usage
* Server uptime
* Load average
* Temperature monitoring where available
* Storage health
* Online/offline server status
* Last heartbeat
* Quick health indicator for every server
* Server groups/tags
* Multi-server overview

Service management

* View all running services
* Start/stop/restart services
* Restart individual containers
* View container status
* View container resource usage
* Container uptime
* View exposed ports
* View environment variables with secrets hidden
* View mounted volumes
* View container image/version
* Pull/update images
* Recreate
* Enable/disable automatic restart
* Service dependencies
* Service health checks

Logs

* Live container logs
* Search logs
* Filter by severity
* Filter by time
* Download logs
* Clear/rotate logs
* Automatically detect common errors
* Log streaming through WebSockets

Deployments

* Deploy a new service
* Update an existing service
* Roll back an update
* Deployment history
* Deployment status
* Git-based deployments
* Image-based deployments
* Deployment logs
* Scheduled deployments
* Automatic health verification after deployment
* Automatic rollback if a deployment fails

Server administration

* SSH terminal through the dashboard
* File browser
* File upload/download
* Service configuration editor
* Environment variable management
* Systemd service management
* Firewall status
* Network interfaces
* DNS configuration
* Reboot/shutdown controls
* Package/update status

Networking

* Port/service map
* Open ports
* Listening processes
* Internal service addresses
* Reverse proxy status
* Cloudflare Tunnel status
* DNS status
* Network traffic graphs
* Connection monitoring

Storage

* Disk usage
* Volume management
* Container volumes
* Backup volumes
* Storage cleanup
* Large-file detection
* Disk health
* Snapshot/backup status

Backups

* Create backup
* Restore backup
* Scheduled backups
* Backup history
* Backup verification
* Remote backup destinations
* Backup encryption
* Retention policies

Security

* Blacklink Auth integration
* Role-based access control
* Admin/developer/operator roles
* 2FA
* API keys
* Session management
* Audit logs
* Login history
* IP allow/deny lists
* Secrets management
* Automatic session expiration
* Destructive-action confirmation

Monitoring & alerts

* CPU alerts
* Memory alerts
* Disk-space alerts
* Service-down alerts
* Container crash alerts
* High-temperature alerts
* SSL certificate expiration alerts
* Failed deployment alerts
* Custom alerts
* Email/webhook notifications
* Alert history
* Maintenance mode

Blacklink-specific stuff

* Blacklink service registry
* Blacklink product/service logos
* Service ownership
* Production/staging/development environments
* Service version tracking
* Internal domains
* Deployment channels
* Blacklink Auth status
* Cloudflare status
* R2 status
* Database status
* API health checks
* Dependency visualization

A really nice feature would be a “Service Details” page. For example:

BellRinger
Production • Online

Version: 2.4.1
Server: BL-SRV-01
Container: blacklink-bellringer
Uptime: 14d 6h
CPU: 4.2%
Memory: 312 MB

[Restart] [Update] [Terminal] [Logs]

Health
✓ Application
✓ Database
✓ API
✓ Cloudflare Tunnel

Recent deployments
2.4.1 — Aug 26 — Successful
2.4.0 — Aug 20 — Successful
2.3.9 — Aug 12 — Successful