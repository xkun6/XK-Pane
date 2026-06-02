package bedrockdocsync

import "xkpane/internal/events"

// progress.go：OTA 同步进度通道（对齐 progress.rs）。stage 判别 + snake_case stage 值，前端易分发。
// 进度事件 wire 类型复用 events 包的 payload（单一来源供 Wails binding 生成）。

type progressEvent = events.BedrockDocSyncProgressPayload

// ProgressSink 是进度发送端。emit 为 nil → 静默丢弃（fire-and-forget，UI 关闭/无订阅不报错、不 panic）。
type ProgressSink struct {
	emit func(progressEvent)
}

// NewProgressSink 包一个发射回调（app 装配处传 app.Event.Emit 双路）；传 nil 得 noop sink。
func NewProgressSink(emit func(events.BedrockDocSyncProgressPayload)) ProgressSink {
	return ProgressSink{emit: emit}
}

func (s ProgressSink) emitEvent(e progressEvent) {
	if s.emit != nil {
		s.emit(e)
	}
}

// stage 值对齐 cc-pane ProgressEvent 的 snake_case 变体名。

func (s ProgressSink) Started(sources []string) {
	s.emitEvent(progressEvent{Stage: "started", Sources: sources})
}

func (s ProgressSink) GiteaSync(source, status string) {
	s.emitEvent(progressEvent{Stage: "gitea_sync", Source: source, Status: status})
}

func (s ProgressSink) HTTPFetch(source string, fetched, total uint32) {
	s.emitEvent(progressEvent{Stage: "http_fetch", Source: source, Fetched: fetched, Total: total})
}

func (s ProgressSink) BuildIndex(kind string, entries uint32) {
	s.emitEvent(progressEvent{Stage: "build_index", Kind: kind, Entries: entries})
}

func (s ProgressSink) Validating() {
	s.emitEvent(progressEvent{Stage: "validating"})
}

func (s ProgressSink) Completed(manifestPath string, durationMs uint64) {
	s.emitEvent(progressEvent{Stage: "completed", ManifestPath: manifestPath, DurationMs: durationMs})
}

func (s ProgressSink) Cancelled() {
	s.emitEvent(progressEvent{Stage: "cancelled"})
}

func (s ProgressSink) Failed(errMsg string, partial bool) {
	s.emitEvent(progressEvent{Stage: "failed", Error: errMsg, Partial: partial})
}

func (s ProgressSink) Info(msg string) {
	s.emitEvent(progressEvent{Stage: "log", Level: "info", Msg: msg})
}
func (s ProgressSink) Warn(msg string) {
	s.emitEvent(progressEvent{Stage: "log", Level: "warn", Msg: msg})
}
func (s ProgressSink) ErrorLog(msg string) {
	s.emitEvent(progressEvent{Stage: "log", Level: "error", Msg: msg})
}
