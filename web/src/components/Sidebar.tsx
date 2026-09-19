import React from 'react'
import { Link, useLocation } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth'

const navItems = [
  { path: '/', label: 'Dashboard', icon: '📊' },
  { path: '/hosts', label: 'Hosts', icon: '🖥️' },
  { path: '/vms', label: 'VMs', icon: '📦' },
  { path: '/storage', label: 'Storage', icon: '💾' },
  { path: '/networks', label: 'Networks', icon: '🌐' },
  { path: '/backups', label: 'Backups', icon: '📸' },
  { path: '/compliance', label: 'Compliance', icon: '✅' },
  { path: '/events', label: 'Events', icon: '📋' },
  { path: '/settings', label: 'Settings', icon: '⚙️' },
]

export default function Sidebar() {
  const location = useLocation()
  const { user, logout } = useAuth()

  return (
    <aside className="w-64 bg-slate-800 border-r border-slate-700 flex flex-col">
      <div className="p-4 border-b border-slate-700">
        <h1 className="text-xl font-bold text-blue-400">HiveStack</h1>
        <p className="text-xs text-slate-400 mt-1">Virtualization Platform</p>
      </div>
      <nav className="flex-1 p-2 overflow-y-auto">
        {navItems.map((item) => (
          <Link
            key={item.path}
            to={item.path}
            className={`flex items-center gap-3 px-3 py-2 rounded-lg mb-1 transition-colors ${
              location.pathname === item.path
                ? 'bg-blue-600 text-white'
                : 'text-slate-300 hover:bg-slate-700'
            }`}
          >
            <span>{item.icon}</span>
            <span className="text-sm font-medium">{item.label}</span>
          </Link>
        ))}
      </nav>
      <div className="p-4 border-t border-slate-700">
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 rounded-full bg-blue-600 flex items-center justify-center text-sm font-bold">
            {user?.username?.charAt(0).toUpperCase() || 'U'}
          </div>
          <div className="flex-1 min-w-0">
            <p className="text-sm font-medium truncate">{user?.username || 'User'}</p>
            <p className="text-xs text-slate-400">{user?.role || 'role'}</p>
          </div>
          <button
            onClick={logout}
            className="text-slate-400 hover:text-red-400 transition-colors"
            title="Logout"
          >
            🚪
          </button>
        </div>
      </div>
    </aside>
  )
}