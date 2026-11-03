"""固定原图和委派失败守卫；仅标准库，不导入上游、不联网或调用模型。"""

import copy
import importlib.util
from pathlib import Path
from types import SimpleNamespace
import unittest


spec = importlib.util.spec_from_file_location("tradingcn_live", Path(__file__).with_name("verify-tradingcn-live.py"))
live = importlib.util.module_from_spec(spec)
spec.loader.exec_module(live)


class EvaluationAgentEvidence(unittest.TestCase):
    def plan(self):
        nodes = [SimpleNamespace(id=node, agent_id=node, config={}) for node in sorted(live.REQUIRED_INPUT_AGENTS)]
        next(node for node in nodes if node.id == "research_manager_v2").config["delegated_agent_ids"] = ["chip_distribution_analyst_v2"]
        return nodes

    def calls(self):
        return [{"node": node, "call_id": f"{index:032x}", "success": True} for index, node in enumerate(sorted(live.REQUIRED_INPUT_AGENTS), 1)]

    def test_fixed_plan_and_only_declared_delegation(self):
        self.assertTrue(live.evaluation_plan_valid(self.plan()))
        for mode in ["missing_main", "unknown_main", "wrong_identity", "missing_delegation", "unknown_delegation", "duplicate_delegation", "extra_parent", "invalid_config"]:
            with self.subTest(mode=mode):
                nodes = self.plan()
                manager = next(node for node in nodes if node.id == "research_manager_v2")
                if mode == "missing_main":
                    nodes.pop(0)
                elif mode == "unknown_main":
                    nodes[0].id = nodes[0].agent_id = "unknown_agent"
                elif mode == "wrong_identity":
                    nodes[0].agent_id = "trader_v2"
                elif mode == "missing_delegation":
                    manager.config.clear()
                elif mode == "unknown_delegation":
                    manager.config["delegated_agent_ids"] = ["unknown_agent"]
                elif mode == "duplicate_delegation":
                    manager.config["delegated_agent_ids"] *= 2
                elif mode == "extra_parent":
                    nodes[0].config = copy.deepcopy(manager.config)
                elif mode == "invalid_config":
                    nodes[0].config = None
                self.assertFalse(live.evaluation_plan_valid(nodes))

    def test_every_attempt_including_delegation_must_succeed(self):
        self.assertTrue(live.evaluation_calls_complete(self.calls(), None))
        delegated = self.calls() + [{"node": "chip_distribution_analyst_v2", "call_id": "f" * 32, "success": True}]
        self.assertTrue(live.evaluation_calls_complete(delegated, {}))
        for mode in ["failed_delegate", "missing_main", "delegate_replaces_main", "unknown", "duplicate_id", "malformed_id", "numeric_success", "missing_success", "extra", "null_call", "node_errors", "too_many"]:
            with self.subTest(mode=mode):
                calls, errors = copy.deepcopy(delegated), None
                if mode == "failed_delegate":
                    calls[-1]["success"] = False
                elif mode == "missing_main":
                    calls.pop(0)
                elif mode == "delegate_replaces_main":
                    calls[0]["node"] = "chip_distribution_analyst_v2"
                elif mode == "unknown":
                    calls[-1]["node"] = "unknown_agent"
                elif mode == "duplicate_id":
                    calls[-1]["call_id"] = calls[0]["call_id"]
                elif mode == "malformed_id":
                    calls[-1]["call_id"] = "invalid"
                elif mode == "numeric_success":
                    calls[-1]["success"] = 1
                elif mode == "missing_success":
                    del calls[-1]["success"]
                elif mode == "extra":
                    calls[-1]["extra"] = True
                elif mode == "null_call":
                    calls[-1] = None
                elif mode == "node_errors":
                    errors = {"research_manager_v2": "仅测试内部失败"}
                elif mode == "too_many":
                    calls *= 9
                self.assertFalse(live.evaluation_calls_complete(calls, errors))


if __name__ == "__main__":
    unittest.main(verbosity=2)
