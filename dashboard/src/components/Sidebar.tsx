'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'
import {
  LayoutDashboard,
  Bot,
  ListTodo,
  DollarSign,
  ScrollText,
  Settings,
  Workflow,
  Target,
  Globe,
  Languages,
  Menu,
  X,
} from 'lucide-react'
import { clsx } from 'clsx'
import { useState, useEffect } from 'react'
import { useTranslation } from '@/lib/i18n'

const navItems = [
  { href: '/', label: 'Dashboard', icon: LayoutDashboard },
  { href: '/agents', label: 'Agents', icon: Bot },
  { href: '/tasks', label: 'Tasks', icon: ListTodo },
  { href: '/goals', label: 'Goals', icon: Target },
  { href: '/workflows', label: 'Workflows', icon: Workflow },
  { href: '/federation', label: 'Federation', icon: Globe },
  { href: '/costs', label: 'Costs', icon: DollarSign },
  { href: '/logs', label: 'Logs', icon: ScrollText },
  { href: '/settings', label: 'Settings', icon: Settings },
]

function LanguageToggle() {
  const { locale, setLocale } = useTranslation()
  return (
    <button
      onClick={() => setLocale(locale === 'zh' ? 'en' : 'zh')}
      className="flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium w-full text-gray-600 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-800 transition-colors"
    >
      <Languages className="w-5 h-5" />
      {locale === 'zh' ? 'English' : '中文'}
    </button>
  )
}

export function Sidebar() {
  const pathname = usePathname()
  const [mobileOpen, setMobileOpen] = useState(false)

  // Close mobile sidebar on route change
  useEffect(() => {
    setMobileOpen(false)
  }, [pathname])

  const sidebarContent = (
    <>
      {/* Logo */}
      <div className="h-16 flex items-center justify-between px-6 border-b border-gray-200 dark:border-gray-800">
        <Link href="/" className="flex items-center gap-2">
          <div className="w-8 h-8 bg-gradient-to-br from-primary-500 to-primary-600 rounded-lg flex items-center justify-center">
            <span className="text-white font-bold text-sm">OPC</span>
          </div>
          <span className="text-lg font-semibold text-gray-900 dark:text-white">
            Platform
          </span>
        </Link>
        <button
          onClick={() => setMobileOpen(false)}
          className="lg:hidden text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
        >
          <X className="w-5 h-5" />
        </button>
      </div>

      {/* Navigation */}
      <nav className="flex-1 px-3 py-4 space-y-1">
        {navItems.map((item) => {
          const Icon = item.icon
          const isActive = pathname === item.href ||
            (item.href !== '/' && pathname.startsWith(item.href))

          return (
            <Link
              key={item.href}
              href={item.href}
              className={clsx(
                'flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium transition-colors',
                isActive
                  ? 'bg-primary-50 text-primary-600 dark:bg-primary-900/20 dark:text-primary-400'
                  : 'text-gray-600 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-800'
              )}
            >
              <Icon className="w-5 h-5" />
              {item.label}
            </Link>
          )
        })}
      </nav>

      {/* Language + Version */}
      <div className="px-3 py-4 border-t border-gray-200 dark:border-gray-800 space-y-2">
        <LanguageToggle />
        <p className="text-xs text-gray-400 px-3">OPC Platform v0.5.0</p>
      </div>
    </>
  )

  return (
    <>
      {/* Mobile hamburger button */}
      <button
        onClick={() => setMobileOpen(true)}
        className="lg:hidden fixed top-4 left-4 z-40 p-2 bg-white dark:bg-gray-900 rounded-lg shadow-md border border-gray-200 dark:border-gray-800"
      >
        <Menu className="w-5 h-5 text-gray-600 dark:text-gray-300" />
      </button>

      {/* Mobile overlay */}
      {mobileOpen && (
        <div
          className="lg:hidden fixed inset-0 z-40 bg-black/50"
          onClick={() => setMobileOpen(false)}
        />
      )}

      {/* Mobile sidebar */}
      <aside
        className={clsx(
          'lg:hidden fixed inset-y-0 left-0 z-50 w-64 bg-white dark:bg-gray-900 border-r border-gray-200 dark:border-gray-800 flex flex-col transform transition-transform duration-200 ease-in-out',
          mobileOpen ? 'translate-x-0' : '-translate-x-full'
        )}
      >
        {sidebarContent}
      </aside>

      {/* Desktop sidebar */}
      <aside className="hidden lg:flex w-64 bg-white dark:bg-gray-900 border-r border-gray-200 dark:border-gray-800 flex-col">
        {sidebarContent}
      </aside>
    </>
  )
}
