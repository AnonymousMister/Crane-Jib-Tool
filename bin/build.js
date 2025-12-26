#!/usr/bin/env node
import { execSync } from 'child_process';
import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';
import os from 'os';
import * as tar from 'tar';
import * as ini from 'ini';
import {installCrane} from "./install.js";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const platform = os.platform();
const isWin = platform === 'win32';
const CRANE_BIN = isWin ? 'crane.exe' : 'crane';
const CRANE_PATH = path.join(__dirname, CRANE_BIN);

/**
 * 核心：解析变量池
 * 优先级：环境变量 < 内置变量 < -f 文件变量 < -f 字符串变量
 */
function buildVarPool(args) {
    const now = new Date();
    const pad = (n) => String(n).padStart(2, '0');
    const defaultTimestamp = `${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(now.getDate())}${pad(now.getHours())}${pad(now.getMinutes())}${pad(now.getSeconds())}`;

    const pool = {
        ...process.env,
        TimestampTag: defaultTimestamp
    };

    for (let i = 0; i < args.length; i++) {
        if (args[i] === '-f' && args[i + 1]) {
            const val = args[i + 1];
            const fullPath = path.resolve(val);

            if (fs.existsSync(fullPath)) {
                const ext = path.extname(val).toLowerCase();
                const content = fs.readFileSync(fullPath, 'utf-8');
                if (ext === '.json') {
                    try { Object.assign(pool, JSON.parse(content)); } catch (e) { console.error(`❌ JSON 变量文件解析失败: ${val}`); }
                } else if (ext === '.ini') {
                    try { Object.assign(pool, ini.parse(content)); } catch (e) { console.error(`❌ INI 变量文件解析失败: ${val}`); }
                }
            } else if (val.includes('=')) {
                const [k, ...vParts] = val.split('=');
                pool[k.trim()] = vParts.join('=').trim();
            }
            i++;
        }
    }
    return pool;
}

/**
 * 核心：递归替换对象中的占位符
 */
function injectVars(obj, vars) {
    const str = JSON.stringify(obj);
    return JSON.parse(str.replace(/\${?(\w+)}?/g, (match, key) => {
        return vars[key] !== undefined ? vars[key] : match;
    }));
}

async function run() {
    // 确保二进制文件可用
    if (!fs.existsSync(CRANE_PATH)) {
        await installCrane();
    }

    const args = process.argv.slice(2);
    const tIndex = args.indexOf('-t');
    const templatePath = (tIndex > -1 && args[tIndex + 1]) ? path.resolve(args[tIndex + 1]) : null;

    if (!templatePath || !fs.existsSync(templatePath)) {
        console.error("❌ 错误: 请使用 -t 指定有效的模板文件。例如: crane-build -t crane.json");
        process.exit(1);
    }




    // 1. 变量处理
    const varPool = buildVarPool(args);
    const rawTemplate = JSON.parse(fs.readFileSync(templatePath, 'utf-8'));
    const cfg = injectVars(rawTemplate, varPool);

    // 2. 核心参数准备
    const IMAGE_BASE = cfg.image;
    const FROM_IMAGE = cfg.from || "nginx:stable-alpine";
    const FINAL_TAG = cfg.tag || varPool.TimestampTag;
    const FULL_IMAGE = `${IMAGE_BASE}:${FINAL_TAG}`;
    const isInsecure = IMAGE_BASE.includes('192.168.') || IMAGE_BASE.includes('localhost') || IMAGE_BASE.includes('10.');
    const flags = isInsecure ? '--insecure' : '';

    try {
        // 3. 认证逻辑
        const registryHost = IMAGE_BASE.split('/')[0];
        if (varPool.DOCKER_USER && varPool.DOCKER_PASS) {
            console.log(`🔑 [Crane-Build] 正在认证: ${registryHost}`);
            execSync(`"${CRANE_PATH}" auth login ${registryHost} -u "${varPool.DOCKER_USER}" -p "${varPool.DOCKER_PASS}" ${flags}`, { stdio: 'inherit' });
        }

        console.log(`\n🚀 [Crane-Build] 变量处理完成`);
        console.log(`   最终镜像名: ${FULL_IMAGE}`);

        const tmpBase = path.join(process.cwd(), '.crane_tmp');
        if (fs.existsSync(tmpBase)) fs.rmSync(tmpBase, { recursive: true, force: true });
        fs.mkdirSync(tmpBase, { recursive: true });

        const layersToAppend = [];

        // 4. 层打包逻辑
        if (cfg.layers && Array.isArray(cfg.layers)) {
            for (const layer of cfg.layers) {
                console.log(`📦 [Crane-Build] 打包层: ${layer.name}`);
                const layerDir = path.join(tmpBase, `l_${layer.name}`);
                fs.mkdirSync(layerDir, { recursive: true });

                for (const mapping of layer.files) {
                    const src = path.resolve(process.cwd(), mapping.from);
                    const dest = path.join(layerDir, mapping.to);
                    fs.mkdirSync(path.dirname(dest), { recursive: true });
                    if (fs.existsSync(src)) {
                        if (fs.lstatSync(src).isDirectory()) fs.cpSync(src, dest, { recursive: true });
                        else fs.copyFileSync(src, dest);
                    } else {
                        console.warn(`⚠️ [Crane-Build] 警告: 文件不存在 ${src}`);
                    }
                }

                const layerTar = path.join(tmpBase, `${layer.name}.tar`);
                await tar.c({ gzip: false, file: layerTar, cwd: layerDir }, ['.']);
                layersToAppend.push(`-f "${layerTar}"`);
            }
        }

        // 5. 推送层数据
        if (layersToAppend.length > 0) {
            console.log(`\n🚚 [Crane-Build] 正在执行合并并推送内容层...`);
            execSync(`"${CRANE_PATH}" append -b ${FROM_IMAGE} ${layersToAppend.join(' ')} -t ${FULL_IMAGE} ${flags}`, { stdio: 'inherit' });
        }

        // 6. 修改元数据
        console.log(`\n🔧 [Crane-Build] 正在修改镜像运行元数据 (Expose/Envs)...`);
        let mutateCmd = `"${CRANE_PATH}" mutate ${FULL_IMAGE} -t ${FULL_IMAGE} ${flags}`;
        (cfg.exposedPorts || []).forEach(p => mutateCmd += ` --exposed-ports ${p}`);
        Object.entries(cfg.envs || {}).forEach(([k, v]) => mutateCmd += ` --env ${k}="${v}"`);
        execSync(mutateCmd, { stdio: 'inherit' });

        // 7. 清理
        console.log(`\n🧹 [Crane-Build] 正在清理临时打包文件...`);
        fs.rmSync(tmpBase, { recursive: true, force: true });

        console.log(`\n✨ [成功] 镜像发布完成！`);
        console.log(`   👉 镜像地址: ${FULL_IMAGE}\n`);

    } catch (error) {
        console.error(`\n❌ [Crane-Build] 构建中断`);
        if (error.stderr) console.error(error.stderr.toString());
        else console.error(error.message);
        process.exit(1);
    }
}

run();