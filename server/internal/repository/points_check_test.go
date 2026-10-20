package repository

import (
	"context"
	"errors"
	"testing"

	mysqldrv "github.com/go-sql-driver/mysql"
)

func TestLedgerVersionMinimumAndVendor(t *testing.T) {
	for _, tc := range []struct {
		version, comment string
		want             bool
	}{
		{"8.0.15", "MySQL Community Server", false}, {"8.0.16", "MySQL Community Server", true},
		{"8.0.46", "MySQL Community Server - GPL", true}, {"8.4.0-commercial", "MySQL Enterprise Server", true},
		{"9.7.0", "MySQL Community Server - GPL", true}, {"5.7.44", "MySQL", false},
		{"10.11.0-MariaDB", "MySQL compatible MariaDB", false}, {"9.0.0", "TiDB", false},
		{"garbage", "MySQL", false}, {"8.0", "MySQL", false}, {"8.0.16evil", "MySQL", false},
		{"99999999999999999999999.0.0", "MySQL", false},
	} {
		if got := validLedgerMySQLVersion(tc.version, tc.comment); got != tc.want {
			t.Errorf("%q/%q = %v，期望 %v", tc.version, tc.comment, got, tc.want)
		}
	}
}

func TestLedgerVerifierErrorsFailClosed(t *testing.T) {
	var nilVerifier *PointsSchemaVerifier
	if err := nilVerifier.Verify(context.Background()); err == nil {
		t.Fatal("空校验器未拒绝")
	}
	if err := NewPointsSchemaVerifier(nil).Verify(context.Background()); err == nil {
		t.Fatal("空数据库未拒绝")
	}
	for _, err := range []error{nil, context.Canceled, errors.New("permission denied"), &mysqldrv.MySQLError{Number: 1452}} {
		if pointsProbeError(err, 3819) {
			t.Fatal("非 CHECK 错误被当作 CHECK 生效")
		}
	}
	if !pointsProbeError(&mysqldrv.MySQLError{Number: 3819}, 3819) {
		t.Fatal("CHECK 错误未识别")
	}
	if errOrMissing(nil) == nil || !errors.Is(errOrMissing(context.Canceled), context.Canceled) {
		t.Fatal("意外成功/错误没有正确传播")
	}
}
