package mcdkbridge

// 错误码 —— 忠实照搬 cc-pane mcdk_bridge（protocol.rs / bridge.rs）。
// cc-pane 用 AppError::coded(code, msg)，前端按 code 做 i18n、测试按 code 断言。
// XK 这边这些错误最终由 orchestrator MC PE 工具包成「错误: …」文本返回给 agent，
// 但保留 code（命门）便于测试断言与未来按码识别。
const (
	CodeProtocolIncompleteHeader  = "MCDK_PROTOCOL_INCOMPLETE_HEADER"
	CodeProtocolIncompletePayload = "MCDK_PROTOCOL_INCOMPLETE_PAYLOAD"
	CodeBridgeBindFailed          = "MCDK_BRIDGE_BIND_FAILED"
	CodeBridgeNoClient            = "MCDK_BRIDGE_NO_CLIENT"
	CodeBridgeWriteFailed         = "MCDK_BRIDGE_WRITE_FAILED"
)

// CodedError 携带一个稳定错误码 + 人类可读消息，对齐 cc-pane AppError::coded。
type CodedError struct {
	Code    string
	Message string
}

func (e *CodedError) Error() string { return e.Code + ": " + e.Message }

func coded(code, msg string) *CodedError { return &CodedError{Code: code, Message: msg} }
