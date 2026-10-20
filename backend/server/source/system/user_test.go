package system

import (
	"context"
	"errors"
	"testing"

	initService "github.com/flipped-aurora/gin-vue-admin/server/service/system"
)

func TestAdminPasswordFromContext(t *testing.T) {
	tests := []struct {
		name     string
		setValue bool
		value    any
		want     string
		wantErr  bool
	}{
		{
			name:    "缺失管理员密码",
			wantErr: true,
		},
		{
			name:     "管理员密码类型错误",
			setValue: true,
			value:    42,
			wantErr:  true,
		},
		{
			name:     "管理员密码为空字符串",
			setValue: true,
			value:    "",
			wantErr:  true,
		},
		{
			name:     "管理员密码全为空白",
			setValue: true,
			value:    " \t\n ",
			wantErr:  true,
		},
		{
			name:     "保留有效密码的前后空白",
			setValue: true,
			value:    "  A1b2c3  ",
			want:     "  A1b2c3  ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.setValue {
				ctx = context.WithValue(ctx, "adminPassword", tt.value)
			}

			got, err := adminPasswordFromContext(ctx)
			if tt.wantErr {
				if !errors.Is(err, initService.ErrInvalidAdminPassword) {
					t.Fatalf("adminPasswordFromContext() error = %v, want ErrInvalidAdminPassword", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("adminPasswordFromContext() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("adminPasswordFromContext() = %q, want %q", got, tt.want)
			}
		})
	}
}
