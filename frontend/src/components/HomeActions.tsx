import { useSetAtom } from 'jotai'
import { Suspense, useState } from 'react'
import { loadingAtom } from '../atoms/ui'
import { useToast } from '../hooks/toast'
import DownloadDialog from './DownloadDialog'
import HomeSpeedDial from './HomeSpeedDial'

const HomeActions: React.FC = () => {
  const setIsLoading = useSetAtom(loadingAtom)

  const [openDownload, setOpenDownload] = useState(false)

  const { pushMessage } = useToast()

  return (
    <>
      <HomeSpeedDial onDownloadOpen={() => setOpenDownload(true)} />
      <Suspense>
        <DownloadDialog
          open={openDownload}
          onClose={() => {
            setOpenDownload(false)
            setIsLoading(true)
          }}
          // TODO: handle optimistic UI update
          onDownloadStart={(url) => {
            pushMessage(`Requested ${url}`, 'info')
            setOpenDownload(false)
            setIsLoading(true)
          }}
        />
      </Suspense>
    </>
  )
}

export default HomeActions