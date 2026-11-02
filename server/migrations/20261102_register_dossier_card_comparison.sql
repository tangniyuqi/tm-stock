-- 原核验清单显式增加两卡协议；不修改历史清单、用途默认值或调用审计。
-- 执行前停止核验登记写入，迁移后运行结构和实际正反例核验。
SET NAMES utf8mb4;
ALTER TABLE tm_ai_dossier_support_plans
 DROP CHECK chk_tm_ai_dossier_support_plan,
 ADD CONSTRAINT chk_tm_ai_dossier_support_plan CHECK(theme_id BETWEEN 1 AND 9007199254740991
  AND version_no BETWEEN 1 AND 9007199254740991 AND analysis_type IN(1,2)
  AND component_digest REGEXP '^[a-f0-9]{64}$' AND version_digest REGEXP '^[a-f0-9]{64}$'
  AND manifest_digest REGEXP '^[a-f0-9]{64}$' AND OCTET_LENGTH(canonical) BETWEEN 2 AND 16384
  AND JSON_VALID(canonical) AND JSON_TYPE(CAST(canonical AS JSON))='OBJECT'
  AND JSON_CONTAINS_PATH(canonical,'all','$.protocol','$.analysisType','$.draft','$.draftDigest','$.themeName','$.keys','$.config','$.componentDigest')=1
  AND JSON_TYPE(JSON_EXTRACT(canonical,'$.protocol'))='STRING'
  AND JSON_TYPE(JSON_EXTRACT(canonical,'$.componentDigest'))='STRING'
  AND JSON_TYPE(JSON_EXTRACT(canonical,'$.draftDigest'))='STRING'
  AND JSON_TYPE(JSON_EXTRACT(canonical,'$.analysisType'))='INTEGER'
  AND JSON_UNQUOTE(JSON_EXTRACT(canonical,'$.analysisType'))=CAST(analysis_type AS CHAR)
  AND JSON_TYPE(JSON_EXTRACT(canonical,'$.draft'))='OBJECT'
  AND JSON_TYPE(JSON_EXTRACT(canonical,'$.keys'))='ARRAY'
  AND JSON_LENGTH(JSON_EXTRACT(canonical,'$.keys')) BETWEEN 1 AND 4
  AND JSON_TYPE(JSON_EXTRACT(canonical,'$.config'))='OBJECT'
  AND (JSON_UNQUOTE(JSON_EXTRACT(canonical,'$.protocol'))='dossier-support-manifest-v1'
   OR (JSON_UNQUOTE(JSON_EXTRACT(canonical,'$.protocol'))='dossier-card-comparison-manifest-v1'
    AND analysis_type=2 AND JSON_LENGTH(JSON_EXTRACT(canonical,'$.keys'))=2
    AND JSON_TYPE(JSON_EXTRACT(canonical,'$.keys[0]'))='STRING'
    AND JSON_TYPE(JSON_EXTRACT(canonical,'$.keys[1]'))='STRING'
    AND JSON_UNQUOTE(JSON_EXTRACT(canonical,'$.keys[0]')) REGEXP '^[a-z][a-z0-9_]{0,31}$'
    AND JSON_UNQUOTE(JSON_EXTRACT(canonical,'$.keys[1]')) REGEXP '^[a-z][a-z0-9_]{0,31}$'
    AND JSON_UNQUOTE(JSON_EXTRACT(canonical,'$.keys[0]'))<JSON_UNQUOTE(JSON_EXTRACT(canonical,'$.keys[1]'))
    AND JSON_CONTAINS_PATH(canonical,'all','$.config.model','$.config.focus','$.config.reviewStep')=1
    AND JSON_TYPE(JSON_EXTRACT(canonical,'$.config.model'))='OBJECT'
    AND JSON_TYPE(JSON_EXTRACT(canonical,'$.config.focus'))='STRING'
    AND JSON_UNQUOTE(JSON_EXTRACT(canonical,'$.config.focus')) IN('definition','progress','measurement')
    AND JSON_TYPE(JSON_EXTRACT(canonical,'$.config.reviewStep'))='INTEGER'
    AND JSON_UNQUOTE(JSON_EXTRACT(canonical,'$.config.reviewStep'))='5'))
  AND JSON_UNQUOTE(JSON_EXTRACT(canonical,'$.componentDigest'))=component_digest
  AND JSON_UNQUOTE(JSON_EXTRACT(canonical,'$.draftDigest'))=version_digest
  AND compiled_at>='1970-01-01 00:00:00.001' AND created_at>=compiled_at AND updated_at=created_at);
