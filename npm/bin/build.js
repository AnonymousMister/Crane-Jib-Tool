#!/usr/bin/env node
import { execSync } from 'child_process';
import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';
import os from 'os';
import * as tar from 'tar';
import * as ini from 'ini';
import { installCrane } from "./install.js";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const platform = os.platform();
const isWin = platform === 'win32';
const CRANE_BIN = isWin ? 'crane.exe' : 'crane';
const CRANE_PATH = path.join(__dirname, CRANE_BIN);

const quote = (p) => `"${p}"`;

function buildVarPool(args) {
    const now = new Date();
    const pad = (n) => String(n).padStart(2, '0');
    const defaultTimestamp = `${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(now.getDate())}${pad(now.getHours())}${pad(now.getMinutes())}${pad(now.getSeconds())}`;
    const pool = { ...process.env, TimestampTag: defaultTimestamp };
    for (let i = 0; i < args.length; i++) {
        if (args[i] === '-f' && args[i + 1]) {
            const val = args[i + 1];
            const fullPath = path.resolve(val);
            if (fs.existsSync(fullPath)) {
                const content = fs.readFileSync(fullPath, 'utf-8');
                let parsed = {};
                if (val.endsWith('.json')) parsed = JSON.parse(content);
                else if (val.endsWith('.ini')) parsed = ini.parse(content);
                Object.assign(pool, parsed);
            } else if (val.includes('=')) {
                const [k, v] = val.split('=');
                pool[k] = v;
            }
        }
    }
    return pool;
}

function replaceVars(str, pool) {
    return str.replace(/\${(.*?)}/g, (_, key) => pool[key] || '');
}

async function run() {
    if (!fs.existsSync(CRANE_PATH)) await installCrane();

    const args = process.argv.slice(2);
    const varPool = buildVarPool(args);
    let configPath = '';
    for (let i = 0; i < args.length; i++) {
        if ((args[i] === '-t' || args[i] === '--template') && args[i + 1]) configPath = path.resolve(args[i + 1]);
    }
    if (!configPath || !fs.existsSync(configPath)) process.exit(1);

    const cfgRaw = JSON.parse(fs.readFileSync(configPath, 'utf-8'));
    const cfg = JSON.parse(replaceVars(JSON.stringify(cfgRaw), varPool));
    const FROM_IMAGE = cfg.from;
    const FINAL_TAG = cfg.tag;
    const IMAGE_BASE = cfg.image;
    const FULL_IMAGE = `${IMAGE_BASE}:${FINAL_TAG}`;
    let platforms = cfg.platform || ['linux/amd64'];
    if (!Array.isArray(platforms)) platforms = [platforms];
    const INSECURE_FLAG = cfg.insecure === true ? '--insecure' : '';

    const rootTmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'crane-job-'));
    const localTarRefs = [];

    try {
        for (const targetPlatform of platforms) {
            console.log(`\n🔨 [Crane-Build] 正在处理平台: [${targetPlatform}]`);
            const platformSuffix = targetPlatform.replace('/', '-');
            const platformDir = path.join(rootTmpDir, platformSuffix);
            fs.mkdirSync(platformDir);
            
            const TAR_NAME = 'image.tar'; 
            const baseFlags = `${INSECURE_FLAG} --platform ${targetPlatform}`;

            // 1. 打包 Layers
            const layersToAppend = [];
            if (cfg.layers) {
                for (const layer of cfg.layers) {
                    const layerDir = path.join(platformDir, layer.name || 'layer');
                    fs.mkdirSync(layerDir, { recursive: true });
                    for (const file of (layer.files || [])) {
                        const s = file.from || file.src;
                        const t = file.to || file.dest;
                        if (!s || !t) continue;
                        const srcAbs = path.resolve(s);
                        const destAbs = path.join(layerDir, t);
                        fs.mkdirSync(path.dirname(destAbs), { recursive: true });
                        if (fs.existsSync(srcAbs)) fs.cpSync(srcAbs, destAbs, { recursive: true });
                    }
                    const layerTar = path.join(platformDir, `${layer.name}.tar`);
                    await tar.c({ gzip: false, file: layerTar, cwd: layerDir }, ['.']);
                    layersToAppend.push(`-f ${quote(layerTar)}`);
                }
            }

            // 2. 合成本地 Tar
            console.log(`   └─ 合成本地镜像...`);
            execSync(`${quote(CRANE_PATH)} append -b ${FROM_IMAGE} ${layersToAppend.join(' ')} -o ${TAR_NAME} --new_tag ${FINAL_TAG} ${baseFlags}`, { 
                cwd: platformDir, 
                stdio: 'inherit' 
            });

            // 3. 修改元数据 - 核心修复：显式包含 ./ 且整体包裹在引号内
            console.log(`   └─ 注入配置 (Env/Port)...`);
            // 【关键点】 "tarball:./image.tar" 明确告诉 crane 这是个本地文件系统路径
            let mutateCmd = `${quote(CRANE_PATH)} mutate tarball:${quote(`./${TAR_NAME}`)}`;
            (cfg.exposedPorts || []).forEach(p => mutateCmd += ` --exposed-ports ${p}`);
            Object.entries(cfg.envs || {}).forEach(([k, v]) => mutateCmd += ` --env ${k}=${quote(v)}`);
            mutateCmd += ` -o ${TAR_NAME} ${baseFlags}`;
            
            execSync(mutateCmd, {
                cwd: platformDir, 
                stdio: 'inherit' 
            });

            localTarRefs.push(path.join(platformDir, TAR_NAME));
        }

        // 4. 多架构统一推送
        if (localTarRefs.length > 0) {
            console.log(`\n🚀 [Crane-Build] 正在执行多架构统一推送...`);
            // optimize 命令同样使用带 tarball: 的绝对路径包裹
            const manifests = localTarRefs.map(ref => quote(`tarball:${ref}`)).join(',');
            const optimizeCmd = `${quote(CRANE_PATH)} index optimize --manifests ${manifests} -t ${quote(FULL_IMAGE)} ${INSECURE_FLAG}`;
            execSync(optimizeCmd, { stdio: 'inherit' });
            console.log(`\n✅ 构建成功: ${FULL_IMAGE}`);
        }
    } catch (e) {
        console.error(`\n❌ 构建中断: ${e.message}`);
        process.exit(1);
    } finally {
        if (fs.existsSync(rootTmpDir)) {
            console.log(`\n🧹 清理临时目录: ${rootTmpDir}`);
            fs.rmSync(rootTmpDir, { recursive: true, force: true });
        }
    }
}
run();
