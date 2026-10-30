// Package main 是 tm-stock C 端 API 服务入口。
//
// 职责：只做装配（读配置 → 初始化依赖 → 注册路由 → 启动），不写业务逻辑。
// 依赖方向：main → handler → service → repository（单向）。
//
// 只用标准库 net/http：Go 1.22+ 的 ServeMux 支持方法 + 路径参数模式，
// 本服务端点很少，没必要为此引入 Web 框架（也省下十几个传递依赖）。
package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql" // 注册驱动；上层各包都不直接依赖它

	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/handler"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("启动失败: %v", err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log.Printf("配置: %s", cfg.Redacted()) // Redacted 刻意不含 DSN
	localRuntime, err := config.LoadAILocalRuntime(cfg.DSN)
	if err != nil {
		return err
	}
	listenAddress := ":" + cfg.Port
	if localRuntime != nil {
		listenAddress = localRuntime.Listen
	}
	readCfg, err := config.LoadAITaskRead()
	if err != nil {
		return err
	}
	cancelEnabled, err := config.LoadAITaskCancel(readCfg)
	if err != nil {
		return err
	}
	balanceEnabled, err := config.LoadPointsBalance(readCfg)
	if err != nil {
		return err
	}
	reportEnabled, err := config.LoadAIReportRead(readCfg)
	if err != nil {
		return err
	}
	createEnabled, err := config.LoadAITaskCreate(readCfg)
	if err != nil {
		return err
	}
	feedbackEnabled, err := config.LoadAIFeedback(readCfg)
	if err != nil {
		return err
	}
	streamEnabled, err := config.LoadAITaskStream(readCfg)
	if err != nil {
		return err
	}
	conversationsEnabled, err := config.LoadAIConversations(readCfg)
	if err != nil {
		return err
	}
	conversationTasksEnabled, err := config.LoadAIConversationTasks(readCfg, conversationsEnabled)
	if err != nil {
		return err
	}
	qaConfig, err := config.LoadAIQA(readCfg, conversationsEnabled)
	if err != nil {
		return err
	}

	db, err := sql.Open("mysql", cfg.DSN)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(time.Hour) // 避免被 MySQL 的 wait_timeout 掐掉连接

	// 启动时就验证连通性。等第一个请求进来才发现连不上，
	// 只会得到一个 500 和一堆猜测。
	pingCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		return err
	}
	if localRuntime != nil {
		if err := repository.NewAILocalInstanceVerifier(db).Verify(pingCtx, localRuntime.InstanceID); err != nil {
			return err
		}
	}
	log.Println("数据库连接正常")

	repo := repository.NewThemeRepo(db)
	dailyRepo := repository.NewDailyRepository(db)

	// 行情：一期 Enabled=false（ADR-0006）。QuoteProvider 传 nil，
	// service 在关闭时不会调用它，涨跌幅一律返回 nil，前端显示「—」。
	themeSvc := service.NewThemeService(repo, nil, service.QuoteConfig{
		Enabled:  cfg.QuoteEnabled,
		DelayMin: cfg.QuoteDelayMin,
		IsMock:   cfg.QuoteMock,
	})

	// 🔴 会员与试吃尚未实现（member-center 未开发）。
	// 默认 fail-closed：不给付费内容。开发放行须显式设 TM_DEV_GRANT_PAID=true。
	var access handler.AccessResolver = handler.DenyAllAccess{}
	if cfg.DevGrantPaid {
		access = handler.NewDevGrantAllAccess()
	}

	mux := http.NewServeMux()
	if err := registerPointsBalance(context.Background(), mux, db, readCfg, balanceEnabled); err != nil {
		return err
	}
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":0,"msg":"ok"}`))
	})
	handler.NewThemeHandler(themeSvc, access).Register(mux)
	handler.NewThemeDailyHandler(service.NewThemeDailyService(dailyRepo, cfg.QuoteDelayMin, cfg.QuoteMock)).Register(mux)
	// 本地只接受显式合成资料计划；没有生产控制器时创建误开仍启动失败。
	deepEnabled, err := config.LoadAIDeepCreate(createEnabled)
	if err != nil {
		return err
	}
	admission, err := prepareAILocalAdmission(db, localRuntime)
	if err != nil {
		return err
	}
	creations, err := prepareAITaskCreatesWithDeep(context.Background(), db, readCfg, createEnabled, deepEnabled, admission)
	if err != nil {
		return err
	}
	if err := registerAITaskCreates(mux, db, readCfg, creations, createEnabled); err != nil {
		return err
	}
	// 正式发布仍无默认放行；本地仅接受同一显式合成计划的已结算报告。
	var reports *service.AIReportReadService
	if localRuntime != nil {
		reports, err = prepareAILocalReportReads(context.Background(), db, readCfg, reportEnabled, admission)
	} else {
		reports, err = prepareAIReportReads(context.Background(), db, readCfg, reportEnabled, nil, nil)
	}
	if err != nil {
		return err
	}
	if err := registerAITaskReadsWithReports(mux, db, readCfg, reports); err != nil {
		return err
	}
	if err := registerAIReportReads(mux, db, readCfg, reports); err != nil {
		return err
	}
	if err := registerAITaskCancels(context.Background(), mux, db, readCfg, cancelEnabled); err != nil {
		return err
	}
	if err := registerAIFeedback(context.Background(), mux, db, readCfg, feedbackEnabled); err != nil {
		return err
	}
	if err := registerAITaskStreams(context.Background(), mux, db, readCfg, streamEnabled); err != nil {
		return err
	}
	if err := registerAIConversationsWithLocal(context.Background(), mux, db, readCfg, conversationsEnabled, localRuntime); err != nil {
		return err
	}
	if err := registerAIConversationTasksWithLocal(context.Background(), mux, db, readCfg, creations, conversationTasksEnabled, createEnabled, localRuntime); err != nil {
		return err
	}
	localQA, err := prepareAILocalQA(context.Background(), db, localRuntime, qaConfig)
	if err != nil {
		return err
	}
	if err := registerAIQAWithLocal(context.Background(), mux, db, readCfg, conversationsEnabled, qaConfig, localQA, reports); err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              listenAddress,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second, // 防慢速头攻击
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// 优雅退出：容器里 SIGTERM 是常态，直接被杀会掐断处理中的请求。
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	errCh := make(chan error, 1)
	go func() {
		log.Printf("tm-stock API 启动，监听 %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case sig := <-stop:
		log.Printf("收到信号 %v，开始优雅退出", sig)
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer shutCancel()
		return srv.Shutdown(shutCtx)
	}
}

// 只读开关不依赖开发付费放行；关闭时没有这些路由，也不装配会员鉴权。
func registerAITaskReads(mux *http.ServeMux, db *sql.DB, cfg config.AITaskReadConfig) error {
	return registerAITaskReadsWithReports(mux, db, cfg, nil)
}

func registerAITaskReadsWithReports(mux *http.ServeMux, db *sql.DB, cfg config.AITaskReadConfig, reports *service.AIReportReadService) error {
	if !cfg.Enabled {
		return nil
	}
	v, err := memberauth.NewVerifier(cfg.Auth)
	if err != nil {
		return err
	}
	sessions, err := service.NewMemberSessionService(v, repository.NewMemberIdentityRepo(db))
	if err != nil {
		return err
	}
	auth, err := middleware.NewMemberAuth(sessions)
	if err != nil {
		return err
	}
	reads, err := service.NewAITaskReadService(repository.NewAITaskRepo(db))
	if reports != nil {
		reads, err = service.NewAITaskReadServiceWithReports(repository.NewAITaskRepo(db), reports)
	}
	if err != nil {
		return err
	}
	handler.NewAITaskReadHandler(reads).Register(mux, auth)
	return nil
}
