package bedrockdocsync

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// gitea.go：基于 Gitea Tree API + 扩展名白名单的选择性同步（对齐 gitea_sync.rs）。
// 为什么直连 Gitea API 而非 git2：netease-bedrock-wiki 镜像 6.7GB，浅克隆仍传全量 blob；
// 我们只要 mcguide/mconline 的 ~7MB markdown。流程：① /repos/{o}/{r} 取默认分支 →
// ② /git/trees/{ref}?recursive=true&per_page=1000 分页列树 → ③ 客户端按子目录+扩展过滤 →
// ④ /{o}/{r}/raw/branch/{b}/{path} 并发拉 blob。无 git2（不引 go-git）。

// pageSize 是 Gitea git/trees 端点 per_page 服务端硬上限。
const pageSize = 1000

// GiteaSyncParams 一次同步入参。Subdirs 为路径前缀白名单（每项当 "<entry>/"），空 ⇒ 全留。
type GiteaSyncParams struct {
	SourceName   string // 进度事件里的稳定标识（如 "wiki"）
	APIBase      string // 如 "https://git.mcplugin.top/api/v1"，尾斜杠可有
	RawBase      string // 如 "https://git.mcplugin.top"，尾斜杠可有
	Owner        string
	Repo         string
	Branch       string   // "" ⇒ 经 /repos/{o}/{r}.default_branch 解析
	Subdirs      []string // 路径前缀白名单，空 ⇒ 全部
	ExtWhitelist []string // 扩展名白名单（小写无点），空 ⇒ 全部
	TargetDir    string   // 文件落到 TargetDir/{entry.path}（镜像结构）
	Concurrency  int      // 并发 raw blob 拉取
}

// FetchFailure 是一条 blob 下载失败记录。
type FetchFailure struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// GiteaSyncReport 一次同步报告。
type GiteaSyncReport struct {
	SourceName            string         `json:"sourceName"`
	Branch                string         `json:"branch"`
	TreeSHA               string         `json:"treeSha"` // 持久化进 SourceInfo.TreeSHA
	Fetched               uint32         `json:"fetched"`
	SkippedNotWhitelisted uint32         `json:"skippedNotWhitelisted"`
	Failed                []FetchFailure `json:"failed"`
}

type giteaRepoMeta struct {
	DefaultBranch string `json:"default_branch"`
}

type giteaTreeResp struct {
	SHA  string           `json:"sha"`
	Tree []giteaTreeEntry `json:"tree"`
}

type giteaTreeEntry struct {
	Path string  `json:"path"`
	Kind string  `json:"type"`
	SHA  string  `json:"sha"`
	Size *uint64 `json:"size"`
}

// giteaSync 执行一次 Gitea 选择性同步。client 由调用方按 ProxyConfig 构造（便于测试注入 mock）。
func giteaSync(ctx context.Context, client *http.Client, params GiteaSyncParams, progress ProgressSink) (GiteaSyncReport, error) {
	if err := os.MkdirAll(params.TargetDir, 0o755); err != nil {
		return GiteaSyncReport{}, fmt.Errorf("mkdir target: %w", err)
	}
	apiBase := strings.TrimRight(params.APIBase, "/")
	rawBase := strings.TrimRight(params.RawBase, "/")

	progress.GiteaSync(params.SourceName, "resolving_branch")

	// Step 1：分支解析。
	branch := strings.TrimSpace(params.Branch)
	if branch == "" {
		url := fmt.Sprintf("%s/repos/%s/%s", apiBase, params.Owner, params.Repo)
		progress.Info(fmt.Sprintf("%s: resolving default branch from %s", params.SourceName, url))
		var meta giteaRepoMeta
		if err := giteaGetJSON(ctx, client, url, &meta); err != nil {
			return GiteaSyncReport{}, fmt.Errorf("repo metadata: %w", err)
		}
		branch = meta.DefaultBranch
	}

	progress.GiteaSync(params.SourceName, "listing_tree")

	// Step 2：分页列递归树，页返回 < per_page 即末页（truncated 不可靠）。
	apiRoot := fmt.Sprintf("%s/repos/%s/%s/git/trees/%s?recursive=true&per_page=%d",
		apiBase, params.Owner, params.Repo, branch, pageSize)
	progress.Info(fmt.Sprintf("%s: listing tree %s/%s@%s (paginated)", params.SourceName, params.Owner, params.Repo, branch))
	var allEntries []giteaTreeEntry
	treeSHA := ""
	page := 1
	for {
		var resp giteaTreeResp
		if err := giteaGetJSON(ctx, client, fmt.Sprintf("%s&page=%d", apiRoot, page), &resp); err != nil {
			return GiteaSyncReport{}, fmt.Errorf("tree: %w", err)
		}
		if treeSHA == "" {
			treeSHA = resp.SHA
		}
		received := len(resp.Tree)
		allEntries = append(allEntries, resp.Tree...)
		if received < pageSize {
			break
		}
		page++
		if page > 100 {
			progress.Warn(fmt.Sprintf("%s: tree pagination exceeded 100 pages, stopping", params.SourceName))
			break
		}
	}
	progress.Info(fmt.Sprintf("%s: tree paginated to %d entries across %d page(s)", params.SourceName, len(allEntries), page))

	// Step 3：客户端过滤。
	extSet := make(map[string]bool, len(params.ExtWhitelist))
	for _, e := range params.ExtWhitelist {
		extSet[strings.ToLower(e)] = true
	}
	targets, skipped := filterBlobs(allEntries, params.Subdirs, extSet)
	total := uint32(len(targets))
	var totalBytes uint64
	for _, e := range targets {
		if e.Size != nil {
			totalBytes += *e.Size
		}
	}
	progress.Info(fmt.Sprintf("%s: %d blobs match (%.1f MB; skipped %d non-whitelisted)",
		params.SourceName, total, float64(totalBytes)/1024.0/1024.0, skipped))

	progress.GiteaSync(params.SourceName, "fetching_blobs")

	// Step 4：并发 raw 下载（worker pool + 信号量，部分失败不致命）。
	concurrency := params.Concurrency
	if concurrency < 1 {
		concurrency = 1
	}
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var fetched uint32
	var failed []FetchFailure

	recordFail := func(path, msg string) {
		mu.Lock()
		failed = append(failed, FetchFailure{Path: path, Error: msg})
		mu.Unlock()
	}

	for _, entry := range targets {
		if ctx.Err() != nil {
			break // 取消：停止派发新任务
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(e giteaTreeEntry) {
			defer wg.Done()
			defer func() { <-sem }()
			url := fmt.Sprintf("%s/%s/%s/raw/branch/%s/%s", rawBase, params.Owner, params.Repo, branch, e.Path)
			b, err := downloadBlob(ctx, client, url)
			if err != nil {
				recordFail(e.Path, err.Error())
				return
			}
			dest := filepath.Join(params.TargetDir, filepath.FromSlash(e.Path))
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				recordFail(e.Path, fmt.Sprintf("mkdir parent: %v", err))
				return
			}
			if err := os.WriteFile(dest, b, 0o644); err != nil {
				recordFail(e.Path, fmt.Sprintf("write: %v", err))
				return
			}
			mu.Lock()
			fetched++
			f := fetched
			mu.Unlock()
			if f%50 == 0 {
				progress.HTTPFetch(params.SourceName, f, total)
			}
		}(entry)
	}
	wg.Wait()

	progress.HTTPFetch(params.SourceName, fetched, total)
	progress.GiteaSync(params.SourceName, "done")

	return GiteaSyncReport{
		SourceName:            params.SourceName,
		Branch:                branch,
		TreeSHA:               treeSHA,
		Fetched:               fetched,
		SkippedNotWhitelisted: skipped,
		Failed:                failed,
	}, nil
}

func giteaGetJSON(ctx context.Context, client *http.Client, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", probeUserAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("status %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func downloadBlob(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}
	req.Header.Set("User-Agent", probeUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("status: %s", resp.Status)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("body: %w", err)
	}
	return b, nil
}

// filterBlobs 纯过滤：保留 kind=="blob" 且 path 以任一 "subdir/" 开头且扩展名在白名单内的 entry。
// 返回 (kept, skipped)；skipped 只数「子目录命中但扩展名 miss」（子目录 miss 静默丢弃不计）。
// 对齐 gitea_sync.rs:336-373。
func filterBlobs(entries []giteaTreeEntry, subdirs []string, extSet map[string]bool) ([]giteaTreeEntry, uint32) {
	prefixes := make([]string, len(subdirs))
	for i, s := range subdirs {
		prefixes[i] = s + "/"
	}
	var kept []giteaTreeEntry
	var skipped uint32
	for _, e := range entries {
		if e.Kind != "blob" {
			continue
		}
		inSubdir := len(prefixes) == 0
		for _, p := range prefixes {
			if strings.HasPrefix(e.Path, p) {
				inSubdir = true
				break
			}
		}
		if !inSubdir {
			continue
		}
		extOK := len(extSet) == 0
		if !extOK {
			if dot := strings.LastIndexByte(e.Path, '.'); dot >= 0 {
				stem := e.Path[:dot]
				if stem != "" && !strings.HasSuffix(stem, "/") {
					extOK = extSet[strings.ToLower(e.Path[dot+1:])]
				}
			}
		}
		if !extOK {
			skipped++
			continue
		}
		kept = append(kept, e)
	}
	return kept, skipped
}
