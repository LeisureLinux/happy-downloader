import FacebookIcon from '@mui/icons-material/Facebook'
import InstagramIcon from '@mui/icons-material/Instagram'
import LanguageIcon from '@mui/icons-material/Language'
import RedditIcon from '@mui/icons-material/Reddit'
import SmartDisplayIcon from '@mui/icons-material/SmartDisplay'
import XIcon from '@mui/icons-material/X'
import YouTubeIcon from '@mui/icons-material/YouTube'
import { SvgIcon, SvgIconProps, Theme } from '@mui/material'
import { ReactNode } from 'react'

// Whether the theme is a dark one.
//
// It is what decides which colour a mark is drawn in: three of them are
// black, and black on a dark background is black on black.
export const isDark = (theme: Theme): boolean =>
  theme.palette.mode === 'dark'

// The icon for a site a file came from.
//
// Material has icons for some of the sites and not for others, so the ones it
// does not have are drawn here. They are single-path glyphs in the same shape
// as the Material ones — a rounded square with the site's mark — so a row of
// them lines up rather than looking like a mix of two icon sets.
//
// A site nobody has an icon for gets the globe: the row still says where the
// file came from, which is the point of the column.

/** The Weibo mark: the eye. */
const WeiboIcon = (props: SvgIconProps) => (
  <SvgIcon {...props}>
    <path d="M10.1 20.3c-3.6 0-6.6-1.8-6.6-4.1 0-1.3.9-2.9 2.6-4.2 2.2-1.7 5-2.5 5.9-1.7.4.4.4 1 .1 1.6-.1.2-.2.4-.2.6 0 .4.4.7 1 .8 1 .4 1.6.4 2.5.4 2.9 0 5.5-2.2 5.5-4.9 0-2.7-2.2-4.7-5.1-4.7h-.2c.4-.9.3-1.8-.3-2.4-.9-.9-2.7-.8-4.4.4-1.6 1.1-2.6 2-2.4.4 0 .6.3.5.8-.3 1.7-3.9 4.9-3.9Zm-2.4 2.3c-1.2 0-2.2.8-2.2 1.7 0 1 .9 1.7 2.2 1.7s2.2-.8 2.2-1.7c0-.9-1-1.7-2.2-1.7Z" />
  </SvgIcon>
)

/** The Bilibili mark: the television with the two ears. */
const BilibiliIcon = (props: SvgIconProps) => (
  <SvgIcon {...props}>
    <path d="M7.5 2.3a1 1 0 0 1 .7.3l2 2h3.6l2-2a1 1 0 1 1 1.4 1.4l-1.2 1.2h.4A4.7 4.7 0 0 1 21 9.6v8.1a4.7 4.7 0 0 1-4.7 4.7H7.7A4.7 4.7 0 0 1 3 17.7V9.6a4.7 4.7 0 0 1 4.7-4.7h.4L6.8 3.7a1 1 0 0 1 .7-1.4ZM8 7.4a2.6 2.6 0 0 0-2.6 2.6v7.3c0 1.4 1.2 2.6 2.6 2.6h8c1.4 0 2.6-1.2 2.6-2.6V10A2.6 2.6 0 0 0 16 7.4H8Zm1.9 3.1a1.6 1.6 0 1 1 0 3.2 1.6 1.6 0 0 1 0-3.2Zm4.2 0a1.6 1.6 0 1 1 0 3.2 1.6 1.6 0 0 1 0-3.2Z" />
  </SvgIcon>
)

/** The Xiaohongshu mark: the red book. */
const XiaohongshuIcon = (props: SvgIconProps) => (
  <SvgIcon {...props}>
    <path d="M4 4.5A2.5 2.5 0 0 1 6.5 2H11a2 2 0 0 1 2 2v.4a2 2 0 0 1 2 2V18a2 2 0 0 1-2 2H6.5A2.5 2.5 0 0 1 4 17.5v-13Zm11.5 2.6a2 2 0 0 1 4 0v10.4A2.5 2.5 0 0 1 17 20h-1.5a2 2 0 0 1-2-2V9.5a2 2 0 0 1 2-2.4ZM6 6.5v9h3v-9H6Z" />
  </SvgIcon>
)

/** The TikTok mark: the note. */
const TikTokIcon = (props: SvgIconProps) => (
  <SvgIcon {...props}>
    <path d="M14 3h2.6a5.4 5.4 0 0 0 4.4 4.4V10a8 8 0 0 1-4.4-1.3v5.9a5.9 5.9 0 1 1-5.1-5.8v2.7a3.2 3.2 0 1 0 2.2 3V3Z" />
  </SvgIcon>
)

/** The Douyin mark: the note with its music. */
const DouyinIcon = (props: SvgIconProps) => (
  <SvgIcon {...props}>
    <path d="M15 3h2.6a5.4 5.4 0 0 0 4.4 4.4V10a8 8 0 0 1-4.4-1.3v5.9a5.9 5.9 0 1 1-5.1-5.8v2.7a3.2 3.2 0 1 0 2.2 3V3Z" />
  </SvgIcon>
)

/** The Vimeo mark: the v. */
const VimeoIcon = (props: SvgIconProps) => (
  <SvgIcon {...props}>
    <path d="M21 7.2c-.1 2.2-1.6 5.2-4.7 9-.3.5-.6 1-.8 1.3-.5.7-1 1.2-1.5 1.5-.6.4-1.2.5-1.8.3-1-.3-1.8-1-2.4-2-.5-1-.9-2.3-1.1-3.9-.3-2-.5-3.3-.6-3.9-.1-.4-.3-.7-.5-.9-.2-.2-.5-.2-.9 0-.3.2-.6.5-.9.9l-.4.6c-.4.7-.8 1.4-1.1 2.1-.4.8-.7 1.4-1 1.9-.4.7-.8 1-1.2 1-.3 0-.5-.1-.7-.3-.2-.2-.3-.5-.5-1-.2-.6-.3-1.3-.4-2.1C1 10.2.8 9 .6 7.8c0-.5.1-.9.4-1.2.3-.3.7-.4 1.2-.4.5 0 1 .2 1.5.6.5.4.9 1 1.3 1.8.4.8.8 1.9 1.2 3.3.1.4.2.7.3.8.1.2.2.3.3.3.2 0 .5-.4.8-1.1.4-.8.6-1.4.8-1.9.3-.8.3-1.5 0-2-.2-.6-.9-1.2-2.1-1.7.6 0 1.2-.1 1.7-.3.5-.2.9-.5 1.2-.9.3-.4.4-.8.4-1.4 0-.7-.3-1.3-.8-1.9-.5-.6-1.4-1-2.7-1.3 0-.1.1-.3.2-.4.3-.4.8-.6 1.4-.8.6-.2 1.3-.2 2.1-.1 1 0 1.9.2 2.8.5.9.3 1.5.7 1.9 1.3.4.6.6 1.3.6 2.2Z" />
  </SvgIcon>
)

/** The Niconico mark: the n in a television. */
const NiconicoIcon = (props: SvgIconProps) => (
  <SvgIcon {...props}>
    {/* The television the logo is drawn on.
        The inner rectangle is a hole rather than more of the same shape: it
        was filled in, so the n drawn on top of it was the same colour as the
        body and the whole mark was a black square. */}
    <path
      fillRule="evenodd"
      d="M4 4h16a1 1 0 0 1 1 1v13a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1Zm1 2v11h14V6H5Z"
    />
    {/* The n. */}
    <path d="M7.4 9.2h2.2v1.1c.5-.8 1.3-1.2 2.4-1.2 1.7 0 2.7 1 2.7 3v3.7h-2.3v-3.4c0-.9-.4-1.4-1.2-1.4-.9 0-1.4.6-1.4 1.6v3.2H7.4V9.2Z" />
  </SvgIcon>
)

/** The Twitch mark: the speech bubble. */
const TwitchIcon = (props: SvgIconProps) => (
  <SvgIcon {...props}>
    <path d="M4.3 2 2.6 6.4v15h5.1v-3.5h2.8l1.6 3.5h3.4l1.6-3.5h4.3V2H4.3Zm15.1 14.4h-2.6l-1.7 3.6-1.7-3.6H6.6V4.1h12.8v12.3ZM8.6 6.7h2.6v5.6H8.6V6.7Zm5.3 0h2.6v5.6h-2.6V6.7Z" />
  </SvgIcon>
)


// The colour a site's icon is drawn in.
//
// The brand colours are what make the column readable at a glance — a red
// YouTube and a blue X are recognisable without reading anything. A site
// without a colour of its own uses the theme's accent, so it still looks
// deliberate.
export const siteColor = (site: string | undefined, primary = '#1976d2'): string => {
  switch (site) {
    case 'youtube':
      return '#FF0000'
    case 'bilibili':
      return '#00A1D6'
    case 'weibo':
      return '#E6162D'
    case 'x':
      return '#000000'
    case 'xiaohongshu':
      return '#FF2442'
    case 'douyin':
      return '#FE2C55'
    case 'tiktok':
      return '#000000'
    case 'reddit':
      return '#FF4500'
    case 'instagram':
      return '#E1306C'
    case 'facebook':
      return '#1877F2'
    case 'vimeo':
      return '#1AB7EA'
    case 'twitch':
      return '#9146FF'
    case 'niconico':
      return '#252525'
    default:
      return primary
  }
}

// The colour a site's icon is drawn in, on a dark background.
//
// Three of the marks are black: X, TikTok and Niconico. Black on a dark
// background is black on black, so the icon was there and unreadable — the
// Niconico one looked like it was missing, and it was not.
//
// The dark-background version of a mark is the same shape in the colour the
// site uses for a light background, which is what every one of these has:
// X and TikTok are white on their site, and Niconico's television is white.
export const siteColorOnDark = (site: string | undefined, primary = '#90caf9'): string => {
  switch (site) {
    case 'x':
    case 'tiktok':
    case 'niconico':
      return '#FFFFFF'
    case 'youtube':
      // The bright red reads better on a dark background than the deep one.
      return '#FF5252'
    default:
      return siteColor(site, primary)
  }
}

// The icon for a site name.
//
// The names come from the server, which derives them from the URL. An
// unknown one gets the globe rather than nothing: the column is there to say
// where the file came from, and a missing icon would say nothing at all.
//
// It never returns nothing. A site without a case used to come back as
// undefined, and a <Tooltip> whose child is undefined is a crash — the whole
// page went down over an icon that was simply not drawn.
//
// It returns an element rather than a bare node, because a bare node has to
// be wrapped to be given a colour, and the wrapping is a <div> that a
// <Tooltip> cannot put a ref on. The element carries its own colour
// instead, so the caller can use it directly.
//
// The dark argument is for the marks that are black. Three of them are — X,
// TikTok and Niconico — and black on a dark background is black on black, so
// the icon was there and unreadable.
export const siteIcon = (site?: string, primary?: string, dark = false): ReactNode => {
  const color = dark ? siteColorOnDark(site, primary) : siteColor(site, primary)

  switch (site) {
    case 'youtube':
      return <YouTubeIcon fontSize="small" sx={{ color }} />
    case 'bilibili':
      return <BilibiliIcon fontSize="small" sx={{ color }} />
    case 'weibo':
      return <WeiboIcon fontSize="small" sx={{ color }} />
    case 'x':
      return <XIcon fontSize="small" sx={{ color }} />
    case 'xiaohongshu':
      return <XiaohongshuIcon fontSize="small" sx={{ color }} />
    case 'douyin':
      return <DouyinIcon fontSize="small" sx={{ color }} />
    case 'tiktok':
      return <TikTokIcon fontSize="small" sx={{ color }} />
    case 'reddit':
      return <RedditIcon fontSize="small" sx={{ color }} />
    case 'instagram':
      return <InstagramIcon fontSize="small" sx={{ color }} />
    case 'facebook':
      return <FacebookIcon fontSize="small" sx={{ color }} />
    case 'vimeo':
      return <VimeoIcon fontSize="small" sx={{ color }} />
    case 'twitch':
      return <TwitchIcon fontSize="small" sx={{ color }} />
    case 'niconico':
      return <NiconicoIcon fontSize="small" sx={{ color }} />
    default:
      return <LanguageIcon fontSize="small" sx={{ color }} />
  }
}

