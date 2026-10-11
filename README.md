# fnos-media-organizer

**飞牛影视整理器**：使用飞牛影视已有识别结果，检查文件命名和目录规范，预览变化后直接移动、重命名影视文件。

[![Build](https://github.com/laincarl/fnos-media-organizer/actions/workflows/build.yml/badge.svg)](https://github.com/laincarl/fnos-media-organizer/actions/workflows/build.yml)
[![Release](https://img.shields.io/github/v/release/laincarl/fnos-media-organizer)](https://github.com/laincarl/fnos-media-organizer/releases/latest)

适用于 **fnOS x86 / amd64**，以 Go 编写，无 Docker、Node.js 或 Python 运行依赖。此项目为独立第三方应用。

## 功能

- 通过 fnOS 统一网关和 NAS OAuth 连接影视账号，不共享浏览器 Cookie。
- 自动读取媒体库目录、封面、片名、年份、季集和单集名称，无需填写源目录和目标目录。
- 读取识别结果时自动检查规范；支持搜索、状态筛选、表头全选和 Shift 连选。
- 自定义电影和剧集的目录、文件名模板，保留原扩展名。
- 确认弹窗展示路径差异，已有目标不覆盖，跨文件系统不移动。
- 顶部任务浮层按“进行中 / 已完成 / 失败”显示结果，整理期间锁定勾选。
- 整理成功后清理空的源目录及空上级目录，保留影视库根目录、授权根目录及所有非空目录。
- 深色界面、移动端卡片列表，顶部和底部固定，内容区滚动。

## 安装与使用

1. 从 [最新 Release](https://github.com/laincarl/fnos-media-organizer/releases/latest) 下载 `fnos-media-organizer_<版本>_x86.fpk`。
2. 在 fnOS 应用中心选择“手动安装”，上传安装包。升级时直接覆盖安装。
3. 从 NAS 应用入口打开“飞牛影视整理器”，使用 NAS 管理员账号授权连接影视。
4. 选择媒体库，点击“读取识别结果”。**扫描和基于识别路径的规范检查无需目录授权。**
5. 勾选待整理文件，点击“整理所选文件”，核对预览后确认。
6. 若提示权限不足，按弹窗指引在 fnOS 系统设置 → 应用 → 飞牛影视整理器中授权对应目录，然后重启应用、重新连接。
7. 完成后在飞牛影视中重新扫描媒体库，使其更新文件路径。

文件操作需要系统授予的目录访问和写入权限。NAS 账号授权用于读取影视数据，不能替代文件系统授权。影视凭据仅保存在服务内存，最长 8 小时；应用重启后需重新授权。

## 默认命名规则

| 类型 | 目录模板 | 文件名模板（不填扩展名） |
| --- | --- | --- |
| 电影 | `{category}/{title} ({year})` | `{title} ({year})` |
| 剧集 | `{category}/{title} ({year})/Season {season}` | `{title} - S{season}E{episode} - {episode_title}` |

变量：`{title}` 片名、`{year}` 年份、`{category}` 分类目录、`{season}` 季、`{episode}` 集、`{episode_title}` 单集名称。季集号补齐为两位；分类仅用于目录，季集和单集名称仅用于剧集。

例如：

```text
电影/示例电影 (2024)/示例电影 (2024).mkv
电视剧/示例剧集 (2024)/Season 01/示例剧集 - S01E02 - 重逢.mkv
```

库目录已名为“电影”“电视剧”“剧集”或“综艺”时省略分类层，避免重复目录。默认剧集规则在没有单集名称时省略名称及对应分隔符。设置页可实时预览自定义规则；旧默认规则会自动迁移，自定义规则保留。

## 整理边界

- 每个文件在自己所属的媒体库目录内整理，不集中到其他库目录。
- Linux 使用 `RENAME_NOREPLACE` 原子移动，不复制文件内容、不覆盖目标。不支持该操作时保留原文件并报错。
- 缺少必要识别信息、重复目标、已有目标、源文件变化、软链接、路径越界或跨文件系统时拒绝移动。
- 只处理影视接口返回的已识别文件，不额外搬运字幕、海报和 NFO，不转码。
- 空目录清理仅沿成功移动文件的源目录向上进行，使用删除空目录的系统调用，不能递归删除非空目录。隐藏文件也会阻止清理。
- 清理失败不会将成功移动改成失败；清理目录和提示记录在任务明细及 `operations.jsonl` 中。
- 不修改飞牛影视数据库，不自动重新扫描。任务逐文件执行，不是批次事务，不提供自动撤回。

## 兼容性

当前适配依据：飞牛影视 **0.9.8 / mediasrv 0.8.42**，fnOS 需支持 `gatewayPrefix` 和 `gatewaySocket`。

应用通过 `/app/media-organizer/` 统一网关访问，仅监听安装目录内的 Unix socket。原 18765 端口已关闭。仓库名为 `fnos-media-organizer`；FPK 内部标识仍为 `media-organizer`，用于兼容既有安装及升级。

影视数据使用飞牛影视网页客户端的私有接口，后续升级可能需要适配。FN Connect 外网链路是否支持 OAuth `/signin` 仍需对应设备实测。

标准安装连接本机 `http://127.0.0.1:5666`；自定义部署可设置服务环境变量 `ORGANIZER_MEDIA_BASE`，但必须连接同一 NAS 影视实例。

## 目录结构

```text
cmd/fnos-media-organizer/  服务启动入口
internal/organizer/        影视接口、网关、命名规则、文件整理及对应测试
web/                      页面、样式、脚本和嵌入资源定义
packaging/fnos/            FPK 清单、权限配置、生命周期脚本、安装向导和桌面图标
scripts/                  构建、打包和服务验证脚本
docs/releases/            版本发布说明
.github/workflows/        自动构建与发布流水线
```

构建产物位于 `dist/`，下载的构建工具位于 `.tools/`，均不提交到 Git。Go 测试与对应业务代码放在同一包内，便于维护。

## 开发与构建

需要 Go 1.24+。FPK 必须在 Linux 文件系统内打包，确保生命周期脚本可执行。

```bash
go test ./...
go vet ./...
node --check web/app.js  # 仅开发检查需要 Node.js
bash scripts/build.sh
```

`scripts/build.sh` 从飞牛官方站点下载固定版本 fnpack 1.2.3 并校验 SHA-256，构建静态 Linux amd64 二进制，打包 FPK、验证服务生命周期，并在 `dist/` 输出安装包及校验文件。也可指定本地打包器：`FNPACK=/path/to/fnpack bash scripts/build.sh`。

网页通过 Go embed 嵌入二进制，修改网页后需要重新构建。Windows 可运行单元测试；生产移动逻辑和 FPK 生命周期在 Linux 验证。

### 自动构建与发布

- 推送 `main`、提交 Pull Request 或手动运行 Actions：测试、检查并构建 FPK，上传构建产物。
- 推送 `v*` 标签：要求标签版本与 `packaging/fnos/manifest` 一致，验证通过后自动创建 GitHub Release，上传 FPK 和 SHA-256。
- 发布步骤仅在标签任务中授予 `contents: write`，普通构建使用只读仓库权限。

发布新版本时更新 `packaging/fnos/manifest` 和 `docs/releases/v<版本>.md`，提交后执行：

```bash
git tag v0.3.21
git push origin main v0.3.21
```

## 问题反馈

请在 [Issues](https://github.com/laincarl/fnos-media-organizer/issues) 提供 fnOS / 影视版本、问题步骤和脱敏错误信息。不要提交账号密码、OAuth 授权码或影视 Token。路径含个人信息时请替换为示例路径。

[飞牛应用开发文档](https://developer.fnnas.com/docs/guide/) · [fnpack 文档](https://developer.fnnas.com/docs/cli/fnpack/)
