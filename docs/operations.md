# Operations

## Service status

```bash
systemctl status h2v2-api h2v2-web hysteria-server caddy
```

## Service restart / reload

```bash
systemctl restart h2v2-api
systemctl restart h2v2-web
systemctl restart hysteria-server
systemctl reload caddy
```

## Logs

```bash
journalctl -u h2v2-api -n 200 --no-pager
journalctl -u h2v2-web -n 200 --no-pager
journalctl -u hysteria-server -n 200 --no-pager
journalctl -u caddy -n 200 --no-pager
```

## Config paths

- Panel env: `/opt/h2v2/.env.generated`
- Hysteria config: `/etc/h2v2/hysteria/server.yaml`
- Hysteria TLS cert/key: `/etc/h2v2/hysteria/tls.crt`, `/etc/h2v2/hysteria/tls.key`
- Storage root: `/var/lib/h2v2`
- Audit dir: `/var/log/h2v2/audit`

## Smoke check

```bash
sudo bash /opt/h2v2/current/scripts/smoke-check.sh /opt/h2v2/.env.generated
```
