//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestAPIPoolUpstream242MigrationsPreserveExistingData(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	// 临时表还原升级前的分组与平台约束，避免接触其他测试或真实数据。
	_, err := tx.ExecContext(ctx, `
CREATE TEMP TABLE groups (LIKE public.groups INCLUDING ALL) ON COMMIT DROP;
CREATE TEMP TABLE user_platform_quotas (LIKE public.user_platform_quotas INCLUDING ALL) ON COMMIT DROP;
CREATE TEMP TABLE composite_model_routes (LIKE public.composite_model_routes INCLUDING ALL) ON COMMIT DROP;
ALTER TABLE groups DROP COLUMN i18n;
INSERT INTO groups (id, name, description, platform, rate_multiplier, subscription_type, daily_limit_usd)
VALUES (1, 'APIPool GPT 订阅', '保留自定义订阅说明', 'openai', 0.15, 'subscription', 50),
       (2, 'APIPool 免费分组', '零倍率不得改成默认倍率', 'anthropic', 0, 'standard', NULL);
INSERT INTO user_platform_quotas (user_id, platform, daily_limit_usd, daily_usage_usd)
VALUES (1, 'openai', 0, 12.34), (1, 'typesafe', NULL, 5.67);
INSERT INTO composite_model_routes (group_id, public_model, target_platform, upstream_model)
VALUES (1, 'custom-model', 'openai', 'gpt-6.1-sol');`)
	require.NoError(t, err)
	legacySQL, err := migrations.FS.ReadFile("241_add_typesafe_platform.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(legacySQL))
	require.NoError(t, err)

	readSnapshot := func(query string) string {
		t.Helper()
		var snapshot string
		require.NoError(t, tx.QueryRowContext(ctx, query).Scan(&snapshot))
		return snapshot
	}
	groups := readSnapshot("SELECT jsonb_agg(to_jsonb(g) ORDER BY id)::text FROM groups g")
	quotasQuery := "SELECT jsonb_agg(to_jsonb(q) ORDER BY id)::text FROM user_platform_quotas q"
	routesQuery := "SELECT jsonb_agg(to_jsonb(r) ORDER BY id)::text FROM composite_model_routes r"
	quotas := readSnapshot(quotasQuery)
	routes := readSnapshot(routesQuery)

	for range 2 {
		for _, name := range []string{"242_drop_platform_check_constraints.sql", "242_group_i18n.sql"} {
			migrationSQL, readErr := migrations.FS.ReadFile(name)
			require.NoError(t, readErr)
			_, err = tx.ExecContext(ctx, string(migrationSQL))
			require.NoError(t, err, name)
		}
		require.JSONEq(t, groups, readSnapshot("SELECT jsonb_agg(to_jsonb(g) - 'i18n' ORDER BY id)::text FROM groups g"))
		require.JSONEq(t, quotas, readSnapshot(quotasQuery))
		require.JSONEq(t, routes, readSnapshot(routesQuery))
		var translatedGroups int
		require.NoError(t, tx.QueryRowContext(ctx, "SELECT count(*) FROM groups WHERE i18n <> '{}'::jsonb OR i18n IS NULL").Scan(&translatedGroups))
		require.Zero(t, translatedGroups, "历史分组应使用原名称和说明，不自动填充译文")
	}
}
