import { CircularProgress } from '@mui/material'
import { Suspense, lazy } from 'react'
import { createHashRouter } from 'react-router-dom'
import Layout from './Layout'
import ErrorBoundary from './components/ErrorBoundary'

const Home = lazy(() => import('./views/Home'))
const Filebrowser = lazy(() => import('./views/Filebrowser'))
const Settings = lazy(() => import('./views/Settings'))

export const router = createHashRouter([
  {
    path: '/',
    Component: () => <Layout />,
    children: [
      {
        path: '/',
        element: (
          <Suspense fallback={<CircularProgress />}>
            <Home />
          </Suspense >
        ),
        errorElement: (
          <Suspense fallback={<CircularProgress />}>
            <ErrorBoundary />
          </Suspense >
        )
      },
      {
        path: '/filebrowser',
        element: (
          <Suspense fallback={<CircularProgress />}>
            <Filebrowser />
          </Suspense >
        ),
        errorElement: (
          <Suspense fallback={<CircularProgress />}>
            <ErrorBoundary />
          </Suspense >
        )
      },
      {
        // Settings is a page, like Browse and Downloads.
        //
        // It used to be a panel over the current view, on the grounds that
        // settings are changed while watching a download. But it behaved
        // differently from the other two destinations in every visible way —
        // it did not highlight itself when it was open, it did not move the
        // browser history, and going back from it did not go back. Now it is
        // a destination like the others.
        path: '/settings',
        element: (
          <Suspense fallback={<CircularProgress />}>
            <Settings />
          </Suspense >
        ),
        errorElement: (
          <Suspense fallback={<CircularProgress />}>
            <ErrorBoundary />
          </Suspense >
        )
      },
      {
        path: '/error',
        element: (
          <Suspense fallback={<CircularProgress />}>
            <ErrorBoundary />
          </Suspense >
        )
      },
    ]
  },
])
