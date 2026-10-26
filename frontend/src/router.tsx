import { CircularProgress } from '@mui/material'
import { Suspense, lazy } from 'react'
import { createHashRouter } from 'react-router-dom'
import Layout from './Layout'
import Terminal from './views/Terminal'

const Home = lazy(() => import('./views/Home'))
const Settings = lazy(() => import('./views/Settings'))
const Filebrowser = lazy(() => import('./views/Filebrowser'))

const ErrorBoundary = lazy(() => import('./components/ErrorBoundary'))

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
        path: '/settings',
        element: (
          <Suspense fallback={<CircularProgress />}>
            <Settings />
          </Suspense >
        )
      },
      {
        path: '/log',
        element: (
          <Suspense fallback={<CircularProgress />}>
            <Terminal />
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