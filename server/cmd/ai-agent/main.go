// ai-agent 是外部 Agent 的内部运维客户端，不提供 HTTP 入口或公开报告。
package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"
	"unicode/utf8"

	_ "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/agents"
	"github.com/tangniyuqi/tm-stock/server/internal/agents/tradingcn"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func readRequest(input io.Reader, submit bool) (agents.Request, error) {
	body, err := io.ReadAll(io.LimitReader(input, 1025))
	if err != nil || len(body) > 1024 || !utf8.Valid(body) {
		return agents.Request{}, agents.ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(body))
	tok, err := d.Token()
	if err != nil || tok != json.Delim('{') {
		return agents.Request{}, agents.ErrInvalid
	}
	seen := map[string]bool{}
	r := agents.Request{}
	for d.More() {
		tok, err := d.Token()
		if err != nil {
			return r, agents.ErrInvalid
		}
		key, ok := tok.(string)
		if !ok || seen[key] || key != "request_id" && (key != "symbol" || !submit) {
			return r, agents.ErrInvalid
		}
		seen[key] = true
		var value string
		if d.Decode(&value) != nil {
			return r, agents.ErrInvalid
		}
		if key == "request_id" {
			r.RequestID = value
		} else {
			r.Symbol = value
		}
	}
	if tok, err = d.Token(); err != nil || tok != json.Delim('}') {
		return r, agents.ErrInvalid
	}
	if _, err = d.Token(); err != io.EOF {
		return r, agents.ErrInvalid
	}
	if !agents.ValidRequestID(r.RequestID) || submit && !r.Valid() {
		return r, agents.ErrInvalid
	}
	return r, nil
}

func run(args []string, input io.Reader, output io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("须指定 version、submit、show、status、result 或 cancel；version 不读取输入或配置，其余命令从标准输入读取 JSON，凭据仅从环境变量读取")
	}
	op := args[0]
	if op == "version" {
		// 纯本地描述，不打开数据库、不读取凭据、不验证远端或启用执行。
		if json.NewEncoder(output).Encode(tradingcn.Baseline()) != nil {
			return fmt.Errorf("输出适配器版本失败")
		}
		return nil
	}
	switch op {
	case "submit", "show", "status", "result", "cancel":
	default:
		return fmt.Errorf("未知 Agent 命令")
	}
	req, err := readRequest(input, op == "submit")
	if err != nil {
		return err
	}
	actor, err := strconv.ParseInt(os.Getenv("TM_AI_AGENTS_ACTOR_ID"), 10, 64)
	if err != nil || actor <= 0 {
		return fmt.Errorf("须指定有效的内部操作人 TM_AI_AGENTS_ACTOR_ID，不能使用会员令牌或用户输入身份")
	}
	var executor agents.Executor
	var profile agents.Profile
	if op != "show" {
		cfg, err := config.LoadAIAgents()
		if err != nil {
			return err
		}
		client, err := tradingcn.New(cfg)
		if err != nil {
			return err
		}
		defer client.Close()
		executor = client
		profile = cfg.Profile()
	}
	dbCfg, err := config.Load()
	if err != nil {
		return err
	}
	db, err := sql.Open("mysql", dbCfg.DSN)
	if err != nil {
		return fmt.Errorf("打开 Agent 数据库失败")
	}
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
	defer cancel()
	if db.PingContext(ctx) != nil {
		return fmt.Errorf("Agent 数据库不可用")
	}
	repo := repository.NewAIAgentRepo(db)
	if op != "show" {
		if err := repo.VerifySchema(ctx); err != nil {
			return err
		}
	}
	svc, err := service.NewAIAgentService(repo, executor, profile)
	if err != nil {
		return err
	}
	var record agents.Record
	switch op {
	case "submit":
		record, err = svc.Submit(ctx, actor, req)
	case "show":
		record, err = svc.Show(ctx, actor, req.RequestID)
	case "status":
		record, err = svc.Status(ctx, actor, req.RequestID)
	case "result":
		record, err = svc.Result(ctx, actor, req.RequestID)
	case "cancel":
		record, err = svc.Cancel(ctx, actor, req.RequestID)
	}
	if record.ID > 0 {
		// 用量未知不能填 0；这里仅声明本工具没有计费/公开报告能力。
		v := struct {
			Record                agents.Record `json:"record"`
			BillableUsageKnown    bool          `json:"billable_usage_known"`
			PublicReportAvailable bool          `json:"public_report_available"`
		}{Record: record}
		if json.NewEncoder(output).Encode(v) != nil {
			return fmt.Errorf("输出 Agent 状态失败；请用原请求编号查询")
		}
	}
	return err
}
