// 由文件扩展名推断 Monaco 语言 id（未知 → plaintext）。
// 一些常见无扩展名文件（Dockerfile / Makefile）按文件名匹配。
const EXT_MAP: Record<string, string> = {
  ts: 'typescript',
  tsx: 'typescript',
  js: 'javascript',
  jsx: 'javascript',
  mjs: 'javascript',
  cjs: 'javascript',
  json: 'json',
  jsonc: 'json',
  md: 'markdown',
  markdown: 'markdown',
  go: 'go',
  py: 'python',
  rs: 'rust',
  java: 'java',
  kt: 'kotlin',
  c: 'c',
  h: 'c',
  cpp: 'cpp',
  cc: 'cpp',
  cxx: 'cpp',
  hpp: 'cpp',
  cs: 'csharp',
  php: 'php',
  rb: 'ruby',
  swift: 'swift',
  html: 'html',
  htm: 'html',
  xml: 'xml',
  css: 'css',
  scss: 'scss',
  less: 'less',
  yaml: 'yaml',
  yml: 'yaml',
  toml: 'ini',
  ini: 'ini',
  cfg: 'ini',
  sh: 'shell',
  bash: 'shell',
  zsh: 'shell',
  bat: 'bat',
  cmd: 'bat',
  ps1: 'powershell',
  sql: 'sql',
  lua: 'lua',
  dart: 'dart',
  vue: 'html',
  svelte: 'html',
  graphql: 'graphql',
  gql: 'graphql',
  dockerfile: 'dockerfile',
};

const NAME_MAP: Record<string, string> = {
  dockerfile: 'dockerfile',
  makefile: 'makefile',
  '.gitignore': 'plaintext',
  '.env': 'ini',
};

export function languageForPath(path: string): string {
  const base = path.replace(/\\/g, '/').split('/').pop() ?? '';
  const lower = base.toLowerCase();
  if (lower in NAME_MAP) return NAME_MAP[lower] as string;
  const dot = lower.lastIndexOf('.');
  if (dot >= 0) {
    const ext = lower.slice(dot + 1);
    if (ext in EXT_MAP) return EXT_MAP[ext] as string;
  }
  return 'plaintext';
}
