#!/usr/bin/env node
import { spawn } from 'child_process';
import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';
import { installCrane } from './install.js';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const isWin = process.platform === 'win32';
const CRANE_BIN = isWin ? 'crane-jib-tool.exe' : 'crane-jib-tool';
const CRANE_PATH = path.join(__dirname, CRANE_BIN);

async function main() {
    const args = process.argv.slice(2);
    
    // 处理 install 命令
    if (args.length > 0 && args[0] === 'install') {
        console.log('🔄 [Crane-Jib-Tool] 正在安装二进制文件...');
        await installCrane();
        console.log('✅ [Crane-Jib-Tool] 二进制文件安装完成');
        process.exit(0);
    }
    
    // 如果二进制文件不存在（比如 postinstall 被拦截了），现场下载
    if (!fs.existsSync(CRANE_PATH)) {
        console.log('⚠️ [Crane-Jib-Tool] 未发现二进制文件，正在准备环境...');
        await installCrane();
    }

    // 将用户输入的参数全部转发给真正的 crane 二进制文件
    const child = spawn(CRANE_PATH, args, {
        stdio: 'inherit', // 保持交互式输出，支持登录时的密码输入
        shell: false
    });

    child.on('exit', (code) => {
        process.exit(code);
    });
}

main().catch(err => {
    console.error('❌ 执行失败:', err);
    process.exit(1);
});