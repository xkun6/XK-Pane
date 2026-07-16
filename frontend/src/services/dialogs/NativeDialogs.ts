// NativeDialogsService：统一封装 Wails v3 的原生 dialog API。
//
// 规范：项目内所有"打开文件 / 保存文件 / 选择文件夹 / 确认对话框 / 错误提示"都
// 通过本服务调用，禁止使用浏览器 alert / confirm / prompt（在 webview 里观感差且
// 在 Wails 里行为不一致）。
//
// 实现细节：
//   - SaveFile / OpenFile / Info / Warning / Error / Question 全部走 @wailsio/runtime
//   - 把 PascalCase 字段名收口为本地的 camelCase API，对调用方更友好
//   - 用户取消时统一返回 null（而不是空字符串），避免上层判空写 if (path === '')
import { Dialogs } from '@wailsio/runtime';
import { isWindows } from '@/shared/platform';

export interface FileFilter {
  displayName: string;
  pattern: string; // e.g. "*.db;*.sqlite"
}

export interface OpenFileOptions {
  title?: string;
  message?: string;
  buttonText?: string;
  directory?: string;
  filters?: FileFilter[];
  allowsMultiple?: boolean;
  // 选择目录而非文件
  chooseDirectories?: boolean;
}

export interface SaveFileOptions {
  title?: string;
  message?: string;
  buttonText?: string;
  directory?: string;
  filename?: string;
  filters?: FileFilter[];
  canCreateDirectories?: boolean;
}

export interface ConfirmOptions {
  title?: string;
  message: string;
  okLabel?: string;
  cancelLabel?: string;
}

const mapFilters = (filters?: FileFilter[]) =>
  filters?.map((f) => ({ DisplayName: f.displayName, Pattern: f.pattern }));

export const NativeDialogs = {
  // 打开文件选择器（单选）。取消返回 null。
  async openFile(options: OpenFileOptions = {}): Promise<string | null> {
    const result = await Dialogs.OpenFile({
      Title: options.title,
      Message: options.message,
      ButtonText: options.buttonText,
      Directory: options.directory,
      Filters: mapFilters(options.filters),
      CanChooseDirectories: options.chooseDirectories ?? false,
      CanChooseFiles: !options.chooseDirectories,
      AllowsMultipleSelection: false,
    });
    return result && result !== '' ? result : null;
  },

  // 打开文件选择器（多选）。取消返回空数组。
  async openFiles(options: OpenFileOptions = {}): Promise<string[]> {
    const result = await Dialogs.OpenFile({
      Title: options.title,
      Message: options.message,
      ButtonText: options.buttonText,
      Directory: options.directory,
      Filters: mapFilters(options.filters),
      CanChooseDirectories: options.chooseDirectories ?? false,
      CanChooseFiles: !options.chooseDirectories,
      AllowsMultipleSelection: true,
    });
    return Array.isArray(result) ? result : [];
  },

  // 选择文件夹。取消返回 null。
  async openDirectory(options: Omit<OpenFileOptions, 'chooseDirectories' | 'filters'> = {}): Promise<string | null> {
    const result = await Dialogs.OpenFile({
      Title: options.title,
      Message: options.message,
      ButtonText: options.buttonText,
      Directory: options.directory,
      CanChooseDirectories: true,
      CanChooseFiles: false,
      CanCreateDirectories: true,
      AllowsMultipleSelection: false,
    });
    return result && result !== '' ? result : null;
  },

  // 保存文件对话框。取消返回 null。
  async saveFile(options: SaveFileOptions = {}): Promise<string | null> {
    const result = await Dialogs.SaveFile({
      Title: options.title,
      Message: options.message,
      ButtonText: options.buttonText,
      Directory: options.directory,
      Filename: options.filename,
      Filters: mapFilters(options.filters),
      CanCreateDirectories: options.canCreateDirectories ?? true,
    });
    return result && result !== '' ? result : null;
  },

  // 二选一确认。OK 返回 true，Cancel / 关闭返回 false。
  //
  // 平台差异（Wails v3 alpha）：Windows 的 Question 对话框硬编码 MB_YESNO——**忽略**
  // 自定义按钮 label，只显示系统本地化的「是/否」，且仅当某按钮 Label 字面等于映射回的
  // "Yes"/"No" 时才触发其回调；否则后端 resp 通道永远收不到值，前端 Promise 永久挂起
  // （表现为「点是后毫无反应」）。故 Windows 必须用 "Yes"/"No" 作 label（反正不可见），
  // 并据此判定。macOS（NSAlert）/ Linux（GTK）原生支持自定义 label，回传该 label 本身。
  async confirm(options: ConfirmOptions): Promise<boolean> {
    if (isWindows) {
      const choice = await Dialogs.Question({
        Title: options.title,
        Message: options.message,
        Buttons: [
          { Label: 'Yes', IsDefault: true },
          { Label: 'No', IsCancel: true },
        ],
      });
      return choice === 'Yes';
    }
    const okLabel = options.okLabel ?? 'OK';
    const cancelLabel = options.cancelLabel ?? 'Cancel';
    const choice = await Dialogs.Question({
      Title: options.title,
      Message: options.message,
      Buttons: [
        { Label: okLabel, IsDefault: true },
        { Label: cancelLabel, IsCancel: true },
      ],
    });
    return choice === okLabel;
  },

  // Info / Warning / Error 必须显式传一个 Label 为 "Ok" 的按钮：Windows 侧这三类对话框
  // 走 MB_OK MessageBox，点击「确定」后返回值被映射为字符串 "Ok"（精确大小写，见 wails
  // dialogs_windows.go 的 responses 表），随后仅当 Buttons 里存在 Label 字面等于 "Ok" 的
  // 按钮时才触发其回调——没有匹配按钮则后端 resp 通道永远收不到值，前端 Promise 永久
  // 挂起（与上方 confirm 注释描述的 Question/MB_YESNO 是同一机制）。macOS / Linux 会把
  // 该按钮真实渲染出来并回传 label 本身，同样可靠 resolve。
  async info(title: string, message: string): Promise<void> {
    await Dialogs.Info({ Title: title, Message: message, Buttons: [{ Label: 'Ok', IsDefault: true }] });
  },

  async warning(title: string, message: string): Promise<void> {
    await Dialogs.Warning({ Title: title, Message: message, Buttons: [{ Label: 'Ok', IsDefault: true }] });
  },

  async error(title: string, message: string): Promise<void> {
    await Dialogs.Error({ Title: title, Message: message, Buttons: [{ Label: 'Ok', IsDefault: true }] });
  },
} as const;
