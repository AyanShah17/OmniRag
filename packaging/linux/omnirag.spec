Name:           omnirag
Version:        1.0.0
Release:        1%{?dist}
Summary:        OmniRAG Dynamic Enterprise Multi-Tenant RAG Backend
License:        Proprietary
URL:            https://github.com/omnirag/omnirag
BuildArch:      x86_64

Requires:       python3 >= 3.10, systemd
BuildRequires:  golang >= 1.21, nodejs >= 18

%description
OmniRAG is an enterprise-grade Dynamic RAG platform featuring multi-cloud storage
connectors (AWS S3, Azure Blob, Supabase, Confluence), incremental SHA-256 chunk-level
diffing, sub-second FlashRank re-ranking, and grounded source citation streaming.

%prep
# Nothing needed for prep

%build
# Build Go Engine
cd %{_builddir}/go-engine
go build -o go-engine ./cmd/server/main.go

# Build Frontend Bundle
cd %{_builddir}/frontend-app
npm install && npm run build

%install
rm -rf %{buildroot}

# Directories
mkdir -p %{buildroot}/usr/bin
mkdir -p %{buildroot}/usr/lib/omnirag
mkdir -p %{buildroot}/usr/lib/systemd/system
mkdir -p %{buildroot}/etc/omnirag
mkdir -p %{buildroot}/var/log/omnirag

# Binaries & Scripts
install -m 0755 %{_builddir}/go-engine/go-engine %{buildroot}/usr/bin/omnirag-go-engine
install -m 0755 %{_builddir}/packaging/config_manager/omnirag_config.py %{buildroot}/usr/bin/omnirag-config

# Python Application Core
cp -r %{_builddir}/python-rag %{buildroot}/usr/lib/omnirag/python-rag

# Systemd Service
install -m 0644 %{_builddir}/packaging/linux/omnirag.service %{buildroot}/usr/lib/systemd/system/omnirag.service

# Default Configuration
install -m 0600 %{_builddir}/.env.example %{buildroot}/etc/omnirag/omnirag.env

%post
# Create omnirag system user if not exists
getent group omnirag >/dev/null || groupadd -r omnirag
getent passwd omnirag >/dev/null || \
    useradd -r -g omnirag -d /usr/lib/omnirag -s /sbin/nologin \
    -c "OmniRAG Service User" omnirag

# Set permissions
chown -R omnirag:omnirag /etc/omnirag
chown -R omnirag:omnirag /var/log/omnirag
chmod 0600 /etc/omnirag/omnirag.env

# Reload systemd
systemctl daemon-reload

echo "======================================================================"
echo " OmniRAG Enterprise Backend Successfully Installed!"
echo "======================================================================"
echo ""
echo " To configure API keys and cloud storage connectors, run:"
echo "   sudo omnirag-config"
echo ""
echo " To start the service, run:"
echo "   sudo systemctl enable --now omnirag"
echo "======================================================================"

%preun
if [ $1 -eq 0 ]; then
    systemctl stop omnirag >/dev/null 2>&1 || :
    systemctl disable omnirag >/dev/null 2>&1 || :
fi

%postun
if [ $1 -ge 1 ]; then
    systemctl restart omnirag >/dev/null 2>&1 || :
fi

%files
/usr/bin/omnirag-go-engine
/usr/bin/omnirag-config
/usr/lib/omnirag/python-rag
/usr/lib/systemd/system/omnirag.service
%config(noreplace) /etc/omnirag/omnirag.env
%dir /etc/omnirag
%dir /var/log/omnirag

%changelog
* Mon Aug 25 2026 OmniRAG Team <support@omnirag.io> - 1.0.0-1
- Initial enterprise release of OmniRAG with multi-cloud diffing and SSE streaming.
