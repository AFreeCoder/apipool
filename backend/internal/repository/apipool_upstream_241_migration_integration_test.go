//go:build integration

package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestAPIPoolUpstream241MigrationsPreserveExistingData(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	// 使用当前完整表结构的临时副本，还原本次升级前的字段和平台约束。
	// 临时表优先于 public 同名表解析，迁移不会触及其他测试数据。
	_, err := tx.ExecContext(ctx, `
CREATE TEMP TABLE payment_orders (LIKE public.payment_orders INCLUDING ALL) ON COMMIT DROP;
CREATE TEMP TABLE user_platform_quotas (LIKE public.user_platform_quotas INCLUDING ALL) ON COMMIT DROP;
CREATE TEMP TABLE composite_model_routes (LIKE public.composite_model_routes INCLUDING ALL) ON COMMIT DROP;
ALTER TABLE payment_orders DROP COLUMN bonus_amount;
INSERT INTO payment_orders (user_id, amount, pay_amount, order_type, status, refund_amount, expires_at)
VALUES (1, 100.00, 700.00, 'balance', 'COMPLETED', 0, NOW()),
       (1, 50.00, 350.00, 'subscription', 'PENDING', 0, NOW()),
       (1, 25.00, 175.00, 'balance', 'REFUNDED', 25.00, NOW());`)
	require.NoError(t, err)

	legacySQL, err := migrations.FS.ReadFile("238_opencode_go_platform.sql")
	require.NoError(t, err)
	// 该迁移的 DO 块处理渠道监控；这里只还原前两个表的平台约束。
	_, err = tx.ExecContext(ctx, strings.SplitN(string(legacySQL), "DO $$", 2)[0])
	require.NoError(t, err)

	platforms := []string{"anthropic", "openai", "gemini", "antigravity", "grok", "kimi", "zhipu", "deepseek", "minimax", "opencode_go"}
	for _, platform := range platforms {
		_, err = tx.ExecContext(ctx, `
INSERT INTO user_platform_quotas (user_id, platform, daily_limit_usd, daily_usage_usd)
VALUES (1, $1, 0, 12.34)`, platform)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `
INSERT INTO composite_model_routes (group_id, public_model, target_platform)
VALUES (1, $1, $1)`, platform)
		require.NoError(t, err)
	}

	readSnapshot := func(query string) string {
		t.Helper()
		var snapshot string
		require.NoError(t, tx.QueryRowContext(ctx, query).Scan(&snapshot))
		return snapshot
	}
	orders := readSnapshot("SELECT jsonb_agg(to_jsonb(p) ORDER BY id)::text FROM payment_orders p")
	quotasQuery := "SELECT jsonb_agg(to_jsonb(q) ORDER BY id)::text FROM user_platform_quotas q"
	routesQuery := "SELECT jsonb_agg(to_jsonb(r) ORDER BY id)::text FROM composite_model_routes r"
	quotas := readSnapshot(quotasQuery)
	routes := readSnapshot(routesQuery)

	for run := 0; run < 2; run++ {
		for _, name := range []string{"241_add_payment_order_bonus_amount.sql", "241_add_typesafe_platform.sql"} {
			migrationSQL, readErr := migrations.FS.ReadFile(name)
			require.NoError(t, readErr)
			_, err = tx.ExecContext(ctx, string(migrationSQL))
			require.NoError(t, err, name)
		}
		// 首次升级和重复执行均须保留订单金额、退款、状态、配额用量与路由。
		require.JSONEq(t, orders, readSnapshot("SELECT jsonb_agg(to_jsonb(p) - 'bonus_amount' ORDER BY id)::text FROM payment_orders p"))
		require.JSONEq(t, quotas, readSnapshot(quotasQuery))
		require.JSONEq(t, routes, readSnapshot(routesQuery))
		var nonzeroBonuses int
		require.NoError(t, tx.QueryRowContext(ctx, "SELECT count(*) FROM payment_orders WHERE bonus_amount <> 0 OR bonus_amount IS NULL").Scan(&nonzeroBonuses))
		require.Zero(t, nonzeroBonuses, "历史订单不得被自动添加赠送额度")
	}

	_, err = tx.ExecContext(ctx, `
INSERT INTO user_platform_quotas (user_id, platform) VALUES (1, 'typesafe');
INSERT INTO composite_model_routes (group_id, public_model, target_platform) VALUES (1, 'jev-latest', 'typesafe');`)
	require.NoError(t, err, "新平台约束应接受 TypeSafe 数据")
}
