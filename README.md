# jsonfmt

本机运行的 JSON 格式化工具：一个 Go 二进制，页面和编辑器都编译在里面，不需要联网，内存占用约 10 MB。

## 功能

- 格式化 / 压缩 / 转义 / 去转义，缩进可选 2 空格、4 空格、Tab，可按键名排序
- 左右两边都能编辑，改任意一边另一边自动同步
- 大整数不丢精度（如 `12345678901234567890`），保留键的原始顺序
- 出错时在出错位置画波浪线，行尾直接写出原因，例如「"gpt-4o" 后面少了一个逗号」「这里用了中文逗号」「键名要加英文双引号」
- 折叠 / 展开、⌘F 查找替换、⌘Z 撤销
- 历史记录自动保存到 `~/.jsonfmt/history.json`，保留最近 200 条，可搜索、加载、删除

## 安装

需要 Go 1.25+，仅支持 macOS（用 launchd 开机自启）：

```bash
./install.sh
```

装好后打开 http://127.0.0.1:7788 。登录后自动启动，进程退出会被自动拉起。改了代码重新执行一次 `./install.sh` 即可。

日志在 `~/.jsonfmt/jsonfmt.log`。

卸载（历史记录保留在 `~/.jsonfmt`）：

```bash
launchctl bootout gui/$(id -u)/local.jsonfmt && rm ~/Library/LaunchAgents/local.jsonfmt.plist
```

## 目录

- `main.go`：HTTP 服务和历史记录接口
- `index.html`：页面
- `codemirror.js`：打包好的 CodeMirror 编辑器，由 `web/` 生成
- `web/`：编辑器打包配置，只有升级编辑器时才需要 Node：`cd web && npm install && npm run build`
