'use client'

import { useState, useEffect, useCallback } from 'react'
import { Save, Key, Bell, Server, Shield } from 'lucide-react'
import { fetchSettings, updateSettings } from '@/lib/api'

export default function SettingsPage() {
  const [saved, setSaved] = useState(false)
  const [saving, setSaving] = useState(false)
  const [loading, setLoading] = useState(true)

  // API Configuration
  const [openaiKey, setOpenaiKey] = useState('')
  const [anthropicKey, setAnthropicKey] = useState('')

  // Notifications
  const [notifyTaskFailure, setNotifyTaskFailure] = useState(true)
  const [notifyBudgetAlert, setNotifyBudgetAlert] = useState(true)
  const [notifyAgentCrash, setNotifyAgentCrash] = useState(true)

  // Gateway
  const [telegramToken, setTelegramToken] = useState('')
  const [discordToken, setDiscordToken] = useState('')

  // Security
  const [confirmDestructive, setConfirmDestructive] = useState(true)
  const [enableAuditLog, setEnableAuditLog] = useState(true)

  const loadSettings = useCallback(async () => {
    try {
      const data = await fetchSettings()
      if (data.openaiKey) setOpenaiKey(data.openaiKey as string)
      if (data.anthropicKey) setAnthropicKey(data.anthropicKey as string)
      if (data.notifyTaskFailure !== undefined) setNotifyTaskFailure(data.notifyTaskFailure as boolean)
      if (data.notifyBudgetAlert !== undefined) setNotifyBudgetAlert(data.notifyBudgetAlert as boolean)
      if (data.notifyAgentCrash !== undefined) setNotifyAgentCrash(data.notifyAgentCrash as boolean)
      if (data.telegramToken) setTelegramToken(data.telegramToken as string)
      if (data.discordToken) setDiscordToken(data.discordToken as string)
      if (data.confirmDestructive !== undefined) setConfirmDestructive(data.confirmDestructive as boolean)
      if (data.enableAuditLog !== undefined) setEnableAuditLog(data.enableAuditLog as boolean)
    } catch {
      // Ignore load errors; use defaults.
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    loadSettings()
  }, [loadSettings])

  const handleSave = async () => {
    setSaving(true)
    try {
      await updateSettings({
        openaiKey,
        anthropicKey,
        notifyTaskFailure,
        notifyBudgetAlert,
        notifyAgentCrash,
        telegramToken,
        discordToken,
        confirmDestructive,
        enableAuditLog,
      })
      setSaved(true)
      setTimeout(() => setSaved(false), 2000)
    } catch {
      // Could show an error toast here.
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return (
      <div className="p-6 flex items-center justify-center">
        <p className="text-gray-500 dark:text-gray-400">Loading settings...</p>
      </div>
    )
  }

  return (
    <div className="p-6 space-y-6 max-w-4xl">
      <div>
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
          Settings
        </h1>
        <p className="text-gray-500 dark:text-gray-400">
          Configure your OPC Platform
        </p>
      </div>

      {/* API Configuration */}
      <section className="bg-white dark:bg-gray-900 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800 p-6">
        <div className="flex items-center gap-3 mb-4">
          <div className="p-2 bg-blue-50 dark:bg-blue-900/20 rounded-lg">
            <Key className="w-5 h-5 text-blue-600 dark:text-blue-400" />
          </div>
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">
            API Configuration
          </h2>
        </div>
        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              OpenAI API Key
            </label>
            <input
              type="password"
              placeholder="sk-..."
              value={openaiKey}
              onChange={(e) => setOpenaiKey(e.target.value)}
              className="w-full px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-white"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Anthropic API Key
            </label>
            <input
              type="password"
              placeholder="sk-ant-..."
              value={anthropicKey}
              onChange={(e) => setAnthropicKey(e.target.value)}
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
            Notifications
          </h2>
        </div>
        <div className="space-y-4">
          <label className="flex items-center justify-between">
            <span className="text-sm text-gray-700 dark:text-gray-300">
              Email notifications for task failures
            </span>
            <input
              type="checkbox"
              checked={notifyTaskFailure}
              onChange={(e) => setNotifyTaskFailure(e.target.checked)}
              className="rounded border-gray-300 text-primary-600 focus:ring-primary-500"
            />
          </label>
          <label className="flex items-center justify-between">
            <span className="text-sm text-gray-700 dark:text-gray-300">
              Budget alert notifications
            </span>
            <input
              type="checkbox"
              checked={notifyBudgetAlert}
              onChange={(e) => setNotifyBudgetAlert(e.target.checked)}
              className="rounded border-gray-300 text-primary-600 focus:ring-primary-500"
            />
          </label>
          <label className="flex items-center justify-between">
            <span className="text-sm text-gray-700 dark:text-gray-300">
              Agent crash notifications
            </span>
            <input
              type="checkbox"
              checked={notifyAgentCrash}
              onChange={(e) => setNotifyAgentCrash(e.target.checked)}
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
            Gateway Channels
          </h2>
        </div>
        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Telegram Bot Token
            </label>
            <input
              type="password"
              placeholder="123456:ABC..."
              value={telegramToken}
              onChange={(e) => setTelegramToken(e.target.value)}
              className="w-full px-3 py-2 border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-white"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Discord Bot Token
            </label>
            <input
              type="password"
              placeholder="..."
              value={discordToken}
              onChange={(e) => setDiscordToken(e.target.value)}
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
            Security
          </h2>
        </div>
        <div className="space-y-4">
          <label className="flex items-center justify-between">
            <span className="text-sm text-gray-700 dark:text-gray-300">
              Require confirmation for destructive operations
            </span>
            <input
              type="checkbox"
              checked={confirmDestructive}
              onChange={(e) => setConfirmDestructive(e.target.checked)}
              className="rounded border-gray-300 text-primary-600 focus:ring-primary-500"
            />
          </label>
          <label className="flex items-center justify-between">
            <span className="text-sm text-gray-700 dark:text-gray-300">
              Enable audit logging
            </span>
            <input
              type="checkbox"
              checked={enableAuditLog}
              onChange={(e) => setEnableAuditLog(e.target.checked)}
              className="rounded border-gray-300 text-primary-600 focus:ring-primary-500"
            />
          </label>
        </div>
      </section>

      {/* Save Button */}
      <div className="flex justify-end">
        <button
          onClick={handleSave}
          disabled={saving}
          className="flex items-center gap-2 px-6 py-2 text-white bg-primary-600 hover:bg-primary-700 rounded-lg transition-colors disabled:opacity-50"
        >
          <Save className="w-4 h-4" />
          {saved ? 'Saved!' : saving ? 'Saving...' : 'Save Changes'}
        </button>
      </div>
    </div>
  )
}
