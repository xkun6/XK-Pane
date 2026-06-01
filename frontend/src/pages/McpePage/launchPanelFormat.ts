// 运行面板日志的纯函数工具（无框架耦合，1:1 移植 cc-pane MCPELaunchPanel.tsx）。
// shortTs 时间戳缩写 / tokenize 行内 file:line 切分 / resolvePath 相对→绝对。

export interface LinkRef {
  path: string;
  line: number;
  column?: number;
}

export interface Segment {
  text: string;
  link?: LinkRef;
}

// `2026-05-16 16:09:57,624` → `16:09:57`；不识别原样返回（纯字符串切割）。
export function shortTs(ts: string): string {
  const sp = ts.indexOf(' ');
  if (sp < 0) return ts;
  const tail = ts.slice(sp + 1);
  const comma = tail.indexOf(',');
  return comma < 0 ? tail : tail.slice(0, comma);
}

const PY_RE = /File "([\w./\\-]+)", line (\d+)/g;
const JSON_RE = /([A-Za-z0-9_\-./\\]+\.json)(?::\*?)?(?:\s+Line\s+(\d+)(?:,\s*Column\s+(\d+))?)?/g;

interface RawMatch {
  start: number;
  end: number;
  path: string;
  line: number;
  column?: number;
}

// 把一行日志切成普通段 + 可点击 file:line 段（先 PY traceback 再 JSON，JSON 避开与 PY 重叠）。
export function tokenize(line: string): Segment[] {
  const matches: RawMatch[] = [];

  PY_RE.lastIndex = 0;
  let m: RegExpExecArray | null;
  while ((m = PY_RE.exec(line)) !== null) {
    const lineNo = Number(m[2]);
    if (!lineNo) continue;
    const raw = m[1] ?? '';
    // 点分模块名（无分隔符、非 .py 结尾）→ foo.bar.Baz → foo/bar/Baz.py
    const path =
      !raw.includes('/') && !raw.includes('\\') && !raw.endsWith('.py') ? raw.replace(/\./g, '/') + '.py' : raw;
    matches.push({ start: m.index, end: m.index + m[0].length, path, line: lineNo });
  }

  JSON_RE.lastIndex = 0;
  while ((m = JSON_RE.exec(line)) !== null) {
    const start = m.index;
    const end = m.index + m[0].length;
    if (matches.some((x) => x.start < end && x.end > start)) continue; // 与 PY 区间重叠则跳过
    const match: RawMatch = { start, end, path: m[1] ?? '', line: m[2] ? Number(m[2]) : 1 };
    if (m[3]) match.column = Number(m[3]);
    matches.push(match);
  }

  if (matches.length === 0) return [{ text: line }];
  matches.sort((a, b) => a.start - b.start);

  const segs: Segment[] = [];
  let cursor = 0;
  for (const mt of matches) {
    if (mt.start > cursor) segs.push({ text: line.slice(cursor, mt.start) });
    const link: LinkRef = { path: mt.path, line: mt.line };
    if (mt.column !== undefined) link.column = mt.column;
    segs.push({ text: line.slice(mt.start, mt.end), link });
    cursor = mt.end;
  }
  if (cursor < line.length) segs.push({ text: line.slice(cursor) });
  return segs;
}

// 相对路径按 projectPath 的分隔风格拼成绝对；已是绝对路径原样返回。
export function resolvePath(projectPath: string, p: string): string {
  if (/^([A-Za-z]:[\\/]|\/)/.test(p)) return p;
  const sep = projectPath.includes('\\') ? '\\' : '/';
  return projectPath.replace(/[\\/]$/, '') + sep + p.replace(/[\\/]/g, sep);
}
