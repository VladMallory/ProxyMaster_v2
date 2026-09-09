cat > /tmp/setup-remnanode-cron.sh <<'SETUP'
#!/bin/bash
# ставить от root
set -e

cat > /usr/local/bin/restart-remnanode.sh <<'EOS'
#!/bin/bash
echo "$(date '+%Y-%m-%d %H:%M:%S'): restarting remnanode..." >> /var/log/remnanode-restart.log
/usr/bin/docker restart remnanode >> /var/log/remnanode-restart.log 2>&1
sleep 5
MEM=$(free -m | awk '/Mem:/{print $3}')
CONT_MEM=$(docker stats --no-stream --format '{{.MemUsage}}' remnanode 2>&1)
echo "$(date '+%Y-%m-%d %H:%M:%S'): done. Host mem: ${MEM}MB used, Container: ${CONT_MEM}" >> /var/log/remnanode-restart.log
if [ $(stat -c%s /var/log/remnanode-restart.log 2>/dev/null || echo 0) -gt 10485760 ]; then
  tail -n 100 /var/log/remnanode-restart.log > /tmp/restart.log.tmp && mv /tmp/restart.log.tmp /var/log/remnanode-restart.log
fi
EOS

chmod +x /usr/local/bin/restart-remnanode.sh
touch /var/log/remnanode-restart.log

# ставим cron без дублей
(crontab -l 2>/dev/null | grep -v 'restart-remnanode'; echo '0 4 * * * /usr/local/bin/restart-remnanode.sh') | crontab -

echo "Done:"
crontab -l | grep restart
ls -lh /usr/local/bin/restart-remnanode.sh /var/log/remnanode-restart.log
SETUP

chmod +x /tmp/setup-remnanode-cron.sh
sudo /tmp/setup-remnanode-cron.sh
