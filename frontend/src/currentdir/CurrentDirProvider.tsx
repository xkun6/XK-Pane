import { createContext, useContext, useMemo, useState, type ReactNode } from 'react';

// 「当前目录 / 文件」全局上下文 —— 「文件管理」功能的中枢。
//
// 三个状态分工：
//   - rootDir：文件浏览器的「树根」。由「打开文件夹」/「最近目录」/工作区右键
//     「在文件浏览器中打开」设定；切换它会重置文件树。
//   - currentDir：当前「聚焦目录」，是右侧 Git / 本地历史抽屉的绑定目标
//     （默认 = rootDir；在树里点子目录、或打开某文件时跟到其所在目录）。
//   - currentFile：当前在编辑器打开的文件（null = 仅看目录）。
//
// 放在 RouterProvider 之上：文件页、工作区树、Git/历史抽屉都在路由内，均可读写。
// 仅持状态，不耦合路由（navigate 由消费方在 RouterProvider 内自行调用）。
export interface CurrentDirContextValue {
  rootDir: string | null;
  currentDir: string | null;
  currentFile: string | null;
  setCurrentDir: (dir: string | null) => void;
  setCurrentFile: (file: string | null) => void;
  // 把某目录作为文件浏览器根打开：rootDir = currentDir = dir，清空 currentFile。
  // 供「打开文件夹」/「最近目录」/工作区右键「在文件浏览器中打开」调用。
  openInBrowser: (dir: string) => void;
}

const CurrentDirContext = createContext<CurrentDirContextValue | null>(null);

export const CurrentDirProvider = ({ children }: { children: ReactNode }) => {
  const [rootDir, setRootDir] = useState<string | null>(null);
  const [currentDir, setCurrentDir] = useState<string | null>(null);
  const [currentFile, setCurrentFile] = useState<string | null>(null);

  const value = useMemo<CurrentDirContextValue>(
    () => ({
      rootDir,
      currentDir,
      currentFile,
      setCurrentDir,
      setCurrentFile,
      openInBrowser: (dir: string) => {
        setRootDir(dir);
        setCurrentDir(dir);
        setCurrentFile(null);
      },
    }),
    [rootDir, currentDir, currentFile],
  );

  return <CurrentDirContext.Provider value={value}>{children}</CurrentDirContext.Provider>;
};

export const useCurrentDir = (): CurrentDirContextValue => {
  const ctx = useContext(CurrentDirContext);
  if (!ctx) {
    throw new Error('useCurrentDir 必须在 <CurrentDirProvider> 内使用');
  }
  return ctx;
};
