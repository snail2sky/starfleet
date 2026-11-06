# Starfleet on Termux (Android 手机)

在 Android/Termux 上部署 starfleet 的适配说明与当前 fleet 状态。

## 为什么需要 static-pie 构建

部分 Android OEM 内核（实测 Realme/ColorOS）会拒绝执行非 PIE 的 ET_EXEC 二进制，
报错 `error: "..." has unexpected e_type: 2`。因此：

- `Makefile` 的构建行已加 `-buildmode=pie`
- `release.yml` 的 CI 构建对 linux 目标同样启用 `-buildmode=pie`
- 同一二进制在普通 Linux 上完全兼容（static-pie 向后兼容）

## Termux 平台映射（相对 orangepi 原始布局）

| orangepi | Termux 手机 | 说明 |
|---|---|---|
| `/data0/apps/<app>` | `~/data0/apps/<app>` | Android 无 /data0 |
| `/data0/logs/<app>` | `~/data0/logs/<app>` | 同上 |
| `/home/orangepi/bin/report.sh` | `~/bin/report.sh` | 用户目录不同 |
| systemd 托管 | `~/.bashrc` 自启 + Termux:Boot | Termux 无 systemd |
| `hostname -I` | `ifconfig` 解析 | Termux 的 inetutils 无 `-I` |

## 当前 fleet 状态

| 节点 | 访问端点 | starfleet | 状态 |
|---|---|---|---|
| orangepi (orangepizero2) | hub.snail2sky.xubi.org:30022 | v1.7.2 原版 | ✅ 在线, systemd 托管 |
| Realme RMX1851 | `100.110.7.78:8022` (tailscale) | v1.7.3 pie, report.yml 已配 | ⏸ 依赖 Termux 会话存活 |
| byron (本机) | `100.79.207.64` (tailscale) | v1.7.3 pie | ✅ 本机部署 |

## 飞书状态上报

`report.sh` 每 5 分钟由 starfleet `report.yml` 调度，上报 time/uptime/LAN/public/tailnet/ssh
到飞书 webhook（与 orangepi 同一 webhook）。

## 附：Termux:Boot 开机自启

`boot/start-services` 放入 `~/.termux/boot/`，配合 Termux:Boot 应用（F-Droid），
手机重启后自动拉起 sshd / tailscaled / starfleet。
