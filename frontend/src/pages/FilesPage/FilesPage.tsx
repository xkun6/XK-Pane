import { Box, Button, IconButton, InputBase, Tooltip, Typography, useTheme } from '@mui/material';
import FolderOpenRoundedIcon from '@mui/icons-material/FolderOpenRounded';
import FolderRoundedIcon from '@mui/icons-material/FolderRounded';
import FolderOffRoundedIcon from '@mui/icons-material/FolderOffRounded';
import OpenInNewRoundedIcon from '@mui/icons-material/OpenInNewRounded';
import AccountTreeRoundedIcon from '@mui/icons-material/AccountTreeRounded';
import HistoryRoundedIcon from '@mui/icons-material/HistoryRounded';
import SearchRoundedIcon from '@mui/icons-material/SearchRounded';
import CloseRoundedIcon from '@mui/icons-material/CloseRounded';
import TextFieldsRoundedIcon from '@mui/icons-material/TextFieldsRounded';
import ManageSearchRoundedIcon from '@mui/icons-material/ManageSearchRounded';
import { useT } from '@/i18n';
import { FileSystemTree } from '@/components/FileSystemTree';
import { filesPageStyles } from './FilesPage.styles';
import { useFilesPage, filesPathBase } from './useFilesPage';
import { FileEditorPane } from './FileEditorPane';
import { ContentSearchResults } from './ContentSearchResults';
import { QuickOpen } from './QuickOpen';

// 「文件」页：左文件树 + 右 Monaco 编辑器（markdown 可切预览）。桌面专属；Web 端降级提示。
// 选目录/文件会写入共享「当前目录/文件」上下文，供右侧 Git / 历史抽屉绑定。
export const FilesPage = () => {
  const theme = useTheme();
  const styles = filesPageStyles(theme);
  const t = useT();
  const vm = useFilesPage();

  if (!vm.isDesktop) {
    return (
      <Box sx={styles.root}>
        <Box sx={styles.webOnly}>
          <FolderOffRoundedIcon fontSize="large" />
          <Typography>{t('files.desktopOnly')}</Typography>
        </Box>
      </Box>
    );
  }

  return (
    <Box sx={styles.root}>
      <Box sx={styles.left}>
        <Box sx={styles.leftHeader}>
          <Typography sx={styles.rootName} title={vm.rootDir ?? ''}>
            {vm.rootDir ? filesPathBase(vm.rootDir) : t('files.noFolder')}
          </Typography>
          <Tooltip title={t('route.git')} arrow>
            <span>
              <IconButton
                size="small"
                sx={vm.activePanel === 'git' ? styles.headerBtnActive : styles.headerBtn}
                disabled={!vm.rootDir}
                onClick={() => vm.togglePanel('git')}
                aria-label={t('route.git')}
                aria-pressed={vm.activePanel === 'git'}
              >
                <AccountTreeRoundedIcon fontSize="small" />
              </IconButton>
            </span>
          </Tooltip>
          <Tooltip title={t('route.history')} arrow>
            <span>
              <IconButton
                size="small"
                sx={vm.activePanel === 'history' ? styles.headerBtnActive : styles.headerBtn}
                disabled={!vm.rootDir}
                onClick={() => vm.togglePanel('history')}
                aria-label={t('route.history')}
                aria-pressed={vm.activePanel === 'history'}
              >
                <HistoryRoundedIcon fontSize="small" />
              </IconButton>
            </span>
          </Tooltip>
          <Tooltip title={t('files.reveal')} arrow>
            <span>
              <IconButton
                size="small"
                sx={styles.headerBtn}
                disabled={!vm.rootDir && !vm.selectedFile}
                onClick={vm.reveal}
                aria-label={t('files.reveal')}
              >
                <OpenInNewRoundedIcon fontSize="small" />
              </IconButton>
            </span>
          </Tooltip>
          <Tooltip title={t('files.openFolder')} arrow>
            <IconButton
              size="small"
              sx={styles.headerBtn}
              onClick={() => void vm.openFolder()}
              aria-label={t('files.openFolder')}
            >
              <FolderOpenRoundedIcon fontSize="small" />
            </IconButton>
          </Tooltip>
        </Box>

        {vm.rootDir ? (
          <>
            <Box sx={styles.searchRow}>
              <Tooltip
                title={vm.searchMode === 'content' ? t('files.content.toName') : t('files.content.toContent')}
                arrow
              >
                <IconButton
                  size="small"
                  sx={styles.searchModeBtn}
                  onClick={() => vm.setSearchMode(vm.searchMode === 'content' ? 'name' : 'content')}
                  aria-label={t('files.content.modeToggle')}
                >
                  {vm.searchMode === 'content' ? (
                    <ManageSearchRoundedIcon fontSize="small" />
                  ) : (
                    <TextFieldsRoundedIcon fontSize="small" />
                  )}
                </IconButton>
              </Tooltip>
              <SearchRoundedIcon fontSize="small" sx={styles.searchIcon} />
              <InputBase
                value={vm.treeQuery}
                onChange={(e) => vm.setTreeQuery(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && vm.searchMode === 'content') void vm.runContentSearch();
                }}
                placeholder={
                  vm.searchMode === 'content'
                    ? t('files.content.placeholder')
                    : t('files.search.placeholder')
                }
                sx={styles.searchInput}
                inputProps={{
                  'aria-label':
                    vm.searchMode === 'content'
                      ? t('files.content.placeholder')
                      : t('files.search.placeholder'),
                }}
              />
              {vm.treeQuery && (
                <IconButton
                  size="small"
                  sx={styles.searchClear}
                  onClick={() => vm.setTreeQuery('')}
                  aria-label={t('files.search.clear')}
                >
                  <CloseRoundedIcon fontSize="small" />
                </IconButton>
              )}
            </Box>
            {vm.searchMode === 'content' ? (
              <ContentSearchResults
                results={vm.contentResults}
                term={vm.contentTerm}
                loading={vm.contentLoading}
                searched={vm.contentSearched}
                selectedPath={vm.selectedFile}
                onOpen={vm.openContentResult}
              />
            ) : (
              <FileSystemTree
                rootPath={vm.rootDir}
                selectedPath={vm.selectedFile}
                onSelectFile={(e) => void vm.selectFile(e)}
                onActivateDir={vm.activateDir}
                onAfterMutate={vm.onTreeMutate}
                onOpenInTerminal={vm.openInTerminal}
                searchQuery={vm.treeQuery}
              />
            )}
          </>
        ) : (
          <Box sx={styles.empty}>
            <Typography sx={styles.emptyTitle}>{t('files.empty.title')}</Typography>
            <Typography sx={styles.emptyHint}>{t('files.empty.hint')}</Typography>
            <Button
              sx={styles.openBtn}
              startIcon={<FolderOpenRoundedIcon />}
              onClick={() => void vm.openFolder()}
            >
              {t('files.empty.openFolder')}
            </Button>
            {vm.recent.length > 0 && (
              <Box sx={styles.recentBox}>
                <Typography sx={styles.recentTitle}>{t('files.recent.title')}</Typography>
                {vm.recent.map((r) => (
                  <Box
                    key={r.id}
                    sx={styles.recentRow}
                    onClick={() => vm.openRecent(r.path)}
                    title={r.path}
                  >
                    <FolderRoundedIcon fontSize="small" />
                    <Typography sx={styles.recentName}>{filesPathBase(r.path)}</Typography>
                  </Box>
                ))}
              </Box>
            )}
          </Box>
        )}
      </Box>

      <FileEditorPane vm={vm} />

      <QuickOpen
        open={vm.quickOpenOpen}
        rootPath={vm.rootDir}
        onClose={vm.closeQuickOpen}
        onPick={vm.openFileByPath}
      />
    </Box>
  );
};
