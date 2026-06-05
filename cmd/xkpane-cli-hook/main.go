// Command xkpane-cli-hook 是被在 pane 内运行的 Claude/Codex 在生命周期事件 spawn 的
// 一次性子进程（非常驻），对齐 cc-pane 的 cc-panes-cli-hook crate。
//
// 子命令（P6d-1，im-forward 已按 D6d-3 砍掉）：
//   - session-start  SessionStart：stdout 注入项目上下文 + 回连 orchestrator 上报 cwd/resumeId
//   - ask-pending    PreToolUse(AskUserQuestion)：把「待答问题」回连 orchestrator，供卡片在落盘前即时渲染作答
//   - plan-archive   PostToolUse(Write)：把写入 ~/.claude/plans/ 的 plan 归档到项目本地（纯本地）
//   - notify         显式触发桌面通知（读 mcp-orchestrator.json → POST /api/notifications/trigger）
//
// 铁律（对齐 cc-pane）：750ms 超时、失败【静默 exit 0】、绝不阻塞 CLI（notify 例外，沿用 exit 1）。
//
// 刻意【纯 stdlib、不 import internal】：hook 是独立进程，要求启动极快、零额外依赖；git 子进程用
// 平台特定的 no-window 包装（避免 GUI/PTY 下窗口闪烁），HTTP 用显式 750ms 短超时 client（不复用
// 主进程的 httpx 单例长连客户端——一次性进程用不上连接池，硬超时才是这里的需求）。
package main

import "os"

func main() {
	if len(os.Args) < 2 {
		os.Exit(0) // 无子命令：静默退出，绝不阻塞调用方
	}
	switch os.Args[1] {
	case "session-start":
		runSessionStart()
	case "ask-pending":
		runAskPending()
	case "plan-archive":
		runPlanArchive()
	case "notify":
		runNotify(os.Args[2:])
	default:
		// 未知子命令：静默 exit 0（向前兼容，绝不让 CLI 因 hook 报错而卡住）
		os.Exit(0)
	}
}
