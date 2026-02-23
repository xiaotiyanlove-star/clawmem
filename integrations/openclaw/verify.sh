#!/usr/bin/env bash
# 快捷验证 OpenClaw 插件整合状态和连通性

set -e

echo "🔍 正在检查 ClawMem <-> OpenClaw 集成环境..."

# 检查当前目录
if [ ! -f "index.ts" ] || [ ! -f "openclaw.plugin.json" ]; then
    echo "❌ 错误: 请在 integrations/openclaw 目录下运行此脚本。"
    exit 1
fi

echo "✅ 核心插件文件就绪 (index.ts, openclaw.plugin.json)"

# 检查用户扩展目录安装情况
target_dir="$HOME/.openclaw/extensions/clawmem-integration"
if [ -d "$target_dir" ] && [ -f "$target_dir/index.ts" ] && [ -f "$target_dir/openclaw.plugin.json" ]; then
    echo "✅ 插件已安装到 OpenClaw 扩展目录: $target_dir"
else
    echo "⚠️ 警告: 尚未在目标路径检测到完整安装的插件，您可执行 'mkdir -p ~/.openclaw/extensions/clawmem-integration && cp index.ts openclaw.plugin.json ~/.openclaw/extensions/clawmem-integration/' 来完成安装。"
fi

# 检查后端服务基础连通性尝试
echo -n "🌐 测试 ClawMem 后端本地默认端口 (http://127.0.0.1:8090/health) 连通性... "
if curl -s -m 2 http://127.0.0.1:8090/health | grep -q 'ok'; then
    echo "成功"
else
    echo "未响应 (请确保您的 ClawMem 服务在此端口运行，或如果运行在远端请忽略此警告)"
fi

echo ""
echo "🎉 验证流程完成。如果您已正确配置 ~/.openclaw/openclaw.json，"
echo "请重启 OpenClaw Gateway (openclaw gateway restart)，"
echo "随后可使用 'openclaw plugins list' 来二次确认插件是否 loaded。"
echo "----------------------------------------------------"
