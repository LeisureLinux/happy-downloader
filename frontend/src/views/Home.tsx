import { Container } from '@mui/material'
import { loadingAtom } from '../atoms/ui'
import DownloadForm from '../components/DownloadForm'
import Downloads from '../components/Downloads'
import LoadingBackdrop from '../components/LoadingBackdrop'
import Splash from '../components/Splash'
import { useAtomValue } from 'jotai'

export default function Home() {
  const isLoading = useAtomValue(loadingAtom)

  return (
    <Container maxWidth="xl" sx={{ mt: 2, mb: 8 }}>
      <LoadingBackdrop isLoading={isLoading} />
      <Splash />
      {/* 下载表单在列表之上：输入 URL、看元数据、选参数、下载，全在这一页 */}
      <DownloadForm />
      <Downloads />
    </Container>
  )
}
