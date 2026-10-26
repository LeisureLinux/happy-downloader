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
      {/* 下载框在最上面：这一页的主操作是输入 URL 开始下载 */}
      <DownloadForm />
      <Downloads />
      {/* 空状态提示放在列表的位置，只有没有任务时才出现 */}
      <Splash />
    </Container>
  )
}
