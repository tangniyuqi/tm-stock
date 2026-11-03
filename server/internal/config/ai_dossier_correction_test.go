package config

import "testing"

func TestDossierCorrectionFlags(t *testing.T) {
	for _, tc := range []struct {
		name, value           string
		parent, want, invalid bool
	}{{"default", "", false, false, false}, {"false", "false", true, false, false}, {"enabled", "true", true, true, false}, {"missing_parent", "true", false, false, true}, {"bad_flag", "unknown", true, false, true}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TM_AI_LOCAL_DOSSIER_CORRECTIONS_ENABLED", tc.value)
			v, err := LoadAILocalDossierCorrections(tc.parent)
			if (err != nil) != tc.invalid || v != tc.want {
				t.Fatal("更正读取开关边界错误")
			}
		})
	}
	for _, value := range []string{"true", "unknown"} {
		t.Run("admin_"+value, func(t *testing.T) {
			t.Setenv("TM_AI_DOSSIER_MANAGEMENT_ADMIN_ENABLED", "false")
			t.Setenv("TM_AI_DOSSIER_MANAGEMENT_ACTORS", "")
			t.Setenv("TM_AI_DOSSIER_MANAGEMENT_PREPARE_DSN", "")
			t.Setenv("TM_AI_DOSSIER_CORRECTIONS_ADMIN_ENABLED", value)
			if _, err := LoadAIDossierManagement(nil, ""); err == nil {
				t.Fatal("更正管理绕过原管理开关")
			}
		})
	}
}
