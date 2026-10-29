import { Alert, Box, Button, Snackbar } from "@mui/material"
import { Toast, toastListState } from '../atoms/toast'
import { useEffect } from 'react'
import { useAtom } from 'jotai'

const Toaster: React.FC = () => {
  const [toasts, setToasts] = useAtom(toastListState)

  // A toast lives as long as it was asked to. Two seconds is the default and
  // is right for an acknowledgement; a message with a button on it needs long
  // enough to be read and pressed.
  const deletePredicate = (t: Toast) =>
    (Date.now() - t.createdAt) < (t.durationMs ?? 2000)

  useEffect(() => {
    if (toasts.length > 0) {
      const closer = setInterval(() => {
        setToasts(t => t.map(t => ({ ...t, open: deletePredicate(t) })))
      }, 900)

      const cleaner = setInterval(() => {
        setToasts(t => t.filter(deletePredicate))
      }, 2005)

      return () => {
        clearInterval(closer)
        clearInterval(cleaner)
      }
    }
  }, [setToasts, toasts.length])

  return (
    <>
      {toasts.map((toast, index) => {
        const alert = (
          <Alert
            variant="filled"
            severity={toast.severity}
            sx={toast.actionLabel ? { boxShadow: 6, alignItems: 'center' } : undefined}
            action={toast.actionLabel ? (
              <Button
                color="inherit"
                size="small"
                onClick={() => {
                  toast.onAction?.()
                  setToasts(t => t.filter(x => x !== toast))
                }}
              >
                {toast.actionLabel}
              </Button>
            ) : undefined}
          >
            {toast.message}
          </Alert>
        )

        // A message with a button on it goes in the middle of the screen.
        //
        // It is not an acknowledgement — it is news about something that
        // finished somewhere else, and the whole point is that the person
        // notices it and can act on it. A toast in a corner is read as
        // "queued" and ignored; the middle is where the eye already is.
        if (toast.actionLabel) {
          return (
            <Box
              key={index}
              sx={{
                position: 'fixed',
                inset: 0,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                zIndex: 1500,
                // The page underneath stays usable: only the alert itself
                // takes a click.
                pointerEvents: 'none',
              }}
            >
              <Box sx={{ pointerEvents: 'auto', maxWidth: '90vw' }}>{alert}</Box>
            </Box>
          )
        }

        return (
          <Snackbar
            key={index}
            open={toast.open}
            sx={index > 0 ? { marginBottom: index * 6.5 } : null}
          >
            {alert}
          </Snackbar>
        )
      })}
    </>
  )
}

export default Toaster