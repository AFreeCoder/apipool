//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestAPIPoolUpstream245MigrationsPreserveCustomizations(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()
	// 独立 schema 随事务回滚，使用现有表结构和合成业务数据演练升级。
	_, err := tx.ExecContext(ctx, `
CREATE SCHEMA apipool_upstream_245_test;
SET LOCAL search_path TO apipool_upstream_245_test;
CREATE TABLE settings (LIKE public.settings INCLUDING ALL);
CREATE TABLE groups (LIKE public.groups INCLUDING ALL);
CREATE TABLE payment_orders (LIKE public.payment_orders INCLUDING ALL);
CREATE TABLE user_subscriptions (LIKE public.user_subscriptions INCLUDING ALL);
CREATE TABLE subscription_plans (LIKE public.subscription_plans INCLUDING ALL);
INSERT INTO settings (key, value) VALUES
 ('site_name', 'APIPool'),
 ('custom_menu_items', '[{"name":"充值","url":"https://store.example.test/recharge"}]'),
 ('purchase_subscription_enabled', 'true'),
 ('purchase_subscription_url', 'https://store.example.test/subscription'),
 ('RECHARGE_BONUS_MODE', 'bonus'),
 ('RECHARGE_BONUS_TIERS', '[{"min_amount":100,"bonus_percent":20}]');
INSERT INTO groups (id, name, platform, rate_multiplier, subscription_type, daily_limit_usd)
VALUES (1, 'APIPool GPT 订阅', 'openai', 0.15, 'subscription', 50),
       (2, 'APIPool 免费分组', 'anthropic', 0, 'standard', NULL);
INSERT INTO payment_orders (user_id, amount, bonus_amount, pay_amount, order_type, status, refund_amount, expires_at)
VALUES (1, 100, 20, 700, 'balance', 'COMPLETED', 0, NOW()),
       (1, 50, 0, 350, 'subscription', 'PENDING', 0, NOW()),
       (1, 25, 5, 175, 'balance', 'REFUND_PENDING', 25, NOW());
INSERT INTO user_subscriptions (user_id, group_id, starts_at, expires_at, daily_usage_usd)
VALUES (1, 1, NOW(), NOW() + INTERVAL '30 days', 12.34);
INSERT INTO subscription_plans (group_id, name, price, original_price, validity_days)
VALUES (1, 'APIPool 月套餐', 99, 129, 30);`)
	require.NoError(t, err)

	snapshot := func(table string) string {
		t.Helper()
		var result string
		require.NoError(t, tx.QueryRowContext(ctx, "SELECT jsonb_agg(to_jsonb(t) ORDER BY id)::text FROM "+table+" t").Scan(&result))
		return result
	}
	before := map[string]string{}
	for _, table := range []string{"settings", "groups", "payment_orders", "user_subscriptions", "subscription_plans"} {
		before[table] = snapshot(table)
	}
	for range 2 {
		for _, name := range []string{"243_gemini_cached_contents.sql", "244_gemini_cached_contents_channel_usage.sql", "245_gemini_cached_contents_mutation_lease.sql"} {
			sql, readErr := migrations.FS.ReadFile(name)
			require.NoError(t, readErr)
			_, err = tx.ExecContext(ctx, string(sql))
			require.NoError(t, err, name)
		}
		for table, expected := range before {
			require.JSONEq(t, expected, snapshot(table), table)
		}
	}
	var count int
	require.NoError(t, tx.QueryRowContext(ctx, "SELECT count(*) FROM gemini_cached_contents").Scan(&count))
	require.Zero(t, count, "升级不得自动创建缓存或产生缓存费用")
}
