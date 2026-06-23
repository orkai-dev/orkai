#!/bin/sh
# Orkai installer (from source) — clones repo, builds, runs natively
# Usage: curl -sSL https://raw.githubusercontent.com/orkai-dev/orkai/main/install-from-git.sh | sudo sh
set -e

# ── Configuration ───────────────────────────────────────────────
REPO_URL="${REPO_URL:-https://github.com/orkai-dev/orkai.git}"
BRANCH="${BRANCH:-main}"
INSTALL_DIR="/opt/orkai"
SRC_DIR="$INSTALL_DIR/src"
BIN_DIR="$INSTALL_DIR/bin"
ENV_FILE="$INSTALL_DIR/.env"
SERVICE_FILE="/etc/systemd/system/orkai.service"

GO_VERSION="1.25.0"
BUN_VERSION="latest"
PGMQ_VERSION="v1.11.1"

# ── Colors ──────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

info()  { printf "${CYAN}[info]${NC}  %s\n" "$1"; }
ok()    { printf "${GREEN}[ok]${NC}    %s\n" "$1"; }
warn()  { printf "${YELLOW}[warn]${NC}  %s\n" "$1"; }
fail()  { printf "${RED}[error]${NC} %s\n" "$1"; exit 1; }

# ── Preflight ───────────────────────────────────────────────────
preflight() {
    if [ "$(id -u)" -ne 0 ]; then
        fail "Please run as root: sudo sh install-from-git.sh"
    fi

    case "$(uname -s)" in
        Linux) ;;
        *) fail "Orkai requires Linux. Detected: $(uname -s)" ;;
    esac

    ARCH="$(uname -m)"
    case "$ARCH" in
        x86_64|amd64) ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        *) fail "Unsupported architecture: $ARCH" ;;
    esac

    MEM_KB=$(grep MemTotal /proc/meminfo | awk '{print $2}')
    MEM_MB=$((MEM_KB / 1024))
    if [ "$MEM_MB" -lt 1800 ]; then
        warn "Low memory: ${MEM_MB}MB (recommended 2048MB+)"
    fi

    if ! command -v k3s >/dev/null 2>&1; then
        for port in 80 443; do
            if ss -tlnp 2>/dev/null | grep -q ":${port} " || \
               netstat -tlnp 2>/dev/null | grep -q ":${port} "; then
                fail "Port ${port} is in use. Traefik needs 80/443 for ingress."
            fi
        done
    fi

    command -v curl >/dev/null 2>&1 || fail "curl is required"
    command -v git >/dev/null 2>&1 || fail "git is required (apt install git / yum install git)"

    ok "Preflight passed (${ARCH}, ${MEM_MB}MB RAM)"
}

# ── Install Go ──────────────────────────────────────────────────
install_go() {
    if command -v go >/dev/null 2>&1; then
        CURRENT_GO=$(go version | awk '{print $3}' | sed 's/go//')
        ok "Go already installed ($CURRENT_GO)"
        return
    fi

    info "Installing Go ${GO_VERSION}..."
    GO_TAR="go${GO_VERSION}.linux-${ARCH}.tar.gz"
    curl -sSL "https://go.dev/dl/${GO_TAR}" -o "/tmp/${GO_TAR}"
    rm -rf /usr/local/go
    tar -C /usr/local -xzf "/tmp/${GO_TAR}"
    rm -f "/tmp/${GO_TAR}"

    export PATH="/usr/local/go/bin:$PATH"
    if ! grep -q '/usr/local/go/bin' /etc/profile.d/go.sh 2>/dev/null; then
        echo 'export PATH="/usr/local/go/bin:$PATH"' > /etc/profile.d/go.sh
    fi

    ok "Go $(go version | awk '{print $3}') installed"
}

# ── Install Bun ─────────────────────────────────────────────────
install_bun() {
    if command -v bun >/dev/null 2>&1; then
        ok "Bun already installed ($(bun --version))"
        return
    fi

    if ! command -v unzip >/dev/null 2>&1; then
        info "Installing unzip (required for Bun)..."
        if command -v apt-get >/dev/null 2>&1; then
            apt-get update -qq && apt-get install -y -qq unzip
        elif command -v dnf >/dev/null 2>&1; then
            dnf install -y -q unzip
        elif command -v yum >/dev/null 2>&1; then
            yum install -y -q unzip
        else
            fail "unzip is required. Install it and re-run."
        fi
    fi

    case "$ARCH" in
        amd64) BUN_TARGET="linux-x64" ;;
        arm64) BUN_TARGET="linux-aarch64" ;;
    esac

    BUN_INSTALL_DIR="/opt/bun"
    BUN_BIN="$BUN_INSTALL_DIR/bin"
    BUN_ZIP="/tmp/bun-${BUN_TARGET}.zip"
    BUN_URI="https://github.com/oven-sh/bun/releases/latest/download/bun-${BUN_TARGET}.zip"

    info "Installing Bun (downloading ${BUN_TARGET} binary)..."
    if ! curl -fsSL --connect-timeout 30 --max-time 300 --progress-bar \
        "$BUN_URI" -o "$BUN_ZIP"; then
        rm -f "$BUN_ZIP"
        fail "Failed to download Bun from $BUN_URI (check network / GitHub access)"
    fi

    rm -rf "$BUN_INSTALL_DIR"
    mkdir -p "$BUN_BIN"
    if ! unzip -oqd "$BUN_BIN" "$BUN_ZIP"; then
        rm -f "$BUN_ZIP"
        fail "Failed to extract Bun archive"
    fi

    mv "$BUN_BIN/bun-${BUN_TARGET}/bun" "$BUN_BIN/bun"
    rm -rf "$BUN_BIN/bun-${BUN_TARGET}" "$BUN_ZIP"
    chmod +x "$BUN_BIN/bun"

    ln -sf "$BUN_BIN/bun" /usr/local/bin/bun
    if [ -f "$BUN_BIN/bunx" ]; then
        ln -sf "$BUN_BIN/bunx" /usr/local/bin/bunx
    fi

    ok "Bun $(bun --version) installed"
}

# ── Install PostgreSQL ──────────────────────────────────────────
install_postgres() {
    if command -v psql >/dev/null 2>&1 && systemctl is-active --quiet postgresql 2>/dev/null; then
        ok "PostgreSQL already running"
        return
    fi

    info "Installing PostgreSQL..."
    if command -v apt-get >/dev/null 2>&1; then
        apt-get update -qq && apt-get install -y -qq postgresql postgresql-client >/dev/null 2>&1
    elif command -v dnf >/dev/null 2>&1; then
        dnf install -y -q postgresql-server postgresql >/dev/null 2>&1
        postgresql-setup --initdb 2>/dev/null || true
    elif command -v yum >/dev/null 2>&1; then
        yum install -y -q postgresql-server postgresql >/dev/null 2>&1
        postgresql-setup initdb 2>/dev/null || true
    else
        fail "Unsupported package manager. Install PostgreSQL manually."
    fi

    systemctl enable --now postgresql >/dev/null 2>&1
    ok "PostgreSQL installed and running"
}

# ── Install K3s ─────────────────────────────────────────────────
install_k3s() {
    if command -v k3s >/dev/null 2>&1; then
        ok "K3s already installed"
        return
    fi

    FLANNEL_BACKEND="vxlan"
    if modinfo wireguard >/dev/null 2>&1 || lsmod | grep -q wireguard; then
        FLANNEL_BACKEND="wireguard-native"
        info "WireGuard detected — using encrypted network"
    else
        info "WireGuard not available — using standard network"
    fi

    info "Installing K3s..."
    curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="server \
        --flannel-backend=$FLANNEL_BACKEND \
        --write-kubeconfig-mode=644" sh -

    info "Waiting for K3s..."
    for i in $(seq 1 60); do
        if k3s kubectl get nodes >/dev/null 2>&1; then break; fi
        sleep 2
    done
    k3s kubectl get nodes >/dev/null 2>&1 || fail "K3s failed to start"
    ok "K3s running"
}

# ── Wait for Traefik ────────────────────────────────────────────
wait_traefik() {
    info "Waiting for Traefik..."
    for i in $(seq 1 60); do
        if k3s kubectl get pods -n kube-system -l app.kubernetes.io/name=traefik -o jsonpath='{.items[0].status.phase}' 2>/dev/null | grep -q Running; then
            ok "Traefik ready (80/443)"
            return
        fi
        sleep 2
    done
    warn "Traefik not ready yet, but may start soon"
}

# ── Configure Traefik TLS ───────────────────────────────────────
configure_traefik_tls() {
    if k3s kubectl get helmchartconfig traefik -n kube-system >/dev/null 2>&1; then
        ok "Traefik TLS already configured"
        return
    fi

    info "Configuring Traefik with Let's Encrypt..."
    cat <<'ACMEEOF' | k3s kubectl apply -f - >/dev/null 2>&1
apiVersion: helm.cattle.io/v1
kind: HelmChartConfig
metadata:
  name: traefik
  namespace: kube-system
spec:
  valuesContent: |
    additionalArguments:
      - "--certificatesresolvers.letsencrypt.acme.storage=/data/acme.json"
      - "--certificatesresolvers.letsencrypt.acme.tlschallenge=true"
    persistence:
      enabled: true
      size: 128Mi
ACMEEOF
    ok "Traefik TLS configured"
}

# ── Generate secrets ────────────────────────────────────────────
generate_secrets() {
    if [ -f "$ENV_FILE" ]; then
        ok "Configuration exists: $ENV_FILE"
        if ! grep -q '^SETUP_SECRET=' "$ENV_FILE"; then
            SETUP_SECRET=$(head -c 32 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 32)
            echo "SETUP_SECRET=$SETUP_SECRET" >> "$ENV_FILE"
        fi
        return
    fi

    DB_PASSWORD=$(head -c 32 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 32)
    JWT_SECRET=$(head -c 48 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 48)
    SETUP_SECRET=$(head -c 32 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 32)

    SERVER_IP=$(curl -sf --max-time 5 https://api.ipify.org 2>/dev/null || \
                curl -sf --max-time 5 https://ifconfig.me 2>/dev/null || \
                hostname -I | awk '{print $1}')

    mkdir -p "$INSTALL_DIR"
    cat > "$ENV_FILE" <<EOF
DB_PASSWORD=$DB_PASSWORD
JWT_SECRET=$JWT_SECRET
SETUP_SECRET=$SETUP_SECRET
SERVER_IP=$SERVER_IP
EOF
    chmod 600 "$ENV_FILE"
    ok "Secrets generated"
}

# ── Setup PostgreSQL database ───────────────────────────────────
setup_database() {
    . "$ENV_FILE"

    if sudo -u postgres psql -tAc "SELECT 1 FROM pg_database WHERE datname='orkai'" 2>/dev/null | grep -q 1; then
        ok "Database 'orkai' already exists"
        return
    fi

    info "Creating database and user..."
    sudo -u postgres psql -c "CREATE USER orkai WITH PASSWORD '$DB_PASSWORD';" 2>/dev/null || true
    sudo -u postgres psql -c "CREATE DATABASE orkai OWNER orkai;" 2>/dev/null || true
    sudo -u postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE orkai TO orkai;" 2>/dev/null || true

    # Ensure md5/scram auth for local TCP connections
    PG_HBA=$(sudo -u postgres psql -tAc "SHOW hba_file" 2>/dev/null)
    if [ -n "$PG_HBA" ] && ! grep -q "host.*orkai.*orkai.*127.0.0.1" "$PG_HBA" 2>/dev/null; then
        echo "host    orkai    orkai    127.0.0.1/32    scram-sha-256" >> "$PG_HBA"
        systemctl reload postgresql 2>/dev/null || true
    fi

    ok "Database ready"
}

# ── Install PGMQ (job queue) ────────────────────────────────────
# Stock Postgres (apt/yum) does not ship the pgmq extension, so we install it
# via the upstream "SQL-only" definition as the orkai DB owner. The API tolerates
# a missing extension when these pgmq.* functions are present.
install_pgmq() {
    . "$ENV_FILE"

    if PGPASSWORD="$DB_PASSWORD" psql -h 127.0.0.1 -U orkai -d orkai -tAc \
        "SELECT 1 FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='pgmq' AND p.proname='create'" 2>/dev/null | grep -q 1; then
        ok "PGMQ already installed"
        return
    fi

    info "Installing PGMQ job queue (${PGMQ_VERSION})..."
    PGMQ_SQL="/tmp/pgmq-${PGMQ_VERSION}.sql"
    PGMQ_URL="https://raw.githubusercontent.com/pgmq/pgmq/${PGMQ_VERSION}/pgmq-extension/sql/pgmq.sql"
    if ! curl -fsSL --connect-timeout 30 --max-time 120 "$PGMQ_URL" -o "$PGMQ_SQL"; then
        rm -f "$PGMQ_SQL"
        fail "Failed to download PGMQ SQL from $PGMQ_URL (check network / GitHub access)"
    fi

    if ! PGPASSWORD="$DB_PASSWORD" psql -h 127.0.0.1 -U orkai -d orkai \
        -v ON_ERROR_STOP=1 -f "$PGMQ_SQL" >/dev/null 2>&1; then
        rm -f "$PGMQ_SQL"
        fail "Failed to install PGMQ into the orkai database"
    fi
    rm -f "$PGMQ_SQL"
    ok "PGMQ installed"
}

# ── Clone and build ─────────────────────────────────────────────
clone_and_build() {
    mkdir -p "$INSTALL_DIR"

    if [ -d "$SRC_DIR/.git" ]; then
        info "Updating source..."
        cd "$SRC_DIR"
        git fetch origin
        git checkout "$BRANCH"
        git reset --hard "origin/$BRANCH"
    else
        info "Cloning repository..."
        git clone --branch "$BRANCH" --depth 1 "$REPO_URL" "$SRC_DIR"
        cd "$SRC_DIR"
    fi

    info "Installing frontend dependencies..."
    cd "$SRC_DIR"
    bun install --frozen-lockfile

    info "Building API..."
    cd "$SRC_DIR/apps/api"
    mkdir -p "$BIN_DIR"
    go build -o "$BIN_DIR/orkai-api" ./cmd/server
    go build -o "$BIN_DIR/orkai-worker" ./cmd/worker
    go build -o "$BIN_DIR/orkai-migrate" ./cmd/migrate

    info "Building frontend..."
    cd "$SRC_DIR/apps/web"
    bun run build

    ok "Build complete"
}

# ── Create systemd service ──────────────────────────────────────
create_service() {
    . "$ENV_FILE"

    cat > "$SERVICE_FILE" <<EOF
[Unit]
Description=Orkai PaaS
After=network.target postgresql.service k3s.service
Requires=postgresql.service

[Service]
Type=simple
User=root
WorkingDirectory=$INSTALL_DIR
ExecStart=$BIN_DIR/orkai-api
Restart=on-failure
RestartSec=5

Environment=DATABASE_URL=postgres://orkai:${DB_PASSWORD}@127.0.0.1:5432/orkai?sslmode=disable
Environment=JWT_SECRET=${JWT_SECRET}
Environment=SETUP_SECRET=${SETUP_SECRET}
Environment=K8S_IN_CLUSTER=false
Environment=KUBECONFIG=/etc/rancher/k3s/k3s.yaml
Environment=APP_URL=http://${SERVER_IP}:3000
Environment=SERVER_PORT=8080
Environment=WEB_DIST_DIR=$SRC_DIR/apps/web/dist
Environment=GIN_MODE=release

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable orkai >/dev/null 2>&1
    systemctl restart orkai

    info "Waiting for Orkai to be ready..."
    for i in $(seq 1 60); do
        if curl -sf http://localhost:3000/healthz >/dev/null 2>&1; then
            ok "Orkai is running"
            return
        fi
        sleep 2
    done

    fail "Orkai failed to start. Check: journalctl -u orkai -f"
}

# ── Summary ─────────────────────────────────────────────────────
summary() {
    . "$ENV_FILE"

    printf "\n"
    printf "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"
    printf "${GREEN}  Orkai is ready! (installed from source)${NC}\n"
    printf "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"
    printf "\n"
    printf "  ${BOLD}Panel:${NC}     ${CYAN}http://%s:3000${NC}\n" "$SERVER_IP"
    printf "  ${BOLD}Source:${NC}    %s\n" "$SRC_DIR"
    printf "  ${BOLD}Binaries:${NC}  %s\n" "$BIN_DIR"
    printf "  ${BOLD}Config:${NC}    %s\n" "$ENV_FILE"
    printf "  ${BOLD}Logs:${NC}      journalctl -u orkai -f\n"
    printf "  ${BOLD}Restart:${NC}   systemctl restart orkai\n"
    printf "\n"
    printf "  ${BOLD}Upgrade:${NC}\n"
    printf "    cd %s && git pull && make build\n" "$SRC_DIR"
    printf "    systemctl restart orkai\n"
    printf "\n"
    printf "  ${BOLD}Port usage:${NC}\n"
    printf "    :3000  → Orkai panel (served by API)\n"
    printf "    :80    → Traefik HTTP  (your deployed apps)\n"
    printf "    :443   → Traefik HTTPS (your deployed apps)\n"
    printf "    :6443  → K3s API\n"
    printf "\n"
    printf "  Open the panel in your browser to create your admin account.\n"
    printf "\n"
}

# ── Main ────────────────────────────────────────────────────────
main() {
    printf "\n"
    printf "${CYAN}  ⛵ Orkai Installer (from source)${NC}\n"
    printf "${CYAN}  Self-hosted PaaS, powered by Kubernetes${NC}\n"
    printf "\n"

    preflight
    install_go
    install_bun
    install_postgres
    install_k3s
    wait_traefik
    configure_traefik_tls
    generate_secrets
    setup_database
    install_pgmq
    clone_and_build
    create_service
    summary
}

main "$@"
