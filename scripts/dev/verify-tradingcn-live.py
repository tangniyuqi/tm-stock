"""隔离调用固定社区版V2原工作流；模型凭据只存在于父进程网关。

从stdin读取测试配置，不读取产品.env；资料工具统一替换为显式测试资料。
本脚本不启动Web/队列、不发布报告、不连接本项目数据库。
"""

import json
import os
from pathlib import Path
import secrets
import socket
import sys
import traceback


def main():
    options = json.load(sys.stdin)
    root = Path(options["source"]).resolve(strict=True)
    output = Path(options["output"]).resolve(strict=True)
    port = int(options["port"])
    execute = options.get("execute") is True
    if not 1024 <= port <= 65535 or root.parent != output:
        raise ValueError("隔离目录或回环端口无效")
    # 原工具加载器按cwd计算模块名，必须在隔离源码根运行。
    os.chdir(root)
    os.environ.update({
        "DOTENV_FILE": str(output / "no-product-env"),
        "USE_MONGODB_STORAGE": "false",
        "JWT_SECRET": secrets.token_hex(32),
        "CSRF_SECRET": secrets.token_hex(32),
        "DEEPSEEK_API_KEY": options["session"],
        "DEEPSEEK_BASE_URL": f"http://127.0.0.1:{port}/v1",
        "AGENT_EXECUTION_TRACE_ENABLED": "false",
        "LANGCHAIN_TRACING_V2": "false",
        "ONLINE_TOOLS_ENABLED": "false",
        "REALTIME_DATA_ENABLED": "false",
    })
    connect = socket.socket.connect
    dns = socket.getaddrinfo
    denied = []

    def local_connect(self, address):
        if not isinstance(address, tuple) or address[:2] != ("127.0.0.1", port):
            denied.append("blocked_outside_model_gateway")
            raise OSError("测试子进程仅允许连接受控模型网关")
        return connect(self, address)

    def local_dns(host, *args, **kwargs):
        if host not in ("127.0.0.1", "::1", "localhost", None):
            denied.append("blocked_external_dns")
            raise OSError("测试子进程不允许外部DNS")
        return dns(host, *args, **kwargs)

    socket.socket.connect = local_connect
    socket.getaddrinfo = local_dns
    sys.path.insert(0, str(root))
    # 原库支持无数据库的代码配置模式。仅缩短本测试的无数据库探测等待，
    # 不伪造数据库结果，也不把内存替身冒充MongoDB/Redis验收。
    import pymongo
    mongo_init = pymongo.MongoClient.__init__
    clients = []

    def bounded_mongo(self, *args, **kwargs):
        kwargs.update(serverSelectionTimeoutMS=80, connectTimeoutMS=80, socketTimeoutMS=80)
        mongo_init(self, *args, **kwargs)
        clients.append(self)

    pymongo.MongoClient.__init__ = bounded_mongo
    import httpx
    from langchain_core.tools import StructuredTool
    import app.core.database as upstream_database

    def unavailable_database(*args, **kwargs):
        raise RuntimeError("隔离评估使用原库的无数据库代码配置模式")

    upstream_database.get_mongo_db_sync = unavailable_database
    from tradingagents.llm_adapters.deepseek_adapter import ChatDeepSeek
    from core.tools.registry import ToolRegistry
    from core.agents.factory import AgentFactory
    from core.agents.base import BaseAgent
    from core.workflow.engine import WorkflowEngine
    from core.workflow.templates.v2_stock_analysis_workflow import V2_STOCK_ANALYSIS_WORKFLOW

    calls = []
    tools = []

    class TrackedDeepSeek(ChatDeepSeek):
        # 固定上游的invoke直接调用_generate，绕过了LangChain标准回调。
        # 此观测钩子仍执行原_generate，不替换响应，不根据日志猜测调用成功。
        evaluation_node: str = ""

        def _generate(self, messages, stop=None, run_manager=None, **kwargs):
            event = {"node": self.evaluation_node, "success": False}
            calls.append(event)
            result = super()._generate(messages, stop, run_manager, **kwargs)
            event["success"] = True
            return result

    # 先以httpx本地传输替身验证直接invoke和bind_tools都能被观测。
    # 只验证计数器，不作为真实模型通过证据；不发送任何HTTP请求。
    def local_response(request):
        return httpx.Response(200, json={
            "id": "test-local-trace", "object": "chat.completion", "created": 1,
            "model": "deepseek-flash", "choices": [{"index": 0, "finish_reason": "stop", "message": {"role": "assistant", "content": "OK"}}],
            "usage": {"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
        })

    with httpx.Client(transport=httpx.MockTransport(local_response), trust_env=False) as mock_client:
        trace_probe = TrackedDeepSeek(model="deepseek-flash", api_key=options.get("session"), base_url=f"http://127.0.0.1:{port}/v1", http_client=mock_client, max_retries=0, evaluation_node="trace-selftest")
        trace_probe.invoke("仅本地计数器自测")
        trace_probe.bind_tools([{"type": "function", "function": {"name": "local_test", "description": "仅测试", "parameters": {"type": "object", "properties": {}}}}]).invoke("仅本地工具绑定计数器自测")
    if len(calls) != 2 or not all(event["success"] for event in calls):
        raise RuntimeError("本地计数器自测失败，不得启动真实评估")
    calls.clear()
    # 原V2并行开局会为同一Agent ID额外创建实例，_agents缓存只保留其中一个。
    # 在原工厂入口附加观测标签，覆盖全部实例和委派，不能只改缓存中的对象。
    original_create = AgentFactory.create
    factory_instances = []

    def observed_create(self, agent_id, config=None, llm=None, tool_ids=None, memory=None, **overrides):
        if isinstance(llm, TrackedDeepSeek):
            llm = llm.model_copy(update={"evaluation_node": agent_id})
        agent = original_create(self, agent_id, config, llm=llm, tool_ids=tool_ids, memory=memory, **overrides)
        factory_instances.append((agent_id, agent))
        return agent

    AgentFactory.create = observed_create
    original_dependencies = BaseAgent.set_dependencies

    def observed_dependencies(self, llm, toolkit=None):
        if isinstance(llm, TrackedDeepSeek):
            llm = llm.model_copy(update={"evaluation_node": self.agent_id})
        return original_dependencies(self, llm, toolkit)

    BaseAgent.set_dependencies = observed_dependencies

    fixture = {
        "scope": "synthetic_local_fixture",
        "subject": "TMEVAL（集成测试主体，非真实证券）",
        "notice": "仅用于接线测试，无真实行情、财报、新闻或投资用途。不得编造缺失数字。",
        "facts": [{"id": "fixture-1", "text": "测试设备用于处理样例数据。"}],
    }
    registry = ToolRegistry()
    # 保留原工具参数schema，仅替换外部资料实现；任何工具均不能访问网络/系统。
    for metadata in list(registry.get_all_tools()):
        original = registry.get_langchain_tool(metadata.id)
        if original is None:
            continue

        def fixture_tool(_tool_id=metadata.id, **kwargs):
            tools.append(_tool_id)
            return json.dumps(fixture, ensure_ascii=False)

        wrapped = StructuredTool.from_function(
            func=fixture_tool, name=metadata.id,
            description="仅返回明确标注的本地测试资料，不读取真实证券或外部服务。",
            args_schema=original.args_schema,
        )
        registry.register_function(
            metadata.id, wrapped, metadata.name, metadata.category,
            description=wrapped.description, is_online=False, override=True,
        )

    client = httpx.Client(trust_env=False, follow_redirects=False, timeout=90)
    llm = TrackedDeepSeek(
        model="deepseek-flash", api_key=options.get("session"),
        base_url=f"http://127.0.0.1:{port}/v1", max_tokens=2048,
        temperature=0, max_retries=0, timeout=90, http_client=client,
        thinking={"type": "disabled"}, cache=False,
    )
    definition = V2_STOCK_ANALYSIS_WORKFLOW.model_copy(deep=True)
    engine = WorkflowEngine(
        legacy_config={"memory_enabled": False, "online_tools": False}, llm=llm,
    )
    engine.load(definition).compile()
    required = {node.id for node in definition.nodes if node.agent_id}
    created = engine._builder._agents
    if len(required) != 14 or not required.issubset(created):
        raise RuntimeError("完整V2原Agent未全部创建")
    classes = {node: type(created[node]).__name__ for node in sorted(required)}
    if any(not type(created[node]).__module__.startswith("core.agents.") for node in required):
        raise RuntimeError("检测到占位或替代Agent")
    mismatches = {agent_id: getattr(agent._llm, "evaluation_node", None) for agent_id, agent in factory_instances if getattr(agent._llm, "evaluation_node", None) != agent_id}
    if not factory_instances or mismatches:
        raise RuntimeError("未覆盖原工厂全部执行实例，不得启动真实评估: " + json.dumps(mismatches))
    summary = {"scope": "upstream_v2_core_fixture_evaluation", "agents": classes, "factory_instances": len(factory_instances), "executed": False}
    try:
        if execute:
            inputs = {
                "ticker": "TMEVAL", "company_of_interest": "TMEVAL", "symbol": "TMEVAL",
                "company_name": fixture["subject"], "analysis_date": "2026-10-21",
                "trade_date": "2026-10-21", "market_type": "美股", "analysis_depth": 1,
                "_max_debate_rounds": 1, "_max_risk_rounds": 1,
                "skip_cache": True, "memory_facts_summary": json.dumps(fixture, ensure_ascii=False),
            }
            result = engine.execute(inputs, config={"recursion_limit": 180, "max_concurrency": 4})
            (output / "quarantined-result.json").write_text(json.dumps(result, ensure_ascii=False, default=str), encoding="utf-8")
            seen = {call["node"] for call in calls if call["success"]}
            missing = sorted(required - seen)
            summary.update(executed=True, model_calls=len(calls), tool_calls=len(tools), node_model_calls=calls, missing_agents=missing)
            if missing or result.get("_node_errors"):
                raise RuntimeError("存在未实调模型或失败的Agent: " + ",".join(missing))
        summary["verified"] = True
        print("TM_TRADINGCN_EVAL=" + json.dumps(summary, ensure_ascii=False), flush=True)
    finally:
        summary["blocked_connection_attempts"] = len(denied)
        (output / "evaluation-summary.json").write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")
        client.close()
        for mongo in clients:
            mongo.close()


if __name__ == "__main__":
    try:
        main()
    except Exception:
        traceback.print_exc()
        sys.exit(1)
