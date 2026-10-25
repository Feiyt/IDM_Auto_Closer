# IDM Auto Closer

IDM Auto Closer 是一个轻量级 Windows 工具，可监控 Internet Download Manager（IDM）的下载状态，并在下载完成后自动关闭 IDM。

## 功能

- 自动等待并检测 IDM。
- 监控 IDM 的文件读写活动。
- IDM 持续低活动约 10 秒后自动关闭。
- 自动加入当前用户的开机启动项，并在更新时替换旧版本。
- 后台运行，不显示终端窗口。

## 使用方法

### 1. 下载

从项目 Releases 页面下载最新的：

```text
IDM_Auto_Closer-Windows-版本号.zip
```

解压后，文件夹中包含：

```text
IDM_Auto_Closer-Windows-版本号.exe
Data Cleanup.exe
```

### 2. 启动

请先确保 IDM 已安装并可以正常使用，然后双击：

```text
IDM_Auto_Closer-Windows-版本号.exe
```

程序会在后台运行并自动监控 IDM，无需保持终端窗口。

### 3. 清除配置

如需移除本程序的开机启动项和相关配置：

1. 先退出 IDM Auto Closer。
2. 双击运行 `Data Cleanup.exe`。
3. 清除程序会匹配所有版本的 IDM Auto Closer 启动项。
4. 清理完成后会弹窗提示结果。

## 注意事项

- 本工具通过监控 IDM 进程的文件读写活动判断下载状态，不直接监控网络流量。
- 暂停下载、等待连接、文件校验或下载队列中的任务可能被识别为低活动。
- 自动关闭前请确认 IDM 中没有仍需保留的下载任务。
