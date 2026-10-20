package service

import (
	"context"
	"fmt"

	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

// ReconcileAll 逐用户复用真实账本对账，不修余额、不创建缺失账户。
// 上层须提供整轮期限；每用户仍受 Reconcile 的 30 秒事务期限保护。
// 发现异常后继续检查其他账户；访问或告警输出失败则中止，Complete 不得置真。
func (s *PointsService) ReconcileAll(ctx context.Context, scan points.AuditScanner, pageSize int, notify func(points.AuditReport) error) (points.AuditSweep, error) {
	var out points.AuditSweep
	if s == nil || s.store == nil || scan == nil || pageSize < 1 || pageSize > 1000 || notify == nil {
		return out, points.ErrInvalid
	}
	if _, ok := ctx.Deadline(); !ok {
		return out, points.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	upper, err := scan.AuditUpperBound(ctx)
	if err != nil {
		return out, fmt.Errorf("读取本轮对账范围失败: %w", err)
	}
	if upper < 0 {
		return out, points.ErrInvariant
	}
	out.UpperUserID = upper
	for out.LastUserID < upper {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		users, err := scan.ListAuditUsers(ctx, out.LastUserID, upper, pageSize)
		if err != nil {
			return out, fmt.Errorf("读取对账分页失败: %w", err)
		}
		if len(users) > pageSize {
			return out, points.ErrInvariant
		}
		// 先检查整页，避免坏游标导致重复审计、漏报或无限循环。
		previous := out.LastUserID
		for _, uid := range users {
			if uid <= previous || uid > upper {
				return out, points.ErrInvariant
			}
			previous = uid
		}
		if len(users) == 0 {
			// 上界来自真实账户，账本禁止删除账户；尚未读到上界就返回空页不能算完整。
			return out, points.ErrInvariant
		}
		for _, uid := range users {
			report, err := s.Reconcile(ctx, uid)
			if err != nil {
				return out, fmt.Errorf("单用户账本对账失败 uid=%d: %w", uid, err)
			}
			out.Checked++
			if !report.Healthy() {
				out.Unhealthy++
				out.Issues += int64(len(report.Issues))
				if err := notify(report); err != nil {
					return out, fmt.Errorf("输出账本对账告警失败: %w", err)
				}
			}
			// 告警被接收后才推进游标；失败不得假称该用户已完成。
			out.LastUserID = uid
		}
	}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	out.Complete = true
	return out, nil
}
