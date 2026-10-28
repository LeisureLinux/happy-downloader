import { useCallback, useEffect, useState } from 'react'
import {
  Alert,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  List,
  ListItem,
  ListItemButton,
  ListItemText,
  Stack,
  TextField,
  Typography
} from '@mui/material'
import CreateNewFolderIcon from '@mui/icons-material/CreateNewFolder'
import DriveFileMoveIcon from '@mui/icons-material/DriveFileMove'
import DriveFileRenameOutlineIcon from '@mui/icons-material/DriveFileRenameOutline'
import { useAtomValue } from 'jotai'
import { serverURL } from '../atoms/settings'
import { useI18n } from '../hooks/useI18n'
import { useToast } from '../hooks/toast'
import { DirectoryEntry } from '../types'

// Making a folder, and moving files into one.
//
// The two are the same dialog, because they are the same idea: a folder is
// made by giving it a name, and it is filled by moving files into it.

// A folder to make, or to move files into.
type Props = {
  open: boolean
  /** The directory the folder is made in. */
  parent: string
  /** The folders that can be moved into. */
  folders: DirectoryEntry[]
  /** The files to move. Empty means "make a folder". */
  files?: string[]
  /** The name of the folder to make, when making one. */
  mode: 'create' | 'move' | 'rename'
  /** The folder to rename, when renaming one. */
  folder?: DirectoryEntry
  onClose: () => void
  onDone: () => void
}

const FolderDialog: React.FC<Props> = ({
  open, parent, folders, files = [], mode, folder, onClose, onDone,
}) => {
  const serverAddr = useAtomValue(serverURL)
  const { i18n } = useI18n()
  const { pushMessage } = useToast()

  const [name, setName] = useState('')
  const [target, setTarget] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (open) {
      // Renaming starts from the name the folder has, so the field shows what
      // it is now rather than an empty box.
      setName(mode === 'rename' ? (folder?.name ?? '') : '')
      setTarget('')
      setError('')
    }
  }, [open, mode, folder])

  const create = async () => {
    setBusy(true)
    setError('')

    try {
      const res = await fetch(`${serverAddr}/filebrowser/folder`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name, subdir: parent }),
      })

      if (!res.ok) {
        throw new Error(await res.text())
      }

      pushMessage(i18n.t('folderCreated').replace('{name}', name), 'success')
      onDone()
      onClose()
    } catch (e) {
      setError(String(e))
    } finally {
      setBusy(false)
    }
  }

  const rename = async () => {
    if (!folder) return

    setBusy(true)
    setError('')

    try {
      const res = await fetch(`${serverAddr}/filebrowser/folder`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ path: folder.path, name }),
      })

      if (!res.ok) {
        throw new Error(await res.text())
      }

      pushMessage(i18n.t('folderRenamed').replace('{name}', name), 'success')
      onDone()
      onClose()
    } catch (e) {
      setError(String(e))
    } finally {
      setBusy(false)
    }
  }

  const move = async () => {
    setBusy(true)
    setError('')

    try {
      const res = await fetch(`${serverAddr}/filebrowser/move`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ paths: files, target }),
      })

      if (!res.ok) {
        throw new Error(await res.text())
      }

      const result = await res.json()
      const failures = result.failures?.length ?? 0

      if (failures > 0) {
        pushMessage(
          i18n.t('folderMoveFailed').replace('{count}', String(failures)),
          'error'
        )
      } else {
        pushMessage(
          i18n.t('folderMoved').replace('{count}', String(result.moved?.length ?? 0)),
          'success'
        )
      }

      onDone()
      onClose()
    } catch (e) {
      setError(String(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>
        <Stack direction="row" spacing={1} alignItems="center">
          {mode === 'create'
            ? <CreateNewFolderIcon />
            : mode === 'rename'
              ? <DriveFileRenameOutlineIcon />
              : <DriveFileMoveIcon />}
          <span>
            {mode === 'create'
              ? i18n.t('folderCreateTitle')
              : mode === 'rename'
                ? i18n.t('folderRenameTitle')
                : i18n.t('folderMoveTitle')}
          </span>
        </Stack>
      </DialogTitle>

      <DialogContent>
        {mode === 'create' || mode === 'rename' ? (
          <TextField
            autoFocus
            fullWidth
            label={i18n.t('folderNameLabel')}
            value={name}
            onChange={e => setName(e.target.value)}
            onKeyDown={e => {
              if (e.key !== 'Enter' || !name.trim()) return
              if (mode === 'rename') void rename()
              else void create()
            }}
            sx={{ mt: 1 }}
          />
        ) : (
          <>
            <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
              {i18n.t('folderMoveHint').replace('{count}', String(files.length))}
            </Typography>

            {folders.length === 0 ? (
              <Alert severity="info">{i18n.t('folderMoveNoFolders')}</Alert>
            ) : (
              <List dense disablePadding sx={{ maxHeight: 320, overflow: 'auto' }}>
                {folders.map(f => (
                  <ListItem key={f.path} disableGutters>
                    <ListItemButton
                      selected={target === f.path}
                      onClick={() => setTarget(f.path)}
                    >
                      <ListItemText
                        primary={f.name}
                        secondary={i18n.t('folderFileCount')
                          .replace('{count}', String(f.fileCount ?? 0))}
                      />
                    </ListItemButton>
                  </ListItem>
                ))}
              </List>
            )}
          </>
        )}

        {error && (
          <Alert severity="error" sx={{ mt: 2 }}>
            {error}
          </Alert>
        )}
      </DialogContent>

      <DialogActions>
        <Button onClick={onClose}>{i18n.t('cancel')}</Button>
        {mode === 'create' ? (
          <Button
            variant="contained"
            disabled={busy || !name.trim()}
            onClick={create}
          >
            {i18n.t('folderCreateButton')}
          </Button>
        ) : mode === 'rename' ? (
          <Button
            variant="contained"
            disabled={busy || !name.trim() || name === folder?.name}
            onClick={rename}
          >
            {i18n.t('folderRenameButton')}
          </Button>
        ) : (
          <Button
            variant="contained"
            disabled={busy || !target}
            onClick={move}
          >
            {i18n.t('folderMoveButton')}
          </Button>
        )}
      </DialogActions>
    </Dialog>
  )
}

export default FolderDialog
