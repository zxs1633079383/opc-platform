'use client'

import { useState } from 'react'
import { Save, Key, Bell, Server, Shield } from 'lucide-react'
import { useTranslation } from '@/lib/i18n'

export default function SettingsPage() {
  const { t } = useTranslation()
  const [saved, setSaved] = useState(false)

  const handleSave = () => {
    setSaved(true)
    setTimeout(() => setSaved(false), 2000)
  }

  return (
    <div className="p-6 space-y-6 max-w-4xl">
      <div>
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
          {t('settings.title')}
        </h1>
        <p className="text-gray-500 dark:text-gray-400">
          {t('settings.subtitle')}
        </p>
      </div>

      {/* API Configuration */}
      <section className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 p-6">
        <div className="flex items-center gap-3 mb-4">
          <div className="p-2 bg-blue-50 dark:bg-blue-900/20 rounded-lg">
            <Key className="w-5 h-5 text-blue-600 dark:text-blue-400" />
          </div>
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">
            {t('settings.apiConfig')}
          </h2>
        </div>
        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              {t('settings.openaiKey')}
            </label>
            <input
              type="password"
              placeholder="sk-..."
              className="w-full px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-white"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              {t('settings.anthropicKey')}
            </label>
            <input
              type="password"
              placeholder="sk-ant-..."
              className="w-full px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-white"
            />
          </div>
        </div>
      </section>

      {/* Notification Settings */}
      <section className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 p-6">
        <div className="flex items-center gap-3 mb-4">
          <div className="p-2 bg-yellow-50 dark:bg-yellow-900/20 rounded-lg">
            <Bell className="w-5 h-5 text-yellow-600 dark:text-yellow-400" />
          </div>
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">
            {t('settings.notifications')}
          </h2>
        </div>
        <div className="space-y-4">
          <label className="flex items-center justify-between">
            <span className="text-sm text-gray-700 dark:text-gray-300">
              {t('settings.emailOnFailure')}
            </span>
            <input
              type="checkbox"
              defaultChecked
              className="rounded border-gray-300 text-primary-600 focus:ring-primary-500"
            />
          </label>
          <label className="flex items-center justify-between">
            <span className="text-sm text-gray-700 dark:text-gray-300">
              {t('settings.budgetAlerts')}
            </span>
            <input
              type="checkbox"
              defaultChecked
              className="rounded border-gray-300 text-primary-600 focus:ring-primary-500"
            />
          </label>
          <label className="flex items-center justify-between">
            <span className="text-sm text-gray-700 dark:text-gray-300">
              {t('settings.agentCrashes')}
            </span>
            <input
              type="checkbox"
              defaultChecked
              className="rounded border-gray-300 text-primary-600 focus:ring-primary-500"
            />
          </label>
        </div>
      </section>

      {/* Gateway Settings */}
      <section className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 p-6">
        <div className="flex items-center gap-3 mb-4">
          <div className="p-2 bg-purple-50 dark:bg-purple-900/20 rounded-lg">
            <Server className="w-5 h-5 text-purple-600 dark:text-purple-400" />
          </div>
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">
            {t('settings.gateway')}
          </h2>
        </div>
        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              {t('settings.telegramToken')}
            </label>
            <input
              type="password"
              placeholder="123456:ABC..."
              className="w-full px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-white"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              {t('settings.discordToken')}
            </label>
            <input
              type="password"
              placeholder="..."
              className="w-full px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-white"
            />
          </div>
        </div>
      </section>

      {/* Security Settings */}
      <section className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 p-6">
        <div className="flex items-center gap-3 mb-4">
          <div className="p-2 bg-red-50 dark:bg-red-900/20 rounded-lg">
            <Shield className="w-5 h-5 text-red-600 dark:text-red-400" />
          </div>
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">
            {t('settings.security')}
          </h2>
        </div>
        <div className="space-y-4">
          <label className="flex items-center justify-between">
            <span className="text-sm text-gray-700 dark:text-gray-300">
              {t('settings.confirmDestructive')}
            </span>
            <input
              type="checkbox"
              defaultChecked
              className="rounded border-gray-300 text-primary-600 focus:ring-primary-500"
            />
          </label>
          <label className="flex items-center justify-between">
            <span className="text-sm text-gray-700 dark:text-gray-300">
              {t('settings.auditLogging')}
            </span>
            <input
              type="checkbox"
              defaultChecked
              className="rounded border-gray-300 text-primary-600 focus:ring-primary-500"
            />
          </label>
        </div>
      </section>

      {/* Save Button */}
      <div className="flex justify-end">
        <button
          onClick={handleSave}
          className="flex items-center gap-2 px-6 py-2 text-white bg-primary-600 hover:bg-primary-700 rounded-lg transition-colors"
        >
          <Save className="w-4 h-4" />
          {saved ? t('settings.saved') : t('settings.save')}
        </button>
      </div>
    </div>
  )
}
