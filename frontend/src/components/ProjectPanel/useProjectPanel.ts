// ProjectPanel 的 ViewModel：消费 ProjectContext + 编排原生对话框。
// 不返回 JSX、不持翻译串——提示文案由 View 传入（遵 MVVM 铁律）。
// add/remove 的后端错误（如 "Project already exists"）经 NativeDialogs.error 提示。
import { NativeDialogs, type Project } from '@/services';
import { useProjects } from '@/project';

// 提示串由 View（持 t()）传入。
export interface AddStrings {
  dialogTitle: string;
  errorTitle: string;
}
export interface RemoveStrings {
  confirmTitle: string;
  confirmMessage: string;
  okLabel: string;
  cancelLabel: string;
  errorTitle: string;
}

export interface ProjectPanelModel {
  projects: Project[];
  loading: boolean;
  add: (s: AddStrings) => Promise<void>;
  remove: (project: Project, s: RemoveStrings) => Promise<void>;
}

function messageOf(e: unknown): string {
  if (e instanceof Error) return e.message;
  return typeof e === 'string' ? e : String(e);
}

export function useProjectPanel(): ProjectPanelModel {
  const { projects, loading, addProject, removeProject } = useProjects();

  const add = async (s: AddStrings): Promise<void> => {
    const dir = await NativeDialogs.openDirectory({ title: s.dialogTitle });
    if (!dir) return;
    try {
      await addProject(dir);
    } catch (e) {
      await NativeDialogs.error(s.errorTitle, messageOf(e));
    }
  };

  const remove = async (project: Project, s: RemoveStrings): Promise<void> => {
    const ok = await NativeDialogs.confirm({
      title: s.confirmTitle,
      message: s.confirmMessage,
      okLabel: s.okLabel,
      cancelLabel: s.cancelLabel,
    });
    if (!ok) return;
    try {
      await removeProject(project.id);
    } catch (e) {
      await NativeDialogs.error(s.errorTitle, messageOf(e));
    }
  };

  return { projects, loading, add, remove };
}
