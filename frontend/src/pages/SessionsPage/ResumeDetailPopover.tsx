import { useState, type MouseEvent } from 'react';
import { Box, Button, IconButton, Popover, Tooltip } from '@mui/material';
import InfoOutlinedIcon from '@mui/icons-material/InfoOutlined';
import ContentCopyRoundedIcon from '@mui/icons-material/ContentCopyRounded';
import PlayArrowRoundedIcon from '@mui/icons-material/PlayArrowRounded';
import DeleteOutlineRoundedIcon from '@mui/icons-material/DeleteOutlineRounded';
import type { LaunchRecord } from '@/services';
import type { SessionsPageSx } from './SessionsPage.styles';

// 文案由父（持 t()）传入，遵 MVVM。
export interface ResumeDetailStrings {
  detail: string;
  sessionId: string;
  launchTime: string;
  lastPrompt: string;
  copyId: string;
  resume: string;
  delete: string;
}

export interface ResumeDetailPopoverProps {
  record: LaunchRecord;
  styles: SessionsPageSx;
  strings: ResumeDetailStrings;
  onResume: (record: LaunchRecord) => void;
  onDelete: (id: number) => void;
}

function truncateId(id: string): string {
  return id.length > 16 ? `${id.slice(0, 8)}...${id.slice(-8)}` : id;
}

function formatFullTime(ts: string): string {
  if (!ts) return '';
  const d = new Date(ts);
  return Number.isNaN(d.getTime()) ? ts : d.toLocaleString();
}

// 启动记录详情气泡：项目名/路径 + session id（截断 + 复制）+ 启动时间 + lastPrompt（3 行截断）
// + Resume / Delete。移植 cc-pane ResumeDetailPopover；XK 无 reveal 能力，省去「在文件夹打开」。
export const ResumeDetailPopover = ({
  record,
  styles,
  strings,
  onResume,
  onDelete,
}: ResumeDetailPopoverProps) => {
  const [anchorEl, setAnchorEl] = useState<HTMLElement | null>(null);
  const sessionId = record.resumeSessionId ?? '';

  const close = (): void => setAnchorEl(null);

  const handleCopy = (e: MouseEvent): void => {
    e.stopPropagation();
    void navigator.clipboard?.writeText(sessionId).catch(() => {});
  };

  return (
    <>
      <Tooltip title={strings.detail}>
        <IconButton
          size="small"
          aria-label={strings.detail}
          onClick={(e) => {
            e.stopPropagation();
            setAnchorEl(e.currentTarget);
          }}
        >
          <InfoOutlinedIcon fontSize="small" />
        </IconButton>
      </Tooltip>
      <Popover
        open={anchorEl !== null}
        anchorEl={anchorEl}
        onClose={close}
        anchorOrigin={{ vertical: 'center', horizontal: 'right' }}
        transformOrigin={{ vertical: 'center', horizontal: 'left' }}
        onClick={(e) => e.stopPropagation()}
        slotProps={{ paper: { sx: styles.popover } }}
      >
        <Box>
          <Box sx={styles.popProjectName}>{record.projectName}</Box>
          <Box sx={styles.popProjectPath} title={record.projectPath}>
            {record.projectPath}
          </Box>
        </Box>

        <Box sx={styles.popRow}>
          <Box component="span" sx={styles.popLabel}>
            {strings.sessionId}:
          </Box>
          <Box component="code" sx={styles.popCode}>
            {truncateId(sessionId)}
          </Box>
          <Tooltip title={strings.copyId}>
            <IconButton size="small" aria-label={strings.copyId} onClick={handleCopy}>
              <ContentCopyRoundedIcon sx={{ fontSize: 14 }} />
            </IconButton>
          </Tooltip>
        </Box>

        <Box sx={styles.popRow}>
          <Box component="span" sx={styles.popLabel}>
            {strings.launchTime}:
          </Box>
          <Box component="span" sx={styles.popValue}>
            {formatFullTime(record.launchedAt)}
          </Box>
        </Box>

        {record.lastPrompt && (
          <Box>
            <Box component="span" sx={styles.popLabel}>
              {strings.lastPrompt}:
            </Box>
            <Box sx={styles.popPrompt}>{record.lastPrompt}</Box>
          </Box>
        )}

        <Box sx={styles.popActions}>
          <Button
            size="small"
            variant="contained"
            startIcon={<PlayArrowRoundedIcon />}
            sx={{ textTransform: 'none' }}
            onClick={(e) => {
              e.stopPropagation();
              close();
              onResume(record);
            }}
          >
            {strings.resume}
          </Button>
          <Button
            size="small"
            variant="outlined"
            color="error"
            startIcon={<DeleteOutlineRoundedIcon />}
            sx={{ textTransform: 'none' }}
            onClick={(e) => {
              e.stopPropagation();
              close();
              onDelete(record.id);
            }}
          >
            {strings.delete}
          </Button>
        </Box>
      </Popover>
    </>
  );
};
