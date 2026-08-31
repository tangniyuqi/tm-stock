#!/bin/bash

# 测试 Meilisearch API 接口

echo "=== 测试 Meilisearch API 接口 ==="
echo ""

# 基础 URL
BASE_URL="http://localhost:8080/api"

echo "1. 测试数据一致性检查接口"
echo "GET ${BASE_URL}/quant/news/checkMeilisearchConsistency"
curl -X GET "${BASE_URL}/quant/news/checkMeilisearchConsistency" \
  -H "Content-Type: application/json" \
  -w "\nHTTP Status: %{http_code}\n" \
  2>/dev/null || echo "请求失败"

echo ""
echo "---"
echo ""

echo "2. 测试全量同步接口"
echo "POST ${BASE_URL}/quant/news/fullSyncToMeilisearch"
curl -X POST "${BASE_URL}/quant/news/fullSyncToMeilisearch" \
  -H "Content-Type: application/json" \
  -w "\nHTTP Status: %{http_code}\n" \
  2>/dev/null || echo "请求失败"

echo ""
echo "---"
echo ""

echo "3. 测试搜索接口"
echo "GET ${BASE_URL}/quant/news/meilisearch?page=1&pageSize=10"
curl -X GET "${BASE_URL}/quant/news/meilisearch?page=1&pageSize=10" \
  -H "Content-Type: application/json" \
  -w "\nHTTP Status: %{http_code}\n" \
  2>/dev/null || echo "请求失败"

echo ""
echo "=== 测试完成 ==="
