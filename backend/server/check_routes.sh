#!/bin/bash

echo "=== 检查后端服务和路由 ==="
echo ""

# 检查服务是否运行
echo "1. 检查服务进程"
if pgrep -f "go run main.go" > /dev/null || pgrep -f "./app" > /dev/null; then
    echo "✓ 后端服务正在运行"
    ps aux | grep -E "go run main.go|./app" | grep -v grep
else
    echo "✗ 后端服务未运行"
    echo ""
    echo "请启动服务："
    echo "  cd server && go run main.go"
fi

echo ""
echo "---"
echo ""

# 检查端口
echo "2. 检查端口占用"
if lsof -i :8080 > /dev/null 2>&1; then
    echo "✓ 端口 8080 已被占用（服务可能在运行）"
    lsof -i :8080
else
    echo "✗ 端口 8080 未被占用（服务未运行）"
fi

echo ""
echo "---"
echo ""

# 测试 API
echo "3. 测试 API 接口"
echo ""

echo "测试: GET /api/quant/news/checkMeilisearchConsistency"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/api/quant/news/checkMeilisearchConsistency 2>/dev/null)
if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "401" ]; then
    echo "✓ 接口可访问 (HTTP $HTTP_CODE)"
elif [ "$HTTP_CODE" = "404" ]; then
    echo "✗ 接口返回 404 - 需要重启服务"
elif [ "$HTTP_CODE" = "000" ]; then
    echo "✗ 无法连接到服务器 - 服务未运行"
else
    echo "? 接口返回 HTTP $HTTP_CODE"
fi

echo ""

echo "测试: POST /api/quant/news/fullSyncToMeilisearch"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST http://localhost:8080/api/quant/news/fullSyncToMeilisearch 2>/dev/null)
if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "401" ]; then
    echo "✓ 接口可访问 (HTTP $HTTP_CODE)"
elif [ "$HTTP_CODE" = "404" ]; then
    echo "✗ 接口返回 404 - 需要重启服务"
elif [ "$HTTP_CODE" = "000" ]; then
    echo "✗ 无法连接到服务器 - 服务未运行"
else
    echo "? 接口返回 HTTP $HTTP_CODE"
fi

echo ""
echo "---"
echo ""

# 检查代码文件
echo "4. 检查代码文件"
if [ -f "api/v1/quant/news.go" ]; then
    if grep -q "CheckMeilisearchConsistency" api/v1/quant/news.go; then
        echo "✓ CheckMeilisearchConsistency 函数已定义"
    else
        echo "✗ CheckMeilisearchConsistency 函数未找到"
    fi
    
    if grep -q "FullSyncToMeilisearch" api/v1/quant/news.go; then
        echo "✓ FullSyncToMeilisearch 函数已定义"
    else
        echo "✗ FullSyncToMeilisearch 函数未找到"
    fi
else
    echo "✗ api/v1/quant/news.go 文件未找到"
fi

echo ""

if [ -f "router/quant/news.go" ]; then
    if grep -q "checkMeilisearchConsistency" router/quant/news.go; then
        echo "✓ checkMeilisearchConsistency 路由已注册"
    else
        echo "✗ checkMeilisearchConsistency 路由未注册"
    fi
    
    if grep -q "fullSyncToMeilisearch" router/quant/news.go; then
        echo "✓ fullSyncToMeilisearch 路由已注册"
    else
        echo "✗ fullSyncToMeilisearch 路由未注册"
    fi
else
    echo "✗ router/quant/news.go 文件未找到"
fi

echo ""
echo "---"
echo ""

# 给出建议
echo "5. 建议操作"
echo ""

if [ "$HTTP_CODE" = "404" ]; then
    echo "⚠️  接口返回 404，需要重启后端服务："
    echo ""
    echo "   1. 停止当前服务（Ctrl+C 或 kill 进程）"
    echo "   2. 重新启动："
    echo "      cd server && go run main.go"
    echo ""
elif [ "$HTTP_CODE" = "000" ]; then
    echo "⚠️  服务未运行，请启动："
    echo ""
    echo "   cd server && go run main.go"
    echo ""
elif [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "401" ]; then
    echo "✓ 服务运行正常，API 接口可用"
    echo ""
    echo "如果前端仍然报错，请检查："
    echo "  - 浏览器控制台的网络请求"
    echo "  - 前端 API 配置是否正确"
    echo "  - 是否有跨域问题"
    echo ""
fi

echo "=== 检查完成 ==="
