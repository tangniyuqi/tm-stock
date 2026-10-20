package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

// AIReportPublication 仅交给受信任发布控制器，正文与许可归档不进入该接口。
type AIReportPublication struct {
	UserID, TaskID, SubjectID, GeneratedAt          int64
	ReportDigest, PolicyDigest, ModelName, FilingNo string
}

// AIReportReadControl 必须核对真实模型/质量/公示及当前可见授权；不可发起模型或修改账务。
// 已知批准缺失/撤回返回ErrUnavailable，依赖故障返回错误。本仓库没有生产放行实现。
type AIReportReadControl interface {
	Recheck(context.Context, AIReportPublication) error
}
type AIReportReadService struct {
	base    *AIReportService
	control AIReportReadControl
	enabled bool
}

func NewAIReportReadService(base *AIReportService, control AIReportReadControl, enabled bool) (*AIReportReadService, error) {
	if base == nil || base.store == nil || enabled && (!base.policy.Valid() || control == nil) {
		return nil, points.ErrInvalid
	}
	return &AIReportReadService{base, control, enabled}, nil
}

type reportReadView struct {
	publication AIReportPublication
	report      dto.AIReport
}

func (s *AIReportReadService) load(ctx context.Context, uid, id int64) (reportReadView, error) {
	var out reportReadView
	err := s.base.store.WithinReportTx(ctx, func(tx finalreport.Tx) error {
		v, err := lockOwnedTask(ctx, tx, uid, id)
		if err != nil {
			return err
		}
		if v.Status != task.Succeeded {
			return finalreport.ErrUnavailable
		}
		c, err := tx.ReportCommit(ctx, uid, id)
		if errors.Is(err, points.ErrNotFound) {
			return points.ErrInvariant
		}
		if err != nil {
			return err
		}
		r := finalreport.Request{TaskNo: v.No, Stage: task.StageRequest{UserID: uid, TaskID: id, Fence: c.Fence, Owner: "member_report_read", StepNo: c.StepNo, Stage: "judge", OperationKey: c.OperationKey, InputDigest: c.InputDigest}}
		view, err := s.base.readReportView(ctx, tx, r)
		if errors.Is(err, points.ErrNotFound) {
			return points.ErrInvariant
		}
		if errors.Is(err, task.ErrStagePolicy) {
			return finalreport.ErrUnavailable
		}
		if err != nil {
			return err
		}
		if view.old == nil || view.old.Status != task.Succeeded {
			return points.ErrInvariant
		}
		p, err := s.base.policy.Decode(string(view.old.Payload))
		if err != nil {
			return points.ErrInvariant
		}
		if p.TaskNo != v.No || p.ThemeID != v.SubjectID {
			return points.ErrInvariant
		}
		snaps, theme, err := s.base.reportEvidence(ctx, tx, v, s.base.policy.EvidenceIDs(p))
		if err != nil {
			if knownEvidenceRejection(err) {
				return finalreport.ErrUnavailable
			}
			return err
		}
		if s.base.policy.Bind(p, snaps, theme) != nil {
			return finalreport.ErrUnavailable
		}
		d := p.Report.Disclosure
		out.publication = AIReportPublication{uid, id, v.SubjectID, c.CreatedAt.UnixMilli(), c.ContentDigest, c.PolicyDigest, d.ModelName, d.FilingNo}
		out.report = dto.AIReport{TaskID: id, SubjectType: v.SubjectType, SubjectID: v.SubjectID, SubjectName: p.ThemeName, GeneratedAt: c.CreatedAt.UnixMilli(), Disclosure: dto.AIReportDisclosure{AIGenerated: d.AIGenerated, ModelName: d.ModelName, FilingNo: d.FilingNo}, Sections: make([]dto.AIReportSection, 0, len(p.Report.Sections))}
		for _, section := range p.Report.Sections {
			item := dto.AIReportSection{Key: string(section.Key), Title: section.Title, AIGenerated: section.AILabel, Quotes: make([]dto.AIReportQuote, 0, len(section.Nodes))}
			for _, n := range section.Nodes {
				if n.Row != nil {
					row := dto.AIReportCompany{Name: n.Row.Name, Code: n.Row.Code, Node: n.Row.Node, Evidence: make([]dto.AIReportCompanyEvidence, 0, len(n.Row.Evidence))}
					for _, e := range n.Row.Evidence {
						row.Evidence = append(row.Evidence, dto.AIReportCompanyEvidence{Text: e.Excerpt, SourceType: e.Type, SourceURL: e.URL, CollectedAt: e.FetchedAt, Trimmed: e.Trimmed})
					}
					item.Companies = append(item.Companies, row)
					continue
				}
				q := n.Quote
				if q == nil {
					return points.ErrInvariant
				}
				item.Quotes = append(item.Quotes, dto.AIReportQuote{Text: n.Text, SourceType: q.SourceType, SourceURL: q.URL, CollectedAt: q.FetchedAt})
			}
			out.report.Sections = append(out.report.Sections, item)
		}
		return nil
	})
	if err != nil {
		return reportReadView{}, err
	}
	return out, nil
}

// Read 只读已交付报告；所有事务/批准确认后才返回白名单正文，不重跑模型或结算。
func (s *AIReportReadService) Read(parent context.Context, uid, id int64) (dto.AIReport, error) {
	if s == nil || uid <= 0 || id <= 0 {
		return dto.AIReport{}, points.ErrInvalid
	}
	if !s.enabled {
		return dto.AIReport{}, finalreport.ErrDisabled
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	before, err := s.load(ctx, uid, id)
	if err != nil {
		return dto.AIReport{}, fmt.Errorf("初次复核会员报告失败: %w", err)
	}
	// 批准校验在事务外，不能持账户/来源锁等待其他服务；第二次读取重新核对当前授权。
	if err := s.control.Recheck(ctx, before.publication); err != nil {
		return dto.AIReport{}, err
	}
	after, err := s.load(ctx, uid, id)
	if err != nil {
		return dto.AIReport{}, fmt.Errorf("最终复核会员报告失败: %w", err)
	}
	if before.publication != after.publication {
		return dto.AIReport{}, points.ErrInvariant
	}
	if err := s.control.Recheck(ctx, after.publication); err != nil {
		return dto.AIReport{}, err
	}
	if err := ctx.Err(); err != nil {
		return dto.AIReport{}, err
	}
	return after.report, nil
}
func (s *AIReportReadService) Available(ctx context.Context, uid, id int64) (bool, error) {
	_, err := s.Read(ctx, uid, id)
	if errors.Is(err, finalreport.ErrDisabled) || errors.Is(err, finalreport.ErrUnavailable) || errors.Is(err, points.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
