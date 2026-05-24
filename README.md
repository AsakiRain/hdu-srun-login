# hdu-srun-login

杭州电子科技大学校园网 Wi-Fi 登录 / 深澜（srun）校园网模拟登录

基于 Go 的深澜校园网自动登录工具，适配 2024 年暑假后的杭州电子科技大学校园网认证，支持生活区和教学区登录认证，支持多用户账号定时切换登录。

## 功能

- 自动探测可用认证入口
- 自动获取本机校园网 IP
- 定时检查在线状态，默认每 2 分钟检查一次
- 定时刷新登录，默认每 6 小时注销并重新登录一次
- 多账号随机重试，适合账号池轮换
- 写入 `srun_login.log`，日志达到 10 MB 后轮转

## 开始使用

需要 Go 1.22 或更高版本。

```bash
git clone https://github.com/AsakiRain/hdu-srun-login.git
cd hdu-srun-login && go build -o hdu-srun-login .
```

创建 `config.yaml`：

```bash
cp config.example.yaml config.yaml
# 编辑 config.yaml 填入账号密码
```

运行：

```bash
nohup ./hdu-srun-login &
```

常用参数：

```bash
./hdu-srun-login \
  -config config.yaml \
  -check-interval 2m \
  -refresh-interval 6h
```

执行一次状态检查和登录：

```bash
./srun-login-go -once
```

## systemd

将二进制和 `auth.json` 放在同一个目录，例如 `/opt/srun-login-go`。

```ini
[Unit]
Description=srun login
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=/opt/srun-login-go
ExecStart=/opt/srun-login-go/srun-login-go -config auth.json
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

启用服务：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now srun-login-go.service
sudo journalctl -u srun-login-go.service -f
```

## License

MIT License
