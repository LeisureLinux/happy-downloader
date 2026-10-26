import { useAtom, useAtomValue } from 'jotai'
import { useEffect } from 'react'
import { loadingDownloadsState } from '../atoms/downloads'
import { loadingAtom } from '../atoms/ui'
import DownloadsTableView from './DownloadsTableView'

const Downloads: React.FC = () => {
  const loadingDownloads = useAtomValue(loadingDownloadsState)

  const [isLoading, setIsLoading] = useAtom(loadingAtom)

  useEffect(() => {
    if (loadingDownloads) {
      return setIsLoading(true)
    }
    setIsLoading(false)
  }, [loadingDownloads, isLoading])

  return <DownloadsTableView />
}

export default Downloads
