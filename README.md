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
- 支持安装为系统服务（Windows/Linux/macOS）

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
  --config config.yaml \
  --check-interval 2m \
  --refresh-interval 6h
```

执行一次状态检查和登录：

```bash
./hdu-srun-login --once
```

## 安装为系统服务

支持 Windows、Linux 和 macOS 平台。使用 `install` 命令可以自动注册系统服务：

- **Windows**: 注册为 Windows Service (SCM)，开机自动启动
- **Linux**: 注册为 systemd 服务，依赖网络可用后启动，失败时自动重启
- **macOS**: 注册为 launchd 服务，开机自动启动

### 安装服务

```bash
# 默认安装（程序到用户 bin 目录，配置到 ~/hdu-srun-login.yaml）
./hdu-srun-login install

# 指定程序安装目录
./hdu-srun-login install --bin-dir /path/to/bin

# 指定要安装的配置文件
./hdu-srun-login install --config /path/to/config.yaml
```

### 服务管理

```bash
# 启动服务
./hdu-srun-login start

# 停止服务
./hdu-srun-login stop

# 重启服务
./hdu-srun-login restart

# 查看服务状态
./hdu-srun-login status
```

### 卸载服务

```bash
./hdu-srun-login uninstall
```

### 配置文件路径

| 平台 | 程序安装目录 | 配置文件 |
|------|-------------|---------|
| Windows | `%LOCALAPPDATA%\bin` | `~/hdu-srun-login.yaml` |
| Linux | `~/.local/bin` | `~/hdu-srun-login.yaml` |
| macOS | `~/.local/bin` | `~/hdu-srun-login.yaml` |

## 跨平台编译

使用 Go 的交叉编译功能，可以为不同平台构建二进制文件：

### Linux / macOS (bash)

```bash
# Linux amd64
GOOS=linux GOARCH=amd64 go build -o hdu-srun-login-linux-amd64 .

# Linux arm64
GOOS=linux GOARCH=arm64 go build -o hdu-srun-login-linux-arm64 .

# Windows amd64
GOOS=windows GOARCH=amd64 go build -o hdu-srun-login-windows-amd64.exe .

# macOS amd64
GOOS=darwin GOARCH=amd64 go build -o hdu-srun-login-darwin-amd64 .

# macOS arm64 (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o hdu-srun-login-darwin-arm64 .

# Linux mipsle (嵌入式路由器等)
GOOS=linux GOARCH=mipsle go build -o hdu-srun-login-linux-mipsle .
```

### Windows PowerShell

```powershell
# 编译 Linux amd64
$env:GOOS="linux"; $env:GOARCH="amd64"; go build -o hdu-srun-login-linux-amd64 .

# 编译 Linux arm64
$env:GOOS="linux"; $env:GOARCH="arm64"; go build -o hdu-srun-login-linux-arm64 .

# 编译 Linux mipsle
$env:GOOS="linux"; $env:GOARCH="mipsle"; go build -o hdu-srun-login-linux-mipsle .
```

### Windows CMD

```cmd
REM 编译 Linux amd64
set GOOS=linux
set GOARCH=amd64
go build -o hdu-srun-login-linux-amd64 .

REM 编译 Linux arm64
set GOOS=linux
set GOARCH=arm64
go build -o hdu-srun-login-linux-arm64 .

REM 编译 Linux mipsle
set GOOS=linux
set GOARCH=mipsle
go build -o hdu-srun-login-linux-mipsle .
```

使用 `go tool dist list` 查看所有支持的平台。

## License

MIT License
