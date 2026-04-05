# Visitor Tracker

访客统计系统，跟踪真实用户与爬虫，收集浏览器指纹和 TLS 指纹。

## 功能特性

- **爬虫识别**：基于 User-Agent 关键词检测，阻止爬虫访问
- **浏览器指纹**：收集 Screen、Timezone、Language、Platform、Canvas、WebGL 等信息
- **TLS 指纹**：通过 JA3/JA4/Akamai 识别客户端 TLS 特征
- **访问统计**：按日期统计访问量、独立 IP、爬虫比例
- **访问日志**：记录每次访问的详细信息
- **管理后台**：查看统计数据和访问日志
- **数据持久化**：JSON 文件存储

## 快速开始

### 下载发布版

从 [Releases](https://github.com/tocha688/go-visitor-web/releases) 下载对应平台的压缩包，解压后运行：

```bash
# Linux
./visitor-linux-amd64

# Windows
visitor-windows-amd64.exe
```

### 配置

修改 `config.yaml`：

```yaml
app:
  host: "0.0.0.0"        # 监听地址
  port: 8080             # 监听端口
  admin_password: "123456" # 管理后台密码
  target_url: "https://www.example.com" # 重定向目标地址
stats:
  visit_file: "data/visits.json" # 数据存储文件
```

### 访问

- 前台页面：`http://localhost:8080/任意路径`
- 管理后台：`http://localhost:8080/adm`

## 目录结构

```
.
├── main.go              # 主程序
├── config.yaml          # 配置文件
├── templates/           # HTML 模板
│   ├── index.html       # 入口页面
│   ├── loading.html     # 加载页面（收集指纹）
│   ├── dashboard.html   # 管理后台
│   └── login.html       # 登录页面
├── data/                # 数据目录（自动创建）
│   └── visits.json      # 访问数据
└── dist/                # 编译输出目录
```

## 编译

### 本地编译

```bash
# Windows
build.bat

# Linux/Mac
chmod +x build.sh
./build.sh
```

### 交叉编译

需要 Go 1.21+

```bash
# 编译所有平台
go build -ldflags="-s -w" -o visitor-windows-amd64.exe .
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o visitor-linux-amd64 .
```

## 发布

推送 v 开头的标签自动发布：

```bash
git tag v0.1.0
git push origin v0.1.0
```

## 工作原理

1. 用户访问任意路径（如 `/123`）进入入口页面
2. 入口页面等待 3 秒后跳转到加载页面
3. 加载页面通过 JavaScript 收集：
   - 浏览器指纹（screen, timezone, canvas, webgl 等）
   - IP 和地理位置（通过 ipapi.co）
   - TLS 指纹（通过 get.ja3.zone）
4. 数据 POST 到服务器后跳转到目标 URL
5. 服务器记录访问日志和统计数据

## 免责声明

本工具仅用于正常的网站访问统计目的。请勿用于：
- 恶意追踪用户
- 绕过反爬虫机制
- 其他非法用途

使用本工具产生的任何问题由使用者自行承担。
