-- 归属依据的节选标记。先执行 20261002 共表收敛，再执行本增量迁移，最后部署读写端。
-- 可重复执行；不改写原文、审核状态或既有映射。历史摘录默认 0，不表示完整转载。
-- 未知列定义/同名约束主动中止，不静默修复；mysql 执行时禁止使用 --force。
SET NAMES utf8mb4;
SET @tm_excerpt_db = DATABASE();
SET @tm_excerpt_base = (SELECT COUNT(*) FROM information_schema.columns
  WHERE table_schema = @tm_excerpt_db AND table_name = 'addon_quant_theme_stock'
    AND column_name IN ('source_type','source_excerpt','source_url','collected_at','audit_status'));
SET @tm_excerpt_sql = IF(@tm_excerpt_base = 5, 'DO 0',
  'SELECT * FROM tm_migration_abort__excerpt_requires_converged_mapping');
PREPARE tm_excerpt_stmt FROM @tm_excerpt_sql; EXECUTE tm_excerpt_stmt; DEALLOCATE PREPARE tm_excerpt_stmt;

SET @tm_excerpt_exists = (SELECT COUNT(*) FROM information_schema.columns
  WHERE table_schema = @tm_excerpt_db AND table_name = 'addon_quant_theme_stock' AND column_name = 'excerpt_trimmed');
SET @tm_excerpt_sql = IF(@tm_excerpt_exists = 0,
  'ALTER TABLE addon_quant_theme_stock ADD COLUMN excerpt_trimmed tinyint NOT NULL DEFAULT 0 COMMENT ''原文摘录是否节选：0否 1是；不代表原文转载放行'' AFTER source_excerpt', 'DO 0');
PREPARE tm_excerpt_stmt FROM @tm_excerpt_sql; EXECUTE tm_excerpt_stmt; DEALLOCATE PREPARE tm_excerpt_stmt;

SET @tm_excerpt_valid = (SELECT COUNT(*) FROM information_schema.columns
  WHERE table_schema = @tm_excerpt_db AND table_name = 'addon_quant_theme_stock' AND column_name = 'excerpt_trimmed'
    AND data_type = 'tinyint' AND is_nullable = 'NO' AND column_default = '0' AND extra = '');
SET @tm_excerpt_sql = IF(@tm_excerpt_valid = 1, 'DO 0',
  'SELECT * FROM tm_migration_abort__excerpt_unknown_column_definition');
PREPARE tm_excerpt_stmt FROM @tm_excerpt_sql; EXECUTE tm_excerpt_stmt; DEALLOCATE PREPARE tm_excerpt_stmt;

SET @tm_excerpt_check = (SELECT COUNT(*) FROM information_schema.table_constraints
  WHERE constraint_schema = @tm_excerpt_db AND table_name = 'addon_quant_theme_stock'
    AND constraint_name = 'chk_theme_stock_excerpt_trimmed');
SET @tm_excerpt_sql = IF(@tm_excerpt_check = 0,
  'ALTER TABLE addon_quant_theme_stock ADD CONSTRAINT chk_theme_stock_excerpt_trimmed CHECK (excerpt_trimmed IN (0,1))', 'DO 0');
PREPARE tm_excerpt_stmt FROM @tm_excerpt_sql; EXECUTE tm_excerpt_stmt; DEALLOCATE PREPARE tm_excerpt_stmt;

-- 检查同名约束确实强制执行且没有换成宽松表达式。
SET @tm_excerpt_valid = (SELECT COUNT(*) FROM information_schema.table_constraints tc
  JOIN information_schema.check_constraints cc ON cc.constraint_schema = tc.constraint_schema AND cc.constraint_name = tc.constraint_name
  WHERE tc.constraint_schema = @tm_excerpt_db AND tc.table_name = 'addon_quant_theme_stock'
    AND tc.constraint_name = 'chk_theme_stock_excerpt_trimmed' AND tc.enforced = 'YES'
    AND LOWER(REPLACE(REPLACE(REPLACE(REPLACE(cc.check_clause,'`',''),' ',''),'(',''),')','')) = 'excerpt_trimmedin0,1');
SET @tm_excerpt_sql = IF(@tm_excerpt_valid = 1, 'DO 0',
  'SELECT * FROM tm_migration_abort__excerpt_unknown_check_definition');
PREPARE tm_excerpt_stmt FROM @tm_excerpt_sql; EXECUTE tm_excerpt_stmt; DEALLOCATE PREPARE tm_excerpt_stmt;
