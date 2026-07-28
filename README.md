# SmartInspectPlatform（华微智检）

[![CI](https://github.com/CIITRS/SmartInspectPlatform/actions/workflows/ci.yml/badge.svg)](https://github.com/CIITRS/SmartInspectPlatform/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/CIITRS/SmartInspectPlatform)](https://github.com/CIITRS/SmartInspectPlatform/releases)
[![License](https://img.shields.io/badge/license-AGPL--3.0-blue.svg)](LICENSE)

患者、样本、检测结果和报告的一体化管理平台。当前版本为 **v0.1**，包含管理后台、Go API 服务和微信小程序。

> 本系统用于医疗检测业务流程管理，不能替代医生诊断或治疗建议。部署和使用时应遵守所在地的医疗、隐私与数据安全法规。

## 功能

- 患者档案、邀请码录入、随访和历史报告
- 样本新增、接收、批次、检测结果和模型计算
- PDF 报告生成、审核、发布、趋势展示和备注
- 销售套餐、订单、检测计划与统计
- 管理后台角色权限、短信、微信、AI 和报告模板设置
- 七牛云对象存储：服务端上传凭证、直传、删除、空间用量与文件目录浏览
- GitHub Release 版本检查和 Linux 服务器自动升级

患者报告对象使用统一路径：

```text
uploads/patient_report/HW患者编号/HW患者编号_YYYYMMDDHHmmss_reportNN.pdf
```

一个上传文件对应一份患者报告；患者编辑页面可查看和删除已上传报告。

## 项目结构

```text
.
├── huawei-go/       # Go 1.25 + CloudWeGo Hertz API、PDF 与数据库逻辑
├── huawei-ui/       # React 19 + TypeScript + Umi Max 管理后台
├── huawei-uni/      # uni-app 微信小程序
├── .github/         # CI 和 Release 工作流
├── CHANGELOG.md
└── VERSION
```

## 环境要求

- Go 1.25.5
- Node.js 20 或更高版本（CI 使用 Node.js 22）
- MySQL 8.0 或更高版本
- Redis（可选；不可用时部分缓存功能降级）
- `pdfcpu` 命令行工具（报告盖章/条形码流程需要）
- Linux 自动升级还需要 Bash、Git、systemd、npm 和 Go

## 本地启动

### 1. 数据库

```sql
CREATE DATABASE huawei_micro_diagnosis
  CHARACTER SET utf8mb4
  COLLATE utf8mb4_unicode_ci;
```

### 2. 后端

```bash
cd huawei-go
cp .env.example .env
# 编辑 .env，至少设置 DB_*。
go mod download
go run .
```

默认监听 `http://localhost:3001`。启动时会检查并补齐必要表和字段；生产数据库仍应在升级前备份。

常用环境变量：

| 变量 | 说明 |
| --- | --- |
| `DB_HOST`、`DB_PORT`、`DB_USER`、`DB_PASSWORD`、`DB_NAME` | MySQL 连接 |
| `PORT` | API 监听端口，默认 `3001` |
| `REDIS_ADDR`、`REDIS_PASSWORD` | Redis 连接 |
| `AI_API_KEY`、`AI_API_URL`、`AI_MODEL` | AI 服务 |
| `QINIU_*` | 七牛配置的环境变量初始值 |
| `GITHUB_REPOSITORY` | 版本来源，默认 `CIITRS/SmartInspectPlatform` |
| `GITHUB_TOKEN` | 可选；提高 GitHub API 限额，私有仓库时必需 |
| `SYSTEM_UPGRADE_ENABLED` | 是否允许自动升级，默认启用；Windows 自动禁用 |
| `SYSTEM_UPGRADE_SCRIPT` | 自定义升级脚本路径 |
| `SMART_INSPECT_SERVICE` | systemd 服务名，默认 `huawei-go` |

不要提交 `.env`、数据库口令、AK/SK、微信密钥、RSA 私钥或证书。敏感系统设置在数据库中加密保存。

### 3. 管理后台

```bash
cd huawei-ui
npm ci
npm start
```

生产构建：

```bash
npm run build
```

后端直接托管管理后台时，将 `huawei-ui/dist/` 内容复制到 `huawei-go/static/`。

### 4. 微信小程序

`huawei-uni/` 是 uni-app 项目。使用 HBuilderX 或对应 uni-app CLI 构建微信小程序，并按部署环境配置 API 地址及微信 AppID/AppSecret。

## 七牛云对象存储

在管理后台进入“系统设置 → 文件存储”，填写：

- 启用开关
- AccessKey / SecretKey
- 空间名称（Bucket）
- 自定义访问域名
- 表单上传地址
- 上传凭证有效期

保存后页面会调用服务端管理接口读取对象总数、已用字节数、目录前缀和文件列表。SecretKey 只用于服务端签名；客户端仅取得短期上传凭证。

主要接口：

| 方法 | 地址 | 用途 |
| --- | --- | --- |
| `GET` | `/api/system/storage/qiniu/overview` | 用量与目录列表 |
| `GET` | `/api/upload/token` | 获取临时上传凭证 |
| `POST` | `/api/upload` | 服务端上传 |
| `DELETE` | `/api/upload` | 删除对象及关联记录 |

实现遵循七牛官方的[上传凭证](https://developer.qiniu.com/kodo/1208/upload-token)、[上传策略](https://developer.qiniu.com/kodo/1206/put-policy)、[表单上传](https://developer.qiniu.com/kodo/1272/form-upload)、[资源列举](https://developer.qiniu.com/kodo/api/list)和[管理凭证](https://developer.qiniu.com/kodo/1201/access-token)规范。

迁移旧患者报告到七牛：

```bash
cd huawei-go
go run ./cmd/migrate-patient-report-files
# 确认预览结果后：
go run ./cmd/migrate-patient-report-files -apply
```

## 版本与升级

版本号保存在根目录 `VERSION`，变更记录保存在 `CHANGELOG.md`。编译时通过 `-ldflags` 注入版本、提交和构建时间；“系统 → 关于”页面会读取 GitHub 最新 Release 并提示升级。

```text
GET  /api/system/version
POST /api/system/version/upgrade
```

自动升级仅支持 Linux。`POST` 接口只接受 GitHub 最新 Release 的合法版本标签，后台执行 `huawei-go/scripts/upgrade.sh`：拉取标签、在隔离 worktree 构建、备份当前程序、原子替换文件并重启 systemd 服务。日志写入 `huawei-go/logs/upgrade.log`。

服务器必须是本仓库的 Git checkout，并建议提前测试：

```bash
sudo systemctl status huawei-go
cd /path/to/SmartInspectPlatform/huawei-go
bash scripts/upgrade.sh v0.1
```

自动升级会改动运行文件并重启服务。生产使用前应配置数据库与文件备份、维护窗口、服务账户权限和回滚预案。

## 测试和发布

```bash
cd huawei-go
go test ./...
go vet ./...

cd ../huawei-ui
npm ci
npm run build
```

推送 `v*` 标签会运行 GitHub Release 工作流，生成 Linux amd64 发布包和 SHA-256 校验文件：

```bash
git tag -a v0.1 -m "SmartInspectPlatform v0.1"
git push origin v0.1
```

## 许可证

本项目使用 [GNU Affero General Public License v3.0](LICENSE)。分发或通过网络提供修改后的程序时，请遵守 AGPL-3.0 的源代码提供义务。第三方依赖仍适用各自许可证。

版权所有 © 2026 中创智科（上海）科技研究有限公司、华微智检医疗科技（哈尔滨）有限公司。
