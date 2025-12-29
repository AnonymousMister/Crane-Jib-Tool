#!/usr/bin/env node
import os from 'os';
import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';
import * as tar from 'tar'; 
import AdmZip from 'adm-zip';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const VERSION = '2.0.2'; // 可以从 package.json 中读取
const REPO = 'AnonymousMister/Crane-Jib-Tool';

/**
 * 解压二进制文件
 * @param {Buffer} buffer - 压缩文件的 buffer
 * @param {string} fileExt - 文件扩展名 (zip 或 tar.gz)
 * @param {string} exeName - 可执行文件名
 * @param {string} targetPath - 目标路径
 * @param {string} platform - 平台类型
 */
async function extractBinary(buffer, fileExt, exeName, targetPath, platform) {
    if (fileExt === 'zip') {
        // 使用 zip 解压
        const zip = new AdmZip(buffer);
        const zipEntries = zip.getEntries();
        
        // 查找并提取目标文件
        let found = false;
        for (const entry of zipEntries) {
            if (entry.entryName === exeName) {
                zip.extractEntryTo(entry, __dirname, false, true);
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
                cwd: __dirname,
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
}

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

    // 根据平台选择不同的文件格式
    const fileExt = platform === 'win32' ? 'zip' : 'tar.gz';
    const exeName = platform === 'win32' ? 'crane-jib-tool.exe' : 'crane-jib-tool';
    const targetPath = path.join(__dirname, exeName);
    
    // 如果文件已存在，直接返回
    if (fs.existsSync(targetPath)) {
        console.log(`✅ 已检测到 Crane-Jib-Tool 二进制文件: ${targetPath}`);
        return targetPath;
    }

    console.log(`[Crane-Jib-Tool] 正在安装 v${VERSION} (${platformName})...`);
    
    let buffer = null;
    let source = '';
    
    // 第一步：尝试从 GitHub Releases 下载
    try {
        const url = `https://github.com/${REPO}/releases/download/v${VERSION}/crane-jib-tool_${platformName}.${fileExt}`;
        console.log(`🔄 正在从 GitHub 下载: ${url}`);
        
        const response = await fetch(url);
        if (!response.ok) throw new Error(`下载失败: HTTP ${response.status}`);
        
        buffer = Buffer.from(await response.arrayBuffer());
        source = 'GitHub Releases';
    } catch (downloadErr) {
        console.error('❌ 从 GitHub Releases 下载失败:', downloadErr.message);
        
        // 第二步：尝试使用本地 lib 目录下的备用方案
        console.log('🔄 尝试使用本地 lib 目录下的备用方案...');
        
        const libDir = path.join(__dirname, '../lib');
        const localArchivePath = path.join(libDir, `crane-jib-tool_${platformName}.${fileExt}`);
        
        if (fs.existsSync(localArchivePath)) {
            try {
                buffer = fs.readFileSync(localArchivePath);
                source = `本地文件: ${localArchivePath}`;
                console.log(`✅ 成功加载本地备用文件: ${localArchivePath}`);
            } catch (readErr) {
                console.error('❌ 读取本地备用文件失败:', readErr.message);
                throw new Error(`下载和本地备用方案均失败: ${downloadErr.message}`);
            }
        } else {
            console.error(`❌ 本地 lib 目录下未找到备用文件: ${localArchivePath}`);
            throw new Error(`下载和本地备用方案均失败: ${downloadErr.message}`);
        }
    }
    
    // 第三步：解压文件（复用相同的解压逻辑）
    try {
        console.log(`🔄 正在解压文件 (来源: ${source})...`);
        await extractBinary(buffer, fileExt, exeName, targetPath, platform);
        
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
