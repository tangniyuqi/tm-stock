package service

import (
	"context"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

// PointsBalance 是同一事务内读取的新账本快照，不包含旧会员资产。
// Available 按有效批次计算，不能用账户汇总值替代；展示不保证后续冻结成功。
type PointsBalance struct {
	Available int64
	Frozen    int64
	CheckedAt int64
}

// Balance 不创建账户、清扫过期批次或写流水。调用方必须提供已鉴权的会员ID。
// HTTP调用通过独立开关及原会员鉴权装配，默认关闭。
func (s *PointsService) Balance(parent context.Context, uid int64) (PointsBalance, error) {
	if s == nil || s.store == nil || parent == nil || uid <= 0 {
		return PointsBalance{}, points.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	var out PointsBalance
	err := s.store.WithinPointsTx(ctx, func(tx points.Tx) error {
		_, _, err := eligiblePointsSnapshot(ctx, tx, uid, &out)
		return err
	})
	if err != nil {
		return PointsBalance{}, err
	}
	return out, nil
}
