# Operations

## Service status

```bash
systemctl status proxy-panel-api proxy-panel-web hysteria-server caddy
```

## Service restart / reload

```bash
systemctl restart proxy-panel-api
systemctl restart proxy-panel-web
systemctl restart hysteria-server
systemctl reload caddy
```

## Logs

```bash
journalctl -u proxy-panel-api -n 200 --no-pager
journalctl -u proxy-panel-web -n 200 --no-pager
journalctl -u hysteria-server -n 200 --no-pager
journalctl -u caddy -n 200 --no-pager
```

## Config paths

- Panel env: `/opt/proxy-panel/.env.generated`
- Hysteria config: `/etc/proxy-panel/hysteria/server.yaml`
- Hysteria TLS cert/key: `/etc/proxy-panel/hysteria/tls.crt`, `/etc/proxy-panel/hysteria/tls.key`
- Storage root: `/var/lib/proxy-panel`
- Audit dir: `/var/log/proxy-panel/audit`

## Smoke check

```bash
sudo bash /opt/proxy-panel/current/scripts/smoke-check.sh /opt/proxy-panel/.env.generated
```
