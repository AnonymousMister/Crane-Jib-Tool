#!/usr/bin/env node
import os from 'os';
import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';
import * as tar from 'tar'; // 引入 tar 模块

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const VERSION = '0.20.7';

/**
 * 暴露给 build.js 调用的安装函数
 */
export async function installCrane() {
    const platform = os.platform();
    const arch = os.arch();

    let platformName = '';
    if (platform === 'win32') platformName = 'Windows_x86_64';
    else if (platform === 'darwin') platformName = arch === 'arm64' ? 'Darwin_arm64' : 'Darwin_x86_64';
    else platformName = arch === 'arm64' ? 'Linux_arm64' : 'Linux_x86_64';

    const url = `https://github.com/google/go-containerregistry/releases/download/v${VERSION}/go-containerregistry_${platformName}.tar.gz`;
    const exeName = platform === 'win32' ? 'crane.exe' : 'crane';
    const targetPath = path.join(__dirname, exeName);

    // 如果文件已存在，直接返回
    if (fs.existsSync(targetPath)) return targetPath;

    console.log(`[Crane-Install] 正在下载并解压 Crane v${VERSION}...`);

    try {
        const response = await fetch(url);
        if (!response.ok) throw new Error(`下载失败: HTTP ${response.status}`);

        // 使用 tar.x 替代命令行解压
        // 直接将 fetch 的 body 管道流向 tar 解压器
        const readableStream = Buffer.from(await response.arrayBuffer());

        // 创建一个临时目录或在当前目录解压指定文件
        // tar.x ({ x: extract }) 参数说明：
        // cwd: 解压到的目录
        // filter: 只解压我们需要的那一个二进制文件
        await new Promise((resolve, reject) => {
            const writer = tar.x({
                cwd: __dirname,
                sync: true, // 使用同步或包装成 promise
            }, [exeName]);

            // 将 buffer 写入解压器
            writer.end(readableStream);
            resolve();
        });

        // 赋予执行权限 (非 Windows 系统)
        if (platform !== 'win32') {
            fs.chmodSync(targetPath, 0o755);
        }

        if (fs.existsSync(targetPath)) {
            console.log(`✅ Crane 安装就绪: ${targetPath}`);
            return targetPath;
        } else {
            throw new Error('解压过程未产生预期的二进制文件');
        }
    } catch (err) {
        console.error('❌ Crane 安装失败:', err.message);
        throw err;
    }
}

// 支持直接运行
if (process.argv[1] === __filename) {
    installCrane().catch(() => process.exit(1));
}
