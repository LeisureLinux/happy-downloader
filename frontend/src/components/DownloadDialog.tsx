import { FileUpload } from '@mui/icons-material'
import CloseIcon from '@mui/icons-material/Close'
import {
  Autocomplete,
  Box,
  Button,
  Checkbox,
  Container,
  FormControl,
  FormControlLabel,
  Grid,
  IconButton,
  InputAdornment,
  MenuItem,
  Paper,
  Select,
  SelectChangeEvent,
  Stack,
  TextField
} from '@mui/material'
import AppBar from '@mui/material/AppBar'
import Dialog from '@mui/material/Dialog'
import Slide from '@mui/material/Slide'
import Toolbar from '@mui/material/Toolbar'
import Typography from '@mui/material/Typography'
import { TransitionProps } from '@mui/material/transitions'
import { useAtom, useAtomValue } from 'jotai'
import {
  FC,
  Suspense,
  forwardRef,
  useRef,
  useState,
  useTransition
} from 'react'
import {
  cookiesTemplateState,
  customArgsState,
  filenameTemplateState,
  savedTemplatesState
} from '../atoms/downloadTemplate'
import { settingsState } from '../atoms/settings'
import { availableDownloadPathsState, connectedState } from '../atoms/status'
import DownloadOptionsPanel from './DownloadOptionsPanel'
import {
  buildDownloadArgs,
  defaultOptions,
  renderArgs,
  type DownloadOptions
} from '../lib/downloadOptions'
import { useToast } from '../hooks/toast'
import { useI18n } from '../hooks/useI18n'
import { useRPC } from '../hooks/useRPC'
import type { DLMetadata } from '../types'
import CustomArgsTextField from './CustomArgsTextField'
import ExtraDownloadOptions from './ExtraDownloadOptions'
import LoadingBackdrop from './LoadingBackdrop'

const Transition = forwardRef(function Transition(
  props: TransitionProps & {
    children: React.ReactElement
  },
  ref: React.Ref<unknown>,
) {
  return <Slide direction="up" ref={ref} {...props} />
})

type Props = {
  open: boolean
  onClose: () => void
  onDownloadStart: (url: string) => void
}

const DownloadDialog: FC<Props> = ({ open, onClose, onDownloadStart }) => {
  const settings = useAtomValue(settingsState)
  const isConnected = useAtomValue(connectedState)
  const availableDownloadPaths = useAtomValue(availableDownloadPathsState)
  const savedTemplates = useAtomValue(savedTemplatesState)
  const customArgs = useAtomValue(customArgsState)
  const cookies = useAtomValue(cookiesTemplateState)

  const [downloadFormats, setDownloadFormats] = useState<DLMetadata>()
  const [options, setOptions] = useState<DownloadOptions>(defaultOptions)
  const [isFormatsLoading, setIsFormatsLoading] = useState(false)

  const [downloadPath, setDownloadPath] = useState('')

  const [filenameTemplate, setFilenameTemplate] = useAtom(
    filenameTemplateState
  )

  const [fileExtension, setFileExtension] = useState('.%(ext)s')

  const [url, setUrl] = useState('')

  const [isPlaylist, setIsPlaylist] = useState(false)

  const { i18n } = useI18n()
  const { client } = useRPC()
  const { pushMessage } = useToast()

  const urlInputRef = useRef<HTMLInputElement>(null)
  const customFilenameInputRef = useRef<HTMLInputElement>(null)

  const [isPending, startTransition] = useTransition()

  /**
    * Retrive url from input, cli-arguments from checkboxes and emits via WebSocket
  */
  const sendUrl = async (immediate?: string) => {
    for (const line of url.split('\n')) {
      // Options chosen in the dialog, rendered as the same flags the preview
      // showed, followed by any custom args from Settings.
      const optionArgs = renderArgs(buildDownloadArgs(options))
      const downloadTemplate = `${optionArgs} ${customArgs}`
        .replace(/  +/g, ' ')
        .trim()

      await new Promise(r => setTimeout(r, 10))
      client.download({
        url: immediate || line,
        args: downloadTemplate,
        pathOverride: downloadPath ?? '',
        renameTo: settings.fileRenaming ? filenameTemplate + (settings.autoFileExtension ? fileExtension : '') : '',
        playlist: isPlaylist,
      })

      setTimeout(() => {
        resetInput()
        setDownloadFormats(undefined)
        onDownloadStart(immediate || line)
      }, 100)
    }

    setUrl('')
  }

  /**
   * Retrive url from input and display the formats selection view
   */
  const sendUrlFormatSelection = () => {
    if (isPlaylist) {
      pushMessage('Format selection on playlist is not supported', 'warning')
      resetInput()
      onClose()
      return
    }

    setIsFormatsLoading(true)

    client.formats(url)
      ?.then(formats => {
        if (formats.result._type === 'playlist') {
          pushMessage('Format selection on playlist is not supported. Downloading as playlist.', 'info')
          resetInput()
          onClose()
          return
        }
        setDownloadFormats(formats.result)
        resetInput()
      })
      .then(() => setIsFormatsLoading(false))
  }

  const handleUrlChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setUrl(e.target.value)
  }

  const handleFilenameTemplateChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setFilenameTemplate(e.target.value)
  }

  const handleFileExtensionChange = (e: SelectChangeEvent<string>) => {
    setFileExtension(e.target.value)
  }

  const parseUrlListFile = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.currentTarget.files
    if (!files || files.length < 1) {
      return
    }

    const file = await files[0].text()

    file
      .split('\n')
      .forEach(u => sendUrl(u))
  }

  const resetInput = () => {
    urlInputRef.current!.value = ''
    if (customFilenameInputRef.current) {
      customFilenameInputRef.current!.value = ''
    }
  }

  return (
    <Dialog
      fullScreen
      open={open}
      onClose={onClose}
      TransitionComponent={Transition}
    >
      <LoadingBackdrop isLoading={isPending || isFormatsLoading} />
      <AppBar sx={{ position: 'relative' }}>
        <Toolbar>
          <IconButton
            edge="start"
            color="inherit"
            onClick={onClose}
            aria-label="close"
          >
            <CloseIcon />
          </IconButton>
          <Typography sx={{ ml: 2, flex: 1 }} variant="h6" component="div">
            Download
          </Typography>
        </Toolbar>
      </AppBar>
      <Box sx={{
        backgroundColor: (theme) => theme.palette.background.default,
        minHeight: (theme) => `calc(99vh - ${theme.mixins.toolbar.minHeight}px)`
      }}>
        <Container sx={{ my: 4 }}>
          <Grid container spacing={2}>
            <Grid item xs={12}>
              <Paper
                elevation={4}
                sx={{
                  p: 2,
                  display: 'flex',
                  flexDirection: 'column',
                }}
              >
                <Grid container>
                  <TextField
                    multiline
                    fullWidth
                    ref={urlInputRef}
                    label={i18n.t('urlInput')}
                    variant="outlined"
                    onChange={handleUrlChange}
                    disabled={
                      !isConnected
                      || (settings.formatSelection && downloadFormats != null)
                    }
                    InputProps={{
                      endAdornment: (
                        <InputAdornment position="end">
                          <label htmlFor="icon-button-file">
                            <input
                              hidden
                              id="icon-button-file"
                              type="file"
                              accept=".txt"
                              onChange={e => parseUrlListFile(e)}
                            />
                            <IconButton
                              color="primary"
                              aria-label="upload file"
                              component="span"
                            >
                              <FileUpload />
                            </IconButton>
                          </label>
                        </InputAdornment>
                      ),
                    }}
                  />
                </Grid>
                <Grid container spacing={1} sx={{ mt: 1 }}>
                  {settings.enableCustomArgs &&
                    <Grid item xs={12}>
                      <CustomArgsTextField />
                    </Grid>
                  }
                  {
                    settings.fileRenaming &&
                    <Grid item xs={
                      !settings.autoFileExtension && !settings.pathOverriding
                        ? 12
                        : !settings.autoFileExtension && settings.pathOverriding
                          ? 8
                          : settings.autoFileExtension && !settings.pathOverriding
                            ? 10
                            : 6
                    }>
                      <TextField
                        sx={{ mt: 1 }}
                        ref={customFilenameInputRef}
                        fullWidth
                        label={i18n.t('customFilename')}
                        variant="outlined"
                        value={filenameTemplate}
                        onChange={handleFilenameTemplateChange}
                        disabled={
                          !isConnected ||
                          (settings.formatSelection && downloadFormats != null)
                        }
                      />
                    </Grid>
                  }
                  {
                    settings.autoFileExtension &&
                    <Grid item xs={2}>
                      <Select
                        sx={{ mt: 1 }}
                        fullWidth
                        label={i18n.t('autoFileExtension')}
                        value={fileExtension}
                        onChange={handleFileExtensionChange}
                        variant="outlined">
                        <MenuItem value=".%(ext)s">Auto</MenuItem>
                        <MenuItem value=".mp4">mp4</MenuItem>
                        <MenuItem value=".mkv">mkv</MenuItem>
                      </Select>
                    </Grid>
                  }
                  {
                    settings.pathOverriding &&
                    <Grid item xs={4}>
                      <FormControl fullWidth>
                        <Autocomplete
                          disablePortal
                          options={availableDownloadPaths.map((dir) => ({ label: dir, dir }))}
                          autoHighlight
                          getOptionLabel={(option) => option.label}
                          onChange={(_, value) => {
                            setDownloadPath(value?.dir!)
                          }}
                          renderOption={(props, option) => (
                            <Box
                              component="li"
                              sx={{ '& > img': { mr: 2, flexShrink: 0 } }}
                              {...props}>
                              {option.label}
                            </Box>
                          )}
                          sx={{ width: '100%', mt: 1 }}
                          renderInput={(params) => <TextField {...params} label={i18n.t('customPath')} />}
                        />
                      </FormControl>
                    </Grid>
                  }
                </Grid>
                <Suspense>
                  {savedTemplates.length > 0 && <ExtraDownloadOptions />}
                </Suspense>
                <Grid container spacing={1} pt={2} justifyContent="space-between">
                  <Grid item>
                    <Grid item>
                      <FormControlLabel
                        control={<Checkbox onChange={() => setIsPlaylist(state => !state)} />}
                        checked={isPlaylist}
                        label={i18n.t('playlistCheckbox')}
                      />
                    </Grid>
                  </Grid>
                  <Grid item>
                    <Button
                      variant="contained"
                      disabled={url === ''}
                      onClick={() => settings.formatSelection
                        ? startTransition(() => sendUrlFormatSelection())
                        : startTransition(async () => await sendUrl())
                      }
                    >
                      {
                        settings.formatSelection
                          ? i18n.t('selectFormatButton')
                          : i18n.t('startButton')
                      }
                    </Button>
                  </Grid>
                </Grid>
              </Paper>
            </Grid>
          </Grid >

          {/* Metadata, once the URL has been probed */}
          {downloadFormats && (
            <Grid container spacing={2} mt={1}>
              <Grid item xs={12}>
                <Paper sx={{ p: 2, display: 'flex', flexDirection: 'column' }}>
                  <Typography variant="h6" pb={1}>
                    {downloadFormats.title}
                  </Typography>
                  {downloadFormats.thumbnail && (
                    <img
                      src={downloadFormats.thumbnail}
                      height={200}
                      style={{ objectFit: 'cover', borderRadius: 4 }}
                    />
                  )}

                  <Box mt={2}>
                    <DownloadOptionsPanel
                      metadata={downloadFormats}
                      value={options}
                      onChange={setOptions}
                    />
                  </Box>

                  <Stack direction="row" spacing={1} sx={{ mt: 2 }}>
                    <Button variant="contained" onClick={() => sendUrl()}>
                      {i18n.t('startButton')}
                    </Button>
                    <Button onClick={() => setDownloadFormats(undefined)}>
                      {i18n.t('clearCompletedButton')}
                    </Button>
                  </Stack>
                </Paper>
              </Grid>
            </Grid>
          )}
        </Container>
      </Box>
    </Dialog>
  )
}

export default DownloadDialog