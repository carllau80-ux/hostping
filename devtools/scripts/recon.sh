echo "=== OS ==="
head -5 /etc/os-release 2>/dev/null
echo "=== Docker ==="
docker ps 2>/dev/null | head -10 || true
echo "=== PG 进程 ==="
ps aux | grep -i postgres | grep -v grep | head -6 || true
echo "=== PG 服务 ==="
systemctl list-units --type=service --all 2>/dev/null | grep -i -E 'postgres|pgsql' || true
echo "=== 5432 监听 ==="
ss -tlnp 2>/dev/null | grep -E '5432|postgres' || echo "(未监听 5432)"
echo "=== psql 版本 ==="
psql --version 2>/dev/null || ls /usr/pgsql-*/bin/psql 2>/dev/null || echo "(未找到 psql)"
echo "=== 配置文件路径 ==="
sudo -u postgres psql -tAc "SHOW config_file" 2>/dev/null || true
sudo -u postgres psql -tAc "SHOW hba_file" 2>/dev/null || true
echo "=== 常见目录 ==="
ls -d /var/lib/pgsql* /etc/postgresql* /usr/pgsql-* 2>/dev/null || true
echo "=== 防火墙 ==="
systemctl is-active firewalld 2>/dev/null || true
ufw status 2>/dev/null | head -3 || true
echo "=== SELinux ==="
getenforce 2>/dev/null || true
