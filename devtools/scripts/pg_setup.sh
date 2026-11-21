CONF=/etc/postgresql/15/main/postgresql.conf
HBA=/etc/postgresql/15/main/pg_hba.conf

echo "=== 1. 备份原配置 ==="
cp -n "$CONF" "$CONF.bak.dbping" && echo "已备份 $CONF.bak.dbping"
cp -n "$HBA" "$HBA.bak.dbping" && echo "已备份 $HBA.bak.dbping"

echo "=== 2. listen_addresses = '*' ==="
sed -i "s/^#\?listen_addresses.*/listen_addresses = '*'/" "$CONF"
grep "^listen_addresses" "$CONF" || echo "WARN: 未写入"

echo "=== 3. pg_hba 增加局域网授权 ==="
if grep -q "dbping-allow" "$HBA"; then
  echo "规则已存在，跳过"
else
  printf '\n# dbping-allow: 局域网远程访问 (192.168.31.0/24)\nhost all all 192.168.31.0/24 scram-sha-256\n' >> "$HBA"
  echo "已追加规则"
fi
tail -4 "$HBA"

echo "=== 4. 设置 postgres 密码 ==="
sudo -u postgres psql -c "ALTER USER postgres PASSWORD 'Dbping#2026';"

echo "=== 5. 重启服务并验证监听 ==="
systemctl restart postgresql@15-main
sleep 2
systemctl is-active postgresql@15-main
ss -tlnp | grep 5432 || echo "WARN: 5432 未监听"

echo "=== 6. 服务端自验证 ==="
sudo -u postgres psql -tAc "SELECT version();"
sudo -u postgres psql -tAc "SELECT name, setting FROM pg_settings WHERE name IN ('listen_addresses','archive_mode','wal_level','max_connections');"
