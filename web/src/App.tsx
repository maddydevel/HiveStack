import React from 'react'
import { Routes, Route, Navigate } from 'react-router-dom'
import { useAuth } from './hooks/useAuth'
import Layout from './components/Layout'
import Login from './pages/Login'
import Dashboard from './pages/Dashboard'
import Hosts from './pages/Hosts'
import HostDetail from './pages/HostDetail'
import VMs from './pages/VMs'
import VMDetail from './pages/VMDetail'
import VMCreate from './pages/VMCreate'
import Storage from './pages/Storage'
import Networks from './pages/Networks'
import Backups from './pages/Backups'
import Compliance from './pages/Compliance'
import Events from './pages/Events'
import Settings from './pages/Settings'

function PrivateRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated } = useAuth()
  return isAuthenticated ? <>{children}</> : <Navigate to="/login" replace />
}

function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route
        path="/*"
        element={
          <PrivateRoute>
            <Layout>
              <Routes>
                <Route path="/" element={<Dashboard />} />
                <Route path="/hosts" element={<Hosts />} />
                <Route path="/hosts/:id" element={<HostDetail />} />
                <Route path="/vms" element={<VMs />} />
                <Route path="/vms/create" element={<VMCreate />} />
                <Route path="/vms/:id" element={<VMDetail />} />
                <Route path="/storage" element={<Storage />} />
                <Route path="/networks" element={<Networks />} />
                <Route path="/backups" element={<Backups />} />
                <Route path="/compliance" element={<Compliance />} />
                <Route path="/events" element={<Events />} />
                <Route path="/settings" element={<Settings />} />
              </Routes>
            </Layout>
          </PrivateRoute>
        }
      />
    </Routes>
  )
}

export default App