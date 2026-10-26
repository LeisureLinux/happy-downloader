import { CircularProgress } from '@mui/material'
import { Suspense, lazy } from 'react'
import { createHashRouter } from 'react-router-dom'
import Layout from './Layout'
import ErrorBoundary from './components/ErrorBoundary'

const Home = lazy(() => import('./views/Home'))
const Filebrowser = lazy(() => import('./views/Filebrowser'))


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