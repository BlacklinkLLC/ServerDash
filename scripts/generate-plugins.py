import json
import pathlib
import re

ROOT = pathlib.Path("/home/Services/ServerDash/plugins")
ID_RE = re.compile(r"^[a-z0-9][a-z0-9-]{1,40}$")
HELPERS = """esc() { printf '%s' "$1" | tr '\\n\\r\\t' '   ' | sed 's/\\\\/\\\\\\\\/g; s/\"/\\\\\"/g'; }
emit_stats() { printf '{\"items\":[{\"label\":\"%s\",\"value\":\"%s\",\"hint\":\"%s\",\"status\":\"%s\"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
"""

items = []


def add(id, name, icon, value, hint="", requires=(), settings=(), status="ok", interval=60, wide=False, description=""):
    items.append(dict(id=id, name=name, icon=icon, value=value, hint=hint, requires=list(requires), settings=list(settings), status=status, interval=interval, wide=wide, description=description))


def setting(key, label, default, type="text", help="", options=()):
    s = {"key": key, "label": label, "type": type, "default": default}
    if help:
        s["help"] = help
    if options:
        s["options"] = list(options)
    return s


url_setting = setting("url", "URL", "https://example.com", help="Checked with curl")
host_setting = setting("host", "Host", "example.com", help="Hostname or IP address")
port_setting = setting("port", "Port", "443", help="TCP port")
domain_setting = setting("domain", "Domain", "example.com", help="Domain to look up")

add("sys-cpu-cores", "CPU Cores", "CPU", "nproc", "Logical CPUs", interval=300)
add("sys-cpu-model", "CPU Model", "CPU", "awk -F: '/model name/{print $2;exit}' /proc/cpuinfo 2>/dev/null || echo n/a", "Processor", interval=3600)
add("sys-cpu-threads", "CPU Threads", "CPU", "ps -eLf --no-headers 2>/dev/null | wc -l", "Active threads", interval=300)
add("sys-load-average", "Load Average", "Speedometer", "awk '{print $1}' /proc/loadavg 2>/dev/null", "1 minute", status='$(awk -v v="$(cat /proc/loadavg 2>/dev/null | awk \'{print $1}\')" \'BEGIN{print (v>4)?"crit":(v>2)?"warn":"ok"}\')')
add("sys-memory-used", "Memory Used", "RAM", "free 2>/dev/null | awk '/Mem:/{if($2==0)print \"0%\";else printf \"%.0f%%\",$3/$2*100}'", "Physical memory", status='$(v=$(free 2>/dev/null | awk \'/Mem:/{print int($3/$2*100)}\'); [ "$v" -ge 90 ] && echo crit || { [ "$v" -ge 75 ] && echo warn || echo ok; })')
add("sys-swap-used", "Swap Used", "RAM", "free 2>/dev/null | awk '/Swap:/{if($2==0)print \"0%\";else printf \"%.0f%%\",$3/$2*100}'", "Swap", interval=300)
add("sys-disk-root", "Root Disk", "Folder", "df -h / 2>/dev/null | awk 'NR==2{print $5}'", "Filesystem /", status='$(v=$(df -P / 2>/dev/null | awk \'NR==2{gsub("%","",$5);print $5}\'); [ "$v" -ge 90 ] && echo crit || { [ "$v" -ge 75 ] && echo warn || echo ok; })')
add("sys-disk-home", "Home Disk", "Folder", "df -h /home 2>/dev/null | awk 'NR==2{print $5}'", "Filesystem /home", status='$(v=$(df -P /home 2>/dev/null | awk \'NR==2{gsub("%","",$5);print $5}\'); [ "$v" -ge 90 ] && echo crit || { [ "$v" -ge 75 ] && echo warn || echo ok; })')
add("sys-disk-tmp", "Temp Disk", "Folder", "df -h /tmp 2>/dev/null | awk 'NR==2{print $5}'", "Filesystem /tmp", status='$(v=$(df -P /tmp 2>/dev/null | awk \'NR==2{gsub("%","",$5);print $5}\'); [ "$v" -ge 90 ] && echo crit || { [ "$v" -ge 75 ] && echo warn || echo ok; })')
add("sys-inodes", "Inodes Used", "Folder", "df -i / 2>/dev/null | awk 'NR==2{print $5}'", "Root filesystem", interval=300)
add("sys-uptime", "Uptime", "Clock", "uptime -p 2>/dev/null || uptime", "Since last boot", interval=300)
add("sys-boot-time", "Boot Time", "Clock", "stat -c %y /proc 2>/dev/null || date", "Host boot timestamp", interval=3600)
add("sys-kernel", "Kernel", "Linux", "uname -r 2>/dev/null", "Running kernel", interval=3600)
add("sys-os", "Operating System", "Server", ". /etc/os-release 2>/dev/null; printf '%s %s' \"${PRETTY_NAME:-unknown}\" \"${VERSION_ID:-}\"", "Distribution", interval=3600)
add("sys-arch", "Architecture", "CPU", "uname -m 2>/dev/null", "Machine architecture", interval=3600)
add("sys-timezone", "Timezone", "Clock", "date +%Z%z 2>/dev/null", "System timezone", interval=3600)
add("sys-local-time", "Local Time", "Clock", "date '+%F %T' 2>/dev/null", "Server clock")
add("sys-users-online", "Users Online", "Users", "who 2>/dev/null | wc -l", "Logged in sessions", interval=300)
add("sys-process-count", "Processes", "CPU", "ps -e --no-headers 2>/dev/null | wc -l", "Running processes", interval=300)
add("sys-zombies", "Zombie Processes", "Warning", "ps -eo stat= 2>/dev/null | grep -c Z || true", "Defunct processes", status='$(v=$(ps -eo stat= 2>/dev/null | grep -c Z || true); [ "$v" -gt 0 ] && echo warn || echo ok)', interval=300)
add("sys-top-cpu", "Top CPU Process", "CPU", "ps -eo comm=,%cpu= --sort=-%cpu 2>/dev/null | head -n1 | awk '{print $1\" \"$2\"%\"}'", "Highest CPU consumer", interval=300)
add("sys-top-mem", "Top Memory Process", "RAM", "ps -eo comm=,%mem= --sort=-%mem 2>/dev/null | head -n1 | awk '{print $1\" \"$2\"%\"}'", "Highest memory consumer", interval=300)
add("sys-open-fds", "Open Descriptors", "Folder", "cat /proc/sys/fs/file-nr 2>/dev/null | awk '{print $1}'", "Allocated file handles", interval=300)
add("sys-fd-limit", "Descriptor Limit", "Folder", "ulimit -n 2>/dev/null", "Per-process limit", interval=3600)
add("sys-cpu-temp", "CPU Temperature", "Thermometer", "sensors 2>/dev/null | awk -F'[+°]' '/Tctl|Package id 0|CPU/{print $2;exit}' || cat /sys/class/thermal/thermal_zone0/temp 2>/dev/null || echo n/a", "Thermal sensor", requires=["sensors"], status='$(v=$(sensors 2>/dev/null | awk -F\'[+°]\' \'/Tctl|Package id 0|CPU/{gsub(/[^0-9.]/,"",$2);print $2;exit}\'); [ -n "$v" ] && [ "${v%.*}" -ge 90 ] 2>/dev/null && echo crit || echo ok)', interval=300)
add("sys-battery", "Battery", "Battery", "cat /sys/class/power_supply/BAT0/capacity 2>/dev/null || echo n/a", "Charge percentage", interval=300)
add("sys-gpu", "GPU", "Desktop", "lspci 2>/dev/null | grep -i -m1 vga | cut -d: -f3 | cut -c2-70 || echo n/a", "Graphics controller", requires=["lspci"], interval=3600)
add("sys-usb-count", "USB Devices", "Connect", "lsusb 2>/dev/null | wc -l || echo 0", "Attached USB devices", requires=["lsusb"], interval=3600)
add("sys-pci-count", "PCI Devices", "Blocks", "lspci 2>/dev/null | wc -l || echo 0", "PCI bus devices", requires=["lspci"], interval=3600)

add("store-log-size", "Log Directory", "Document", "du -sh /var/log 2>/dev/null | awk '{print $1}'", "Size of /var/log", interval=900)
add("store-cache-size", "Cache Directory", "Folder", "du -sh /var/cache 2>/dev/null | awk '{print $1}'", "Size of /var/cache", interval=900)
add("store-home-size", "Home Directory", "Folder", "du -sh /home 2>/dev/null | awk '{print $1}'", "Size of /home", interval=3600)
add("store-var-size", "Var Directory", "Folder", "du -sh /var 2>/dev/null | awk '{print $1}'", "Size of /var", interval=3600)
add("store-usr-size", "Usr Directory", "Folder", "du -sh /usr 2>/dev/null | awk '{print $1}'", "Size of /usr", interval=3600)
add("store-backup-size", "Backup Size", "Folder", "du -sh /var/backups 2>/dev/null | awk '{print $1}'", "Size of /var/backups", interval=900)
add("store-largest-log", "Largest Log", "Document", "find /var/log -type f -printf '%s %p\\n' 2>/dev/null | sort -nr | head -n1 | cut -c1-90", "Biggest log file", interval=900)
add("store-old-tmp", "Old Temp Files", "Trash", "find /tmp -type f -mtime +7 2>/dev/null | wc -l", "Older than 7 days", status='$(v=$(find /tmp -type f -mtime +7 2>/dev/null | wc -l); [ "$v" -gt 100 ] && echo warn || echo ok)', interval=900)
add("store-recent-logs", "Recent Log Files", "Document", "find /var/log -type f -mmin -60 2>/dev/null | wc -l", "Changed in last hour", interval=900)
add("store-file-count", "Home Files", "Folder", "find /home -type f 2>/dev/null | wc -l", "Files under /home", interval=3600)
add("store-dir-count", "Home Directories", "Folder", "find /home -type d 2>/dev/null | wc -l", "Directories under /home", interval=3600)
add("store-mount-count", "Mounts", "Folder", "findmnt -n 2>/dev/null | wc -l", "Active mounts", interval=300)
add("store-block-devices", "Block Devices", "Server", "lsblk -n -o NAME 2>/dev/null | wc -l", "Detected block devices", requires=["lsblk"], interval=3600)
add("store-nfs-mounts", "NFS Mounts", "Folder", "mount -t nfs 2>/dev/null | wc -l", "Network mounts", interval=300)
add("store-tmp-files", "Temp Files", "Trash", "find /tmp -maxdepth 1 -type f 2>/dev/null | wc -l", "Files directly in /tmp", interval=900)
add("store-largest-home", "Largest Home", "Folder", "du -sh /home/* 2>/dev/null | sort -hr | head -n1 | cut -c1-90", "Biggest home folder", interval=3600)
add("store-open-files", "Open Files", "Folder", "lsof 2>/dev/null | wc -l || echo 0", "Files held open", requires=["lsof"], interval=900)
add("store-inode-used", "Inodes Allocated", "Folder", "df -i / 2>/dev/null | awk 'NR==2{print $3}'", "Used inodes", interval=900)
add("store-inode-free", "Inodes Free", "Folder", "df -i / 2>/dev/null | awk 'NR==2{print $4}'", "Free inodes", interval=900)

add("net-default-gw", "Default Gateway", "Connect", "ip route 2>/dev/null | awk '/default/{print $3;exit}'", "IPv4 gateway", requires=["ip"], interval=300)
add("net-dns-servers", "DNS Servers", "Globe", "awk '/^nameserver/{printf \"%s \",$2}' /etc/resolv.conf 2>/dev/null", "Resolver addresses", interval=300)
add("net-listening-ports", "Listening Ports", "Connect", "ss -lnt 2>/dev/null | tail -n +2 | wc -l", "TCP listeners", requires=["ss"], interval=300)
add("net-established", "Established TCP", "Connect", "ss -ant 2>/dev/null | grep -c ESTAB || true", "Active connections", requires=["ss"], interval=300)
add("net-tcp-sockets", "TCP Sockets", "Connect", "ss -t 2>/dev/null | wc -l", "All TCP sockets", requires=["ss"], interval=300)
add("net-udp-sockets", "UDP Sockets", "Connect", "ss -u 2>/dev/null | wc -l", "All UDP sockets", requires=["ss"], interval=300)
add("net-public-v4", "Public IPv4", "Globe", "curl -fsS --max-time 5 https://api.ipify.org 2>/dev/null || echo n/a", "External address", requires=["curl"], interval=900)
add("net-public-v6", "Public IPv6", "Globe", "curl -fsS --max-time 5 -6 https://api64.ipify.org 2>/dev/null || echo n/a", "External address", requires=["curl"], interval=900)
add("net-ping-loss", "Packet Loss", "Ethernet", "ping -c 4 -W 2 1.1.1.1 2>/dev/null | tail -n2 | head -n1 || echo n/a", "ICMP to 1.1.1.1", requires=["ping"], interval=300)
add("net-dns-lookup", "DNS Lookup", "Globe", "getent hosts example.com 2>/dev/null | awk '{print $1}' | head -n1", "example.com address", interval=900)
add("net-http-status", "HTTP Status", "Globe", "curl -sS -o /dev/null -w '%{http_code}' --max-time 5 \"${SETTING_URL:-https://example.com}\" 2>/dev/null || echo n/a", "Target response code", requires=["curl"], settings=[url_setting], status='$(v=$(curl -sS -o /dev/null -w \'%{http_code}\' --max-time 5 "${SETTING_URL:-https://example.com}" 2>/dev/null); case "$v" in 2??|3??) echo ok ;; 4??) echo warn ;; 5??) echo crit ;; *) echo warn ;; esac)', interval=300)
add("net-http-time", "HTTP Latency", "Speedometer", "curl -sS -o /dev/null -w '%{time_total}s' --max-time 5 \"${SETTING_URL:-https://example.com}\" 2>/dev/null || echo n/a", "Time to first byte", requires=["curl"], settings=[url_setting], interval=300)
add("net-http-size", "HTTP Size", "Download", "curl -sS -o /dev/null -w '%{size_download} bytes' --max-time 5 \"${SETTING_URL:-https://example.com}\" 2>/dev/null || echo n/a", "Downloaded bytes", requires=["curl"], settings=[url_setting], interval=300)
add("net-tls-protocol", "TLS Protocol", "Lock", "echo | openssl s_client -connect \"${SETTING_HOST:-example.com}:${SETTING_PORT:-443}\" -servername \"${SETTING_HOST:-example.com}\" 2>/dev/null | awk '/Protocol/{print $3;exit}' || echo n/a", "Negotiated protocol", requires=["openssl"], settings=[host_setting, port_setting], interval=3600)
add("net-traceroute-hops", "Traceroute Hops", "Globe", "traceroute -m 10 -w 1 \"${SETTING_HOST:-example.com}\" 2>/dev/null | tail -n1 | awk '{print $1}' || echo n/a", "Hops to target", requires=["traceroute"], settings=[host_setting], interval=1800)
add("net-whois-expiry", "Domain Expiry", "Clock", "whois \"${SETTING_DOMAIN:-example.com}\" 2>/dev/null | awk -F: '/Registry Expiry Date|Expiry Date/{print $2;exit}' | cut -c1-32 || echo n/a", "Registration expiry", requires=["whois"], settings=[domain_setting], interval=86400)
add("net-ntp-offset", "NTP Offset", "Clock", "chronyc tracking 2>/dev/null | awk -F: '/System time/{print $2;exit}' | cut -c1-32 || echo n/a", "Clock offset", requires=["chronyc"], interval=900)
add("net-time-sync", "Clock Synced", "Clock", "timedatectl 2>/dev/null | awk -F: '/System clock synchronized/{gsub(/^ +/,\"\",$2);print $2;exit}' || echo n/a", "Synchronized state", requires=["timedatectl"], interval=900)
add("net-wifi-ssid", "WiFi Network", "WiFi", "nmcli -t -f active,ssid dev wifi 2>/dev/null | awk -F: '$1==\"yes\"{print $2;exit}' || echo n/a", "Active SSID", requires=["nmcli"], interval=300)
add("net-vpn-links", "VPN Interfaces", "Encrypted", "ip link 2>/dev/null | grep -E 'tun|tap|wg' | wc -l", "Tunnel interfaces", requires=["ip"], interval=300)
add("net-proxy-vars", "Proxy Variables", "Settings", "env 2>/dev/null | grep -iE '^(http|https|all|no)_proxy=' | wc -l", "Configured proxy vars", interval=3600)
add("net-port-check", "Port Open", "Connect", "nc -z -w 2 \"${SETTING_HOST:-127.0.0.1}\" \"${SETTING_PORT:-80}\" 2>/dev/null && echo open || echo closed", "TCP probe result", requires=["nc"], settings=[host_setting, port_setting], status='$(test "$(nc -z -w 2 "${SETTING_HOST:-127.0.0.1}" "${SETTING_PORT:-80}" 2>/dev/null && echo open || echo closed)" = open && echo ok || echo warn)', interval=300)
add("net-ping-host", "Host Ping", "Ethernet", "ping -c 4 -W 2 \"${SETTING_HOST:-1.1.1.1}\" 2>/dev/null | tail -n2 | head -n1 || echo n/a", "ICMP to configured host", requires=["ping"], settings=[host_setting], interval=300)
add("net-link-speed", "Link Speed", "Ethernet", "cat /sys/class/net/*/speed 2>/dev/null | head -n1 || echo n/a", "First NIC speed", interval=3600)

add("docker-version", "Docker Version", "Container", "docker --version 2>/dev/null | awk '{print $3}' | tr -d ',' || echo n/a", "Engine version", requires=["docker"], interval=3600)
add("docker-images", "Docker Images", "Container", "docker images -q 2>/dev/null | wc -l", "Local images", requires=["docker"], interval=300)
add("docker-containers", "Docker Containers", "Container", "docker ps -aq 2>/dev/null | wc -l", "All containers", requires=["docker"], interval=300)
add("docker-running", "Docker Running", "Play", "docker ps -q 2>/dev/null | wc -l", "Running containers", requires=["docker"], interval=300)
add("docker-stopped", "Docker Stopped", "Pause", "docker ps -aq --filter status=exited 2>/dev/null | wc -l", "Exited containers", requires=["docker"], interval=300)
add("docker-volumes", "Docker Volumes", "Folder", "docker volume ls -q 2>/dev/null | wc -l", "Named volumes", requires=["docker"], interval=300)
add("docker-networks", "Docker Networks", "Connect", "docker network ls -q 2>/dev/null | wc -l", "Networks", requires=["docker"], interval=300)
add("docker-compose-count", "Compose Projects", "Stack", "find /srv /opt /home -maxdepth 4 \\( -name docker-compose.yml -o -name compose.yml -o -name compose.yaml \\) -type f 2>/dev/null | wc -l", "Compose files found", interval=1800)
add("docker-unused-images", "Dangling Images", "Trash", "docker images -f dangling=true -q 2>/dev/null | wc -l", "Unused layers", requires=["docker"], status='$(v=$(docker images -f dangling=true -q 2>/dev/null | wc -l); [ "$v" -gt 10 ] && echo warn || echo ok)', interval=900)
add("docker-unhealthy", "Unhealthy Containers", "Warning", "docker ps --filter health=unhealthy -q 2>/dev/null | wc -l", "Failing health checks", requires=["docker"], status='$(v=$(docker ps --filter health=unhealthy -q 2>/dev/null | wc -l); [ "$v" -gt 0 ] && echo crit || echo ok)', interval=300)
add("docker-restarts", "Container Restarts", "Refresh", "docker inspect --format '{{.RestartCount}}' $(docker ps -q) 2>/dev/null | awk '{s+=$1}END{print s+0}'", "Total restart count", requires=["docker"], interval=300)
add("docker-log-size", "Container Logs", "Document", "du -sh /var/lib/docker/containers 2>/dev/null | awk '{print $1}'", "Docker log storage", interval=900)
add("podman-version", "Podman Version", "Container", "podman --version 2>/dev/null | awk '{print $3}' || echo n/a", "Podman version", requires=["podman"], interval=3600)
add("podman-containers", "Podman Containers", "Container", "podman ps -aq 2>/dev/null | wc -l", "All containers", requires=["podman"], interval=300)
add("k8s-nodes", "Kubernetes Nodes", "Organization", "kubectl get nodes --no-headers 2>/dev/null | wc -l", "Cluster nodes", requires=["kubectl"], interval=300)
add("k8s-pods", "Kubernetes Pods", "Container", "kubectl get pods -A --no-headers 2>/dev/null | wc -l", "Pods across namespaces", requires=["kubectl"], interval=300)
add("k8s-failed-pods", "Failed Pods", "Warning", "kubectl get pods -A --field-selector=status.phase=Failed --no-headers 2>/dev/null | wc -l", "Failed workloads", requires=["kubectl"], status='$(v=$(kubectl get pods -A --field-selector=status.phase=Failed --no-headers 2>/dev/null | wc -l); [ "$v" -gt 0 ] && echo crit || echo ok)', interval=300)
add("k8s-services", "Kubernetes Services", "Connect", "kubectl get services -A --no-headers 2>/dev/null | wc -l", "Services", requires=["kubectl"], interval=300)
add("k8s-deployments", "Deployments", "Stack", "kubectl get deployments -A --no-headers 2>/dev/null | wc -l", "Deployments", requires=["kubectl"], interval=300)
add("helm-releases", "Helm Releases", "Script", "helm list -A --no-headers 2>/dev/null | wc -l", "Installed charts", requires=["helm"], interval=900)
add("containerd-version", "containerd Version", "Container", "containerd --version 2>/dev/null | awk '{print $3}' || echo n/a", "Runtime version", requires=["containerd"], interval=3600)

add("dev-git-version", "Git Version", "Git", "git --version 2>/dev/null | awk '{print $3}' || echo n/a", "SCM version", requires=["git"], interval=3600)
add("dev-git-repos", "Git Repositories", "Git", "find /srv /opt /home -maxdepth 5 -type d -name .git 2>/dev/null | wc -l", "Repos discovered", requires=["git"], interval=1800)
add("dev-node-version", "Node Version", "Code", "node --version 2>/dev/null || echo n/a", "JavaScript runtime", requires=["node"], interval=3600)
add("dev-npm-version", "npm Version", "Script", "npm --version 2>/dev/null || echo n/a", "Package manager", requires=["npm"], interval=3600)
add("dev-go-version", "Go Version", "Code", "go version 2>/dev/null | awk '{print $3}' || echo n/a", "Go toolchain", requires=["go"], interval=3600)
add("dev-python-version", "Python Version", "Code", "python3 --version 2>/dev/null | awk '{print $2}' || echo n/a", "Python runtime", requires=["python3"], interval=3600)
add("dev-rust-version", "Rust Version", "Code", "rustc --version 2>/dev/null | awk '{print $2}' || echo n/a", "Rust toolchain", requires=["rustc"], interval=3600)
add("dev-java-version", "Java Version", "Code", "java -version 2>&1 | head -n1 | cut -d'\"' -f2 || echo n/a", "JDK runtime", requires=["java"], interval=3600)
add("dev-php-version", "PHP Version", "Code", "php --version 2>/dev/null | head -n1 | awk '{print $2}' || echo n/a", "PHP runtime", requires=["php"], interval=3600)
add("dev-ruby-version", "Ruby Version", "Code", "ruby --version 2>/dev/null | awk '{print $2}' || echo n/a", "Ruby runtime", requires=["ruby"], interval=3600)
add("dev-compose-version", "Compose Version", "Stack", "docker compose version 2>/dev/null | awk '{print $NF}' || echo n/a", "Docker Compose", requires=["docker"], interval=3600)
add("dev-make-version", "Make Version", "Code", "make --version 2>/dev/null | head -n1 | awk '{print $NF}' || echo n/a", "Build tool", requires=["make"], interval=3600)
add("dev-cmake-version", "CMake Version", "Code", "cmake --version 2>/dev/null | head -n1 | awk '{print $3}' || echo n/a", "Build generator", requires=["cmake"], interval=3600)
add("dev-jq-version", "jq Version", "Code", "jq --version 2>/dev/null || echo n/a", "JSON processor", requires=["jq"], interval=3600)
add("dev-yq-version", "yq Version", "Code", "yq --version 2>/dev/null | head -n1 || echo n/a", "YAML processor", requires=["yq"], interval=3600)
add("dev-curl-version", "curl Version", "Globe", "curl --version 2>/dev/null | head -n1 | awk '{print $2}' || echo n/a", "HTTP client", requires=["curl"], interval=3600)
add("dev-openssl-version", "OpenSSL Version", "Lock", "openssl version 2>/dev/null | awk '{print $2}' || echo n/a", "Crypto library", requires=["openssl"], interval=3600)
add("dev-ssh-version", "OpenSSH Version", "Key", "ssh -V 2>&1 | cut -d_ -f2 | cut -d' ' -f1 || echo n/a", "SSH client", requires=["ssh"], interval=3600)
add("dev-ansible-version", "Ansible Version", "Script", "ansible --version 2>/dev/null | head -n1 | awk '{print $3}' || echo n/a", "Automation engine", requires=["ansible"], interval=3600)
add("dev-terraform-version", "Terraform Version", "Stack", "terraform version 2>/dev/null | head -n1 | awk '{print $2}' || echo n/a", "Infrastructure tool", requires=["terraform"], interval=3600)

add("svc-sshd", "SSH Service", "Key", "systemctl is-active sshd 2>/dev/null || systemctl is-active ssh 2>/dev/null || echo unknown", "sshd unit", requires=["systemctl"], status='$(test "$(systemctl is-active sshd 2>/dev/null || systemctl is-active ssh 2>/dev/null || echo unknown)" = active && echo ok || echo warn)', interval=300)
add("svc-nginx", "Nginx Service", "Globe", "systemctl is-active nginx 2>/dev/null || echo unknown", "nginx unit", requires=["systemctl"], interval=300)
add("svc-apache", "Apache Service", "Globe", "systemctl is-active httpd 2>/dev/null || systemctl is-active apache2 2>/dev/null || echo unknown", "web server unit", requires=["systemctl"], interval=300)
add("svc-mysql", "MySQL Service", "Folder", "systemctl is-active mysqld 2>/dev/null || systemctl is-active mysql 2>/dev/null || echo unknown", "database unit", requires=["systemctl"], interval=300)
add("svc-postgres", "PostgreSQL Service", "Folder", "systemctl is-active postgresql 2>/dev/null || echo unknown", "database unit", requires=["systemctl"], interval=300)
add("svc-redis", "Redis Service", "Stack", "systemctl is-active redis 2>/dev/null || systemctl is-active redis-server 2>/dev/null || echo unknown", "cache unit", requires=["systemctl"], interval=300)
add("svc-docker", "Docker Service", "Container", "systemctl is-active docker 2>/dev/null || echo unknown", "daemon unit", requires=["systemctl"], interval=300)
add("svc-cron", "Cron Service", "Clock", "systemctl is-active cron 2>/dev/null || systemctl is-active crond 2>/dev/null || echo unknown", "scheduler unit", requires=["systemctl"], interval=300)
add("svc-failed-units", "Failed Units", "Warning", "systemctl --failed --no-legend 2>/dev/null | wc -l", "Failed systemd units", requires=["systemctl"], status='$(v=$(systemctl --failed --no-legend 2>/dev/null | wc -l); [ "$v" -gt 0 ] && echo crit || echo ok)', interval=300)
add("svc-active-units", "Active Units", "Blocks", "systemctl list-units --type=service --state=running --no-legend 2>/dev/null | wc -l", "Running services", requires=["systemctl"], interval=300)
add("svc-timers", "Systemd Timers", "Clock", "systemctl list-timers --no-legend 2>/dev/null | wc -l", "Installed timers", requires=["systemctl"], interval=900)
add("svc-sockets", "Systemd Sockets", "Connect", "systemctl list-sockets --no-legend 2>/dev/null | wc -l", "Socket units", requires=["systemctl"], interval=900)
add("svc-mounts", "Systemd Mounts", "Folder", "systemctl list-units --type=mount --no-legend 2>/dev/null | wc -l", "Mount units", requires=["systemctl"], interval=900)
add("svc-journal-size", "Journal Size", "Document", "journalctl --disk-usage 2>/dev/null | awk '{print $NF \" \" $4}' || echo n/a", "Persisted logs", requires=["journalctl"], interval=900)
add("svc-updates-available", "Updates Available", "Refresh", "dnf check-update -q 2>/dev/null | wc -l", "Package updates", requires=["dnf"], status='$(v=$(dnf check-update -q 2>/dev/null | wc -l); [ "$v" -gt 20 ] && echo warn || echo ok)', interval=3600)

add("sec-fail2ban-banned", "Banned IPs", "Shield", "fail2ban-client status sshd 2>/dev/null | awk -F'\\t' '/Banned IP list/{print $2}' || echo n/a", "fail2ban sshd bans", requires=["fail2ban-client"], interval=300)
add("sec-certbot-count", "TLS Certificates", "Lock", "certbot certificates 2>/dev/null | grep -c 'Certificate Name' || echo 0", "Managed certificates", requires=["certbot"], interval=3600)
add("sec-ssh-failed", "Failed SSH Logins", "Warning", "grep -h -c 'Failed password' /var/log/secure /var/log/auth.log 2>/dev/null | awk -F: '{s+=$1}END{print s+0}'", "Auth failures", status='$(v=$(grep -h -c \'Failed password\' /var/log/secure /var/log/auth.log 2>/dev/null | awk -F: \'{s+=$1}END{print s+0}\'); [ "$v" -gt 50 ] && echo warn || echo ok)', interval=900)
add("sec-ssh-root", "Root SSH Logins", "User", "grep -h -c 'Accepted password for root' /var/log/secure /var/log/auth.log 2>/dev/null | awk -F: '{s+=$1}END{print s+0}'", "Successful root logins", status='$(v=$(grep -h -c \'Accepted password for root\' /var/log/secure /var/log/auth.log 2>/dev/null | awk -F: \'{s+=$1}END{print s+0}\'); [ "$v" -gt 0 ] && echo warn || echo ok)', interval=900)
add("sec-sudo-events", "Sudo Events", "Key", "grep -h -c sudo /var/log/secure /var/log/auth.log 2>/dev/null | awk -F: '{s+=$1}END{print s+0}'", "Privilege escalations", interval=900)
add("sec-world-listeners", "Public Listeners", "Globe", "ss -lnt 2>/dev/null | awk 'NR>1 && $4 !~ /127\\.0\\.0\\.1|::1/' | wc -l", "Non-loopback listeners", requires=["ss"], interval=900)
add("sec-world-writable", "World Writable Dirs", "Folder", "find / -xdev -type d -perm -0002 2>/dev/null | wc -l", "Risky directory modes", interval=86400, wide=True)
add("sec-suid-files", "SUID Files", "Shield", "find / -xdev -type f -perm -4000 2>/dev/null | wc -l", "Set-user-ID binaries", interval=86400, wide=True)
add("sec-empty-passwords", "Empty Passwords", "Warning", "awk -F: '($2==\"\"){n++}END{print n+0}' /etc/shadow 2>/dev/null || echo n/a", "Accounts without password", status='$(v=$(awk -F: \'($2=="" ){n++}END{print n+0}\' /etc/shadow 2>/dev/null || echo 0); [ "$v" -gt 0 ] && echo crit || echo ok)', interval=3600)
add("sec-expired-accounts", "Expired Accounts", "Users", "passwd -Sa 2>/dev/null | awk '$2==\"L\"||$2==\"E\"{n++}END{print n+0}'", "Locked or expired", interval=3600)
add("sec-home-perms", "Home Permissions", "Home", "stat -c '%a' /home 2>/dev/null || echo n/a", "Mode of /home", interval=3600)
add("sec-ssh-perms", "SSH Config Mode", "Key", "stat -c '%a' /etc/ssh/sshd_config 2>/dev/null || echo n/a", "sshd_config mode", interval=3600)
add("sec-sudoers-mode", "Sudoers Mode", "Shield", "stat -c '%a' /etc/sudoers 2>/dev/null || echo n/a", "sudoers file mode", interval=3600)
add("sec-shadow-mode", "Shadow Mode", "Lock", "stat -c '%a' /etc/shadow 2>/dev/null || echo n/a", "shadow file mode", interval=3600)

seen = set()
existing = {"docker-disk", "folder-sizes", "public-ip", "serverdash-backup", "ssl-certs", "uptime-monitor"}
for plugin in items:
    pid = plugin["id"]
    assert ID_RE.match(pid), pid
    assert pid not in seen and pid not in existing, pid
    seen.add(pid)
    for s in plugin["settings"]:
        assert re.match(r"^[A-Za-z][A-Za-z0-9_]{0,40}$", s["key"]), s["key"]

for plugin in items:
    directory = ROOT / plugin["id"]
    directory.mkdir(parents=True, exist_ok=True)
    manifest = {
        "id": plugin["id"],
        "name": plugin["name"],
        "version": "1.0.0",
        "description": plugin["description"] or plugin["hint"],
        "author": "Blacklink",
        "icon": plugin["icon"],
        "widgets": [{
            "id": "main",
            "title": plugin["name"],
            "format": "stats",
            "command": "sh ./main.sh",
            "interval": plugin["interval"],
            "wide": plugin["wide"],
        }],
    }
    if plugin["requires"]:
        manifest["requires"] = plugin["requires"]
    if plugin["settings"]:
        manifest["settings"] = plugin["settings"]
    (directory / "plugin.json").write_text(json.dumps(manifest, indent=2) + "\n")
    script = HELPERS + "value=$(" + plugin["value"] + ")\n"
    script += "emit_stats \"" + plugin["name"] + "\" \"$value\" \"" + plugin["hint"] + "\" \"" + plugin["status"] + "\"\n"
    script_path = directory / "main.sh"
    script_path.write_text(script)
    script_path.chmod(0o755)

print(f"generated {len(items)} plugins")
