import { styled } from '@mui/material'
import MuiAppBar from '@mui/material/AppBar'

// The app bar no longer shifts for a drawer — there is no drawer. It is a
// plain top bar: product name on the left, the two destinations on the right.
const AppBar = styled(MuiAppBar)(({ theme }) => ({
  zIndex: theme.zIndex.drawer + 1,
}))

export default AppBar
