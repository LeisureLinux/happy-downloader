import { styled } from '@mui/material'
import MuiAppBar from '@mui/material/AppBar'

// The app bar no longer shifts for a drawer — there is no drawer. It is a
// plain top bar: product name on the left, the four controls on the right.
//
// The bar is the colour of the page, not a band of accent across the top. MUI
// paints an AppBar in palette.primary and the title and controls are drawn in
// palette.primary, so an accent bar behind accent text is a bar of nothing:
// the text was there, invisible, and Ctrl-A was the only way to see it. The
// controls have to be the accent — that is the whole point of the accent
// setting — so the bar is not.
const AppBar = styled(MuiAppBar)(({ theme }) => ({
  zIndex: theme.zIndex.drawer + 1,
  backgroundColor: theme.palette.background.default,
  color: theme.palette.text.primary,
  // Paper in dark mode carries a translucent white gradient, which on a bar
  // meant to be the page colour reads as a lighter band with nothing in it.
  backgroundImage: 'none',
  // Elevation 4 is a shadow under a coloured bar. Under a bar the same colour
  // as the page it is a smudge, so the separation is a hairline instead.
  boxShadow: 'none',
  borderBottom: `1px solid ${theme.palette.divider}`,
}))

export default AppBar
