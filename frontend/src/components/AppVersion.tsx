import { Suspense } from 'react'
import { Typography } from '@mui/material'
import { ytdlpRpcVersionState } from '../atoms/status'
import { useAtomValue } from 'jotai'

// The app version, shown next to the product name in the header.
//
// The version atom is a Promise atom: reading it without Suspense yields the
// Promise itself, so `version.rpcVersion` is undefined and a conditional
// render on it re-renders until React aborts with "too many re-renders".
// Wrapping the reader in Suspense lets jotai resolve it first.
//
// The build timestamp travels with the version because the version string
// alone does not distinguish a stale binary from a current one — that check
// was being skipped by looking at the file mtime instead.
const AppVersion: React.FC = () => {
  const version = useAtomValue(ytdlpRpcVersionState)

  // A plain "go build" injects "dev" and no timestamp; show nothing rather
  // than a misleading "vdev".
  if (!version.rpcVersion || version.rpcVersion === 'dev') return null

  return (
    <Typography
      component="span"
      variant="caption"
      sx={{ ml: 1.5, opacity: 0.7, fontFamily: 'Roboto Mono' }}
    >
      {`v${version.rpcVersion}${buildStamp(version.rpcBuiltAt)}`}
    </Typography>
  )
}

const buildStamp = (builtAt?: string): string => {
  if (!builtAt || builtAt === '0') return ''
  const n = Number(builtAt)
  if (!Number.isFinite(n) || n <= 0) return ''
  return `@${n}`
}

const AppVersionWithSuspense: React.FC = () => (
  <Suspense fallback={null}>
    <AppVersion />
  </Suspense>
)

export default AppVersionWithSuspense
