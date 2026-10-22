package qa

import "context"

// AccessStore只补问答装配所需的范围加载，执行/审计继续复用原Store。
type AccessStore interface {
	Store
	WithinAccess(context.Context, func(AccessTx) error) error
}
type AccessTx interface {
	Tx
	Scope(context.Context, string, int64) (Scope, string, error)
	ScopeForProof(context.Context, Proof) (Scope, error)
}
