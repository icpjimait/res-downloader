## 下载安装文件

前往项目 Releases 页面下载适合你操作系统的安装包：  
👉 **[下载页面](https://github.com/icpjimait/res-downloader/releases)**

---

## 🪟 Windows 安装与运行
1. 下载 `res-downloader-windows-amd64.zip`。
2. 解压压缩包到任意目录。
3. 双击 `res-downloader.exe` 即可直接运行。
4. **提示**：
   - 首次运行建议右键选择“以管理员身份运行”以确保证书正确配置。
   - 点击窗口右上角关闭按钮时，软件将保持后台运行并**最小化至右下角系统托盘**。
   - 重复打开软件时，将自动激活并置顶已有窗口，不会重复创建进程。

---

## 🍎 macOS 安装与运行
1. 下载 `res-downloader-macos-universal.zip`（通用包，原生支持 Apple Silicon M1/M2/M3 及 Intel 芯片）。
2. 解压后将 `res-downloader.app` 拖入 `Applications`（应用程序）文件夹即可。
3. **如提示“已损坏”或“无法验证开发者”**：
   - 打开 Mac 终端执行：
   ```bash
   sudo xattr -d com.apple.quarantine /Applications/res-downloader.app
   ```

---

## 🐧 Linux 安装与运行
1. 下载 `res-downloader-linux-amd64.tar.gz`。
2. 解压并赋予执行权限：
   ```bash
   tar -zxvf res-downloader-linux-amd64.tar.gz
   chmod +x ./res-downloader
   ./res-downloader
   ```
3. **依赖要求**：系统需安装 `libgtk-3` 及 `webkit2gtk`（Ubuntu/Debian 用户可通过 `sudo apt install libgtk-3-0 libwebkit2gtk-4.0-37` 安装）。
