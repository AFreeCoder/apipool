package handler

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"go.uber.org/zap"
)

// logRequestBodyParseFailure 记录请求体 JSON 解析或校验失败的真实原因。
// 客户端仍收到通用错误；诊断日志只保留底层错误、长度和稳定哈希，
// 不把提示词、凭据、个人信息或畸形请求片段写入应用日志。
//
// 对直接使用 gjson.ValidBytes 校验的调用方，err 可以为空；此时会从请求体推导诊断错误。
func logRequestBodyParseFailure(reqLog *zap.Logger, body []byte, err error) {
	if reqLog == nil {
		return
	}
	if err == nil {
		err = service.DescribeInvalidJSON(body)
	}

	digest := sha256.Sum256(body)
	fields := []zap.Field{
		zap.Error(err),
		zap.Int("body_len", len(body)),
		zap.String("body_sha256", hex.EncodeToString(digest[:8])),
	}
	reqLog.Warn("parse request body failed", fields...)
}
