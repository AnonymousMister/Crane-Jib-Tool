#!/usr/bin/env node
import os from 'os';
import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';
import * as tar from 'tar';
import AdmZip from 'adm-zip';

import { VERSION } from './version.js';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const REPO = 'AnonymousMister/Crane-Jib-Tool';

/**
 * 获取平台名称
 */
function getPlatformName() {
    const platform = os.platform();
    const arch = os.arch();

    if (platform === 'win32') return 'Windows_x86_64';
    if (platform === 'darwin') return arch === 'arm64' ? 'Darwin_arm64' : 'Darwin_x86_64';
    return arch === 'arm64' ? 'Linux_arm64' : 'Linux_x86_64';
}

/**
 * 获取用户目录下的安装路径
 */
function getInstallDir() {
    const homeDir = os.homedir();
    const platformName = getPlatformName();
    return path.join(homeDir, '.crane-jib-tool', VERSION, platformName);
}

/**
 * 获取二进制文件完整路径
 */
export function getBinaryPath() {
    const platform = os.platform();
    const exeName = platform === 'win32' ? 'crane-jib-tool.exe' : 'crane-jib-tool';
    return path.join(getInstallDir(), exeName);
}

/**
 * 解压二进制文件
 * @param {Buffer} buffer - 压缩文件的 buffer
 * @param {string} fileExt - 文件扩展名 (zip 或 tar.gz)
 * @param {string} exeName - 可执行文件名
 * @param {string} targetDir - 目标目录
 * @param {string} platform - 平台类型
 */
async function extractBinary(buffer, fileExt, exeName, targetDir, platform) {
    // 确保目标目录存在
    fs.mkdirSync(targetDir, { recursive: true });

    const targetPath = path.join(targetDir, exeName);

    if (fileExt === 'zip') {
        // 使用 zip 解压
        const zip = new AdmZip(buffer);
        const zipEntries = zip.getEntries();

        // 查找并提取目标文件
        let found = false;
        for (const entry of zipEntries) {
            if (entry.entryName === exeName) {
                zip.extractEntryTo(entry, targetDir, false, true);
                found = true;
                break;
            }
        }

        if (!found) {
            throw new Error(`zip 文件中未找到 ${exeName} 文件`);
        }
    } else {
        // 使用 tar 解压
        await new Promise((resolve, reject) => {
            const writer = tar.x({
                cwd: targetDir,
                sync: true,
            }, [exeName]);

            // 将 buffer 写入解压器
            writer.end(buffer);
            resolve();
        });
    }

    // 赋予执行权限 (非 Windows 系统)
    if (platform !== 'win32') {
        fs.chmodSync(targetPath, 0o755);
    }

    if (!fs.existsSync(targetPath)) {
        throw new Error('解压过程未产生预期的二进制文件');
    }

    return targetPath;
}

/**
 * 暴露给 build.js 调用的安装函数
 */
export async function installCrane() {
    const platform = os.platform();
    const platformName = getPlatformName();
    const installDir = getInstallDir();
    const targetPath = getBinaryPath();

    // 根据平台选择不同的文件格式
    const fileExt = platform === 'win32' ? 'zip' : 'tar.gz';
    const exeName = platform === 'win32' ? 'crane-jib-tool.exe' : 'crane-jib-tool';

    // 如果文件已存在，直接返回
    if (fs.existsSync(targetPath)) {
        console.log(`✅ 已检测到 Crane-Jib-Tool v${VERSION} (${platformName}): ${targetPath}`);
        return targetPath;
    }

    console.log(`[Crane-Jib-Tool] 正在安装 v${VERSION} (${platformName})...`);
    console.log(`📁 安装目录: ${installDir}`);

    let buffer = null;
    let source = '';

    // 第一步：尝试使用本地 lib 目录下的方案
    try {
        console.log('🔄 尝试使用本地 lib 目录下的方案...');
        const libDir = path.join(__dirname, '../lib');
        const localArchivePath = path.join(libDir, `crane-jib-tool_${platformName}.${fileExt}`);
        if (fs.existsSync(localArchivePath)) {
            try {
                buffer = fs.readFileSync(localArchivePath);
                source = `本地文件: ${localArchivePath}`;
                console.log(`✅ 成功加载本地备用文件: ${localArchivePath}`);
            } catch (readErr) {
                throw new Error(`加载本地文件失败: ${readErr.message}`);
            }
        } else {
            throw new Error(`本地 lib 目录下未找到备用文件`);
        }
    } catch (readErra) {
        console.log('ℹ️ 本地文件不可用:', readErra.message);
        // 第二步：尝试从 GitHub Releases 下载
        try {
            const url = `https://github.com/${REPO}/releases/download/v${VERSION}/crane-jib-tool_${platformName}.${fileExt}`;
            console.log(`🔄 正在从 GitHub 下载: ${url}`);
            const response = await fetch(url);
            if (!response.ok) throw new Error(`下载失败: HTTP ${response.status}`);
            buffer = Buffer.from(await response.arrayBuffer());
            source = 'GitHub Releases';
        } catch (downloadErr) {
            throw new Error(`从 GitHub Releases 下载失败: ${downloadErr.message}`);
        }
    }

    // 第三步：解压文件
    try {
        console.log(`🔄 正在解压文件 (来源: ${source})...`);
        await extractBinary(buffer, fileExt, exeName, installDir, platform);

        console.log(`✅ Crane-Jib-Tool 安装就绪: ${targetPath}`);
        return targetPath;
    } catch (extractErr) {
        console.error('❌ 解压文件失败:', extractErr.message);
        throw new Error(`解压失败: ${extractErr.message}`);
    }
}

// 支持直接运行
if (process.argv[1] === __filename) {
    installCrane().catch(() => process.exit(1));
}
