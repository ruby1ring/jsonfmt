#!/bin/sh
# 构建 jsonfmt 并注册成 launchd 用户服务：登录后自动启动，进程退出会被自动拉起。
# 重复执行 = 重新构建并重启服务。
set -eu
cd "$(dirname "$0")"

DATA_DIR="$HOME/.jsonfmt"
BIN="$DATA_DIR/jsonfmt"
LABEL="local.jsonfmt"
PLIST="$HOME/Library/LaunchAgents/$LABEL.plist"
DOMAIN="gui/$(id -u)"

# 二进制放在 ~/.jsonfmt 而不是桌面目录：launchd 启动的后台进程访问「桌面」会被 macOS 隐私权限拦住
mkdir -p "$DATA_DIR" "$HOME/Library/LaunchAgents"
chmod 700 "$DATA_DIR" # 历史里可能有敏感数据，只给自己读写
go build -trimpath -ldflags "-s -w" -o "$BIN" .

cat > "$PLIST" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>$LABEL</string>
  <key>ProgramArguments</key><array><string>$BIN</string></array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>ProcessType</key><string>Background</string>
  <key>StandardOutPath</key><string>$DATA_DIR/jsonfmt.log</string>
  <key>StandardErrorPath</key><string>$DATA_DIR/jsonfmt.log</string>
</dict>
</plist>
EOF

if launchctl print "$DOMAIN/$LABEL" >/dev/null 2>&1; then
  launchctl kickstart -k "$DOMAIN/$LABEL"
else
  launchctl bootstrap "$DOMAIN" "$PLIST"
fi

echo "jsonfmt 已启动: http://127.0.0.1:7788"
echo "日志: $DATA_DIR/jsonfmt.log    历史: $DATA_DIR/history.json"
