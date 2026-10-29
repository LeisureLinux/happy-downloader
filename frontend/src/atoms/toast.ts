import { AlertColor } from '@mui/material'
import { atom } from 'jotai'

export type Toast = {
  open: boolean,
  message: string
  autoClose: boolean
  createdAt: number,
  severity?: AlertColor
  // How long it stays, in milliseconds. The default is two seconds, which is
  // right for "queued" and far too short for something a person has to act
  // on — a recording that just finished is news, not an acknowledgement.
  durationMs?: number
  // A button on the toast. It is how "jump to the file" is offered: the
  // recording has appeared somewhere else, and telling somebody to go and
  // look is worse than taking them.
  actionLabel?: string
  onAction?: () => void
}

export const toastListState = atom<Toast[]>([])