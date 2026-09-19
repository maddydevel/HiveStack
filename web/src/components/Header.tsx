import React from 'react'
import { Link } from 'react-router-dom'

export default function Header() {
  return (
    <header className="h-14 bg-slate-800 border-b border-slate-700 flex items-center px-6">
      <div className="flex items-center gap-4">
        <Link to="/" className="text-lg font-bold text-blue-400">HiveStack</Link>
        <span className="text-xs text-slate-500">|</span>
        <span className="text-sm text-slate-400">Virtualization Platform</span>
      </div>
      <div className="ml-auto flex items-center gap-4">
        <span className="text-sm text-slate-400">SLES 15 SP7 + KVM</span>
      </div>
    </header>
  )
}