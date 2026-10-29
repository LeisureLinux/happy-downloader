import { Alert, Button, Snackbar } from "@mui/material"
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
      {toasts.map((toast, index) => (
        <Snackbar
          key={index}
          open={toast.open}
          sx={index > 0 ? { marginBottom: index * 6.5 } : null}
        >
          <Alert
            variant="filled"
            severity={toast.severity}
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
        </Snackbar>
      ))}
    </>
  )
}

export default Toaster